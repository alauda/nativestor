package raw_device

import (
	"context"
	"github.com/alauda/topolvm-operator/csi"
	raw_device2 "github.com/alauda/topolvm-operator/pkg/raw_device"
	"google.golang.org/protobuf/types/known/wrapperspb"
	ctrl "sigs.k8s.io/controller-runtime"
)

var idLogger = ctrl.Log.WithName("driver").WithName("identity")

// NewIdentityService returns a new IdentityServer.
//
// ready is a function to check the plugin status.
// It should return non-nil error if the plugin is not healthy.
// If the plugin is not yet ready, it should return (false, nil).
// Otherwise, return (true, nil).
func NewIdentityService() csi.IdentityServer {
	return &identityService{}
}

type identityService struct {
	csi.UnimplementedIdentityServer
}

func (s identityService) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	idLogger.Info("GetPluginInfo", "req", req.String())
	return &csi.GetPluginInfoResponse{
		Name:          raw_device2.PluginName,
		VendorVersion: raw_device2.Version,
	}, nil
}

func (s identityService) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	idLogger.Info("GetPluginCapabilities", "req", req.String())
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}, nil
}

func (s identityService) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	idLogger.Info("Probe", "req", req.String())
	return &csi.ProbeResponse{
		Ready: &wrapperspb.BoolValue{
			Value: true,
		},
	}, nil
}
