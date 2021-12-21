package raw_device

import (
	"context"
	"github.com/alauda/topolvm-operator/csi"
	lister "github.com/alauda/topolvm-operator/generated/nativestore/rawdevice/listers/rawdevice/v1"
	clientctx "github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/raw_device"
	"github.com/topolvm/topolvm/filesystem"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mountutil "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	"os"
	"path"
	ctrl "sigs.k8s.io/controller-runtime"
	"sync"
)

const (
	devicePermission = 0600 | unix.S_IFBLK
)

var nodeLogger = ctrl.Log.WithName("driver").WithName("node")

// NewNodeService returns a new NodeServer.
func NewNodeService(ctx *clientctx.Context, deviceLister lister.RawDeviceLister, nodeName string) csi.NodeServer {
	return &nodeService{
		nodeName:        nodeName,
		ctx:             ctx,
		rawDeviceLister: deviceLister,
		mounter: mountutil.SafeFormatAndMount{
			Interface: mountutil.New(""),
			Exec:      utilexec.New(),
		},
	}
}

type nodeService struct {
	csi.UnimplementedNodeServer
	ctx             *clientctx.Context
	rawDeviceLister lister.RawDeviceLister
	nodeName        string
	mu              sync.Mutex
	mounter         mountutil.SafeFormatAndMount
}

func (s *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeContext := req.GetVolumeContext()
	volumeID := req.GetVolumeId()

	nodeLogger.Info("NodePublishVolume called",
		"volume_id", volumeID,
		"publish_context", req.GetPublishContext(),
		"target_path", req.GetTargetPath(),
		"volume_capability", req.GetVolumeCapability(),
		"read_only", req.GetReadonly(),
		"num_secrets", len(req.GetSecrets()),
		"volume_context", volumeContext)

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume_capability is provided")
	}
	isBlockVol := req.GetVolumeCapability().GetBlock() != nil
	if !isBlockVol {
		return nil, status.Errorf(codes.InvalidArgument, "no supported volume capability: %v", req.GetVolumeCapability())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.nodePublishBlockVolume(ctx, req)
	if err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishBlockVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) error {

	target := req.GetTargetPath()

	err := os.MkdirAll(path.Dir(target), 0755)
	if err != nil {
		return status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", path.Dir(target), err)
	}

	//get device major and minor
	rawDevice, err := s.ctx.RawDeviceClientset.RawdeviceV1().RawDevices().Get(ctx, req.VolumeId, metav1.GetOptions{})
	if err != nil {
		return err
	}

	devno := unix.Mkdev(rawDevice.Spec.Major, rawDevice.Spec.Minor)
	if err := filesystem.Mknod(target, devicePermission, int(devno)); err != nil {
		return status.Errorf(codes.Internal, "mknod failed for %s: error=%v", target, err)
	}

	nodeLogger.Info("NodePublishVolume(block) succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", target)
	return nil
}

func (s *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volID := req.GetVolumeId()
	target := req.GetTargetPath()
	nodeLogger.Info("NodeUnpublishVolume called",
		"volume_id", volID,
		"target_path", target)

	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.nodeUnpublishBlockVolume(req)
}

func (s *nodeService) nodeUnpublishBlockVolume(req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := os.Remove(req.GetTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove failed for %s: error=%v", req.GetTargetPath(), err)
	}
	nodeLogger.Info("NodeUnpublishVolume(block) is succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", req.GetTargetPath())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (s *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				raw_device.TopologyNodeKey: s.nodeName,
			},
		},
	}, nil
}
