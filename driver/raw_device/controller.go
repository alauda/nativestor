package raw_device

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/alauda/nativestor/apis/rawdevice/v1"
	"github.com/alauda/nativestor/csi"
	lister "github.com/alauda/nativestor/generated/nativestore/rawdevice/listers/rawdevice/v1"
	clientctx "github.com/alauda/nativestor/pkg/cluster"
	"github.com/alauda/nativestor/pkg/raw_device"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
)

var ctrlLogger = ctrl.Log.WithName("driver").WithName("controller")

// NewControllerService returns a new ControllerServer.
func NewControllerService(ctx *clientctx.Context, deviceLister lister.RawDeviceLister) csi.ControllerServer {
	return &controllerService{
		ctx:             ctx,
		rawDeviceLister: deviceLister,
	}
}

type controllerService struct {
	csi.UnimplementedControllerServer
	ctx             *clientctx.Context
	rawDeviceLister lister.RawDeviceLister
}

func (s controllerService) getMaxCapacity(ctx context.Context) (node string, capacity int64, err error) {

	// list RawDevice find out max size
	rawDevicelist, err := s.rawDeviceLister.List(labels.Everything())
	if err != nil {
		return "", 0, err
	}

	for _, ele := range rawDevicelist {
		if (ele.Status.Name != "") || (!ele.Spec.Available) {
			continue
		}
		if ele.Spec.Size > capacity {
			capacity = ele.Spec.Size
			node = ele.Spec.NodeName
		}
	}

	return
}

func (s controllerService) createVolume(ctx context.Context, node string, requestGb int64, name string) (volumeId string, err error) {

	// find rawdevice that match the requirement
	set := labels.Set{"node": node}
	rawDevicelist, err := s.rawDeviceLister.List(labels.SelectorFromSet(set))
	if err != nil {
		return "", err
	}
	matchIndex := -1
	var matchSize int64

	for index, dev := range rawDevicelist {
		if (dev.Status.Name != "") || (!dev.Spec.Available) {
			continue
		}
		if (dev.Spec.Size >> 30) >= requestGb {
			if matchSize == 0 {
				matchSize = dev.Spec.Size
				matchIndex = index
			}

			if dev.Spec.Size < matchSize {
				matchSize = dev.Spec.Size
				matchIndex = index
			}
		}
	}

	if matchIndex >= 0 {
		// update rawdevice

		device := rawDevicelist[matchIndex].DeepCopy()
		volumeId = device.Name
		device.Status.Name = volumeId
		_, err = s.ctx.RawDeviceClientset.RawdeviceV1().RawDevices().UpdateStatus(ctx, device, metav1.UpdateOptions{})
		if err != nil {
			volumeId = ""
			return
		}

	} else {

		return "", status.Error(codes.Internal, "not found match device")
	}

	return
}

func (s controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	capabilities := req.GetVolumeCapabilities()
	source := req.GetVolumeContentSource()

	ctrlLogger.Info("CreateVolume called",
		"name", req.GetName(),
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
		"parameters", req.GetParameters(),
		"num_secrets", len(req.GetSecrets()),
		"capabilities", capabilities,
		"content_source", source,
		"accessibility_requirements", req.GetAccessibilityRequirements().String())

	if source != nil {
		return nil, status.Error(codes.InvalidArgument, "volume_content_source not supported")
	}
	if capabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume capabilities are provided")
	}

	// check required volume capabilities
	for _, capability := range capabilities {
		if block := capability.GetBlock(); block != nil {
			ctrlLogger.Info("CreateVolume specifies volume capability", "access_type", "block")
		} else if mount := capability.GetMount(); mount != nil {
			ctrlLogger.Info("CreateVolume specifies volume capability",
				"access_type", "mount",
				"fs_type", mount.GetFsType(),
				"flags", mount.GetMountFlags())
		} else {
			return nil, status.Error(codes.InvalidArgument, "unknown or empty access_type")
		}

		if mode := capability.GetAccessMode(); mode != nil {
			modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(mode.GetMode())]
			ctrlLogger.Info("CreateVolume specifies volume capability", "access_mode", modeName)
			// we only support SINGLE_NODE_WRITER
			switch mode.GetMode() {
			case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported access mode: %s", modeName)
			}
		}
	}

	requestGb, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// process topology
	var node string
	requirements := req.GetAccessibilityRequirements()
	if requirements == nil {
		// In CSI spec, controllers are required that they response OK even if accessibility_requirements field is nil.
		// So we must create volume, and must not return error response in this case.
		// - https://github.com/container-storage-interface/spec/blob/release-1.1/spec.md#createvolume
		// - https://github.com/kubernetes-csi/csi-test/blob/6738ab2206eac88874f0a3ede59b40f680f59f43/pkg/sanity/controller.go#L404-L428
		ctrlLogger.Info("decide node because accessibility_requirements not found")
		nodeName, capacity, err := s.getMaxCapacity(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get max capacity node %v", err)
		}
		if nodeName == "" {
			return nil, status.Error(codes.Internal, "can not find any node")
		}
		if capacity < (requestGb << 30) {
			return nil, status.Errorf(codes.ResourceExhausted, "can not find enough volume space %d", capacity)
		}
		node = nodeName
	} else {
		for _, topo := range requirements.Preferred {
			if v, ok := topo.GetSegments()[raw_device.TopologyNodeKey]; ok {
				node = v
				break
			}
		}
		if node == "" {
			for _, topo := range requirements.Requisite {
				if v, ok := topo.GetSegments()[raw_device.TopologyNodeKey]; ok {
					node = v
					break
				}
			}
		}
		if node == "" {
			return nil, status.Errorf(codes.InvalidArgument, "cannot find key '%s' in accessibility_requirements", raw_device.TopologyNodeKey)
		}
	}

	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid name")
	}

	name = strings.ToLower(name)

	volumeID, err := s.createVolume(ctx, node, requestGb, name)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestGb << 30,
			VolumeId:      volumeID,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{raw_device.TopologyNodeKey: node},
				},
			},
		},
	}, nil
}

func convertRequestCapacity(requestBytes, limitBytes int64) (int64, error) {
	if requestBytes < 0 {
		return 0, errors.New("required capacity must not be negative")
	}
	if limitBytes < 0 {
		return 0, errors.New("capacity limit must not be negative")
	}

	if limitBytes != 0 && requestBytes > limitBytes {
		return 0, fmt.Errorf(
			"requested capacity exceeds limit capacity: request=%d limit=%d", requestBytes, limitBytes,
		)
	}

	if requestBytes == 0 {
		return 1, nil
	}
	return (requestBytes-1)>>30 + 1, nil
}

func (s controllerService) deleteVolume(ctx context.Context, volumeId string) error {

	rawDevicelist, err := s.rawDeviceLister.List(labels.Everything())
	if err != nil {
		return err
	}
	matchIndex := -1
	for index, ele := range rawDevicelist {
		if ele.Status.Name == volumeId {
			matchIndex = index
		}
	}

	if matchIndex < 0 {
		return status.Error(codes.NotFound, "")
	}
	rawDevice := rawDevicelist[matchIndex].DeepCopy()
	rawDevice.Status.Name = ""
	_, err = s.ctx.RawDeviceClientset.RawdeviceV1().RawDevices().UpdateStatus(ctx, rawDevice, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s controllerService) getVolume(ctx context.Context, volumeId string) (*v1.RawDevice, error) {

	rawDevices, err := s.rawDeviceLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	matchIndex := -1
	for index, ele := range rawDevices {
		if ele.Status.Name == volumeId {
			matchIndex = index
		}
	}
	if matchIndex < 0 {
		return nil, status.Error(codes.NotFound, "")
	}
	return rawDevices[matchIndex].DeepCopy(), nil

}

func (s controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ctrlLogger.Info("DeleteVolume called",
		"volume_id", req.GetVolumeId(),
		"num_secrets", len(req.GetSecrets()))
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume_id is not provided")
	}

	err := s.deleteVolume(ctx, req.GetVolumeId())
	if err != nil {
		ctrlLogger.Error(err, "DeleteVolume failed", "volume_id", req.GetVolumeId())
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (s controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	ctrlLogger.Info("ValidateVolumeCapabilities called",
		"volume_id", req.GetVolumeId(),
		"volume_context", req.GetVolumeContext(),
		"volume_capabilities", req.GetVolumeCapabilities(),
		"parameters", req.GetParameters(),
		"num_secrets", len(req.GetSecrets()))

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are empty")
	}

	_, err := s.getVolume(ctx, req.GetVolumeId())
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", req.GetVolumeId())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Since TopoLVM does not provide means to pre-provision volumes,
	// any existing volume is valid.
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (s controllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	topology := req.GetAccessibleTopology()
	capabilities := req.GetVolumeCapabilities()
	ctrlLogger.Info("GetCapacity called",
		"volume_capabilities", capabilities,
		"parameters", req.GetParameters(),
		"accessible_topology", topology)
	if capabilities != nil {
		ctrlLogger.Info("capability argument is not nil, but TopoLVM ignores it")
	}

	var (
		capacity          int64
		maximumVolumeSize int64
		minimumVolumeSize int64
	)
	switch topology {
	case nil:
		return nil, status.Error(codes.InvalidArgument, "must provide topology info")
	default:
		v, ok := topology.Segments[raw_device.TopologyNodeKey]
		if !ok {
			err := fmt.Errorf("%s is not found in req.AccessibleTopology", raw_device.TopologyNodeKey)
			ctrlLogger.Error(err, "target node key is not found")
			return &csi.GetCapacityResponse{AvailableCapacity: 0}, nil
		}
		var err error
		capacity, maximumVolumeSize, minimumVolumeSize, err = s.getCapacityByTopologyLabel(ctx, v)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &csi.GetCapacityResponse{
		AvailableCapacity: capacity,
		MaximumVolumeSize: &wrapperspb.Int64Value{Value: maximumVolumeSize},
		MinimumVolumeSize: &wrapperspb.Int64Value{Value: minimumVolumeSize},
	}, nil
}

func (s controllerService) getCapacityByTopologyLabel(ctx context.Context, node string) (availableCapacity int64, maximumVolumeSize int64, minimumVolumeSize int64, err error) {

	set := labels.Set{"node": node}
	rawDevicelist, err := s.rawDeviceLister.List(labels.SelectorFromSet(set))
	if err != nil {
		return 0, 0, 0, err
	}

	for index, dev := range rawDevicelist {
		if dev.Status.Name != "" || !dev.Spec.Available {
			continue
		}
		availableCapacity += dev.Spec.Size
		if index == 0 {
			minimumVolumeSize = dev.Spec.Size
		}
		if dev.Spec.Size > maximumVolumeSize {
			maximumVolumeSize = dev.Spec.Size
		}
		if dev.Spec.Size < minimumVolumeSize {
			minimumVolumeSize = dev.Spec.Size
		}
	}
	ctrlLogger.Info("get capacity by topology label",
		"availableCapacity", availableCapacity,
		"maximumVolumeSize", maximumVolumeSize,
		"minimumVolumeSize", minimumVolumeSize,
	)

	return

}

func (s controllerService) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	capabilities := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
	}

	csiCaps := make([]*csi.ControllerServiceCapability, len(capabilities))
	for i, capability := range capabilities {
		csiCaps[i] = &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: capability,
				},
			},
		}
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

func (s controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	ctrlLogger.Info("ControllerExpandVolume called",
		"volumeID", volumeID,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
		"num_secrets", len(req.GetSecrets()))

	return nil, status.Error(codes.Unimplemented, "")
}
