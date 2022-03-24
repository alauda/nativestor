package clean

import (
	"context"
	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	"github.com/alauda/nativestor/pkg/cluster"
	"github.com/alauda/nativestor/pkg/util/exec"
	"github.com/alauda/nativestor/pkg/util/sys"
	"github.com/coreos/pkg/capnslog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = capnslog.NewPackageLogger("topolvm/operator", "cleanup")

type CleanUp struct {
	nodeName           string
	namespace          string
	topolvmClusterName string
	context            *cluster.Context
}

func NewCleanUp(nodeName string, nameSpace string, topolvmClusterName string, context *cluster.Context) *CleanUp {
	return &CleanUp{
		nodeName:           nodeName,
		namespace:          nameSpace,
		topolvmClusterName: topolvmClusterName,
		context:            context,
	}
}

func (c *CleanUp) Start() error {
	topolvmCluster, err := c.context.TopolvmClusterClientset.TopolvmV2().TopolvmClusters(c.namespace).Get(context.TODO(), c.topolvmClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	vgs, pvs := getVgInfo(topolvmCluster, c.nodeName)
	return cleanVgs(c.context.Executor, vgs, pvs)
}

func getVgInfo(topolvmCluster *topolvmv2.TopolvmCluster, node string) (map[string]string, map[string]string) {

	vgs := make(map[string]string)
	pvs := make(map[string]string)

	if topolvmCluster.Spec.Storage.VolumeGroupName != "" {
		vgs[topolvmCluster.Spec.Storage.VolumeGroupName] = topolvmCluster.Spec.Storage.VolumeGroupName
	}
	if topolvmCluster.Spec.Storage.DeviceClasses != nil {
		for _, ele := range topolvmCluster.Spec.Storage.DeviceClasses {
			if ele.NodeName == node {
				for _, v := range ele.DeviceClasses {
					vgs[v.VgName] = v.VgName
					for _, d := range v.Device {
						pvs[d.Name] = d.Name
					}
				}
			}
		}
	}

	return vgs, pvs
}

func cleanVgs(executor exec.Executor, vgs map[string]string, pvs map[string]string) error {

	for _, vg := range vgs {
		res, err := sys.GetPhysicalVolume(executor, vg)
		if err == nil {
			for k, _ := range res {
				delete(pvs, k)
			}
		}
		err = sys.RemoveVolumeGroup(executor, vg)
		if err != nil {
			logger.Errorf("clean vg %s failed err %v", vg, err)
		}
	}

	for _, pv := range pvs {
		err := sys.RemovePhysicalVolume(executor, pv)
		if err != nil {
			logger.Errorf("remove pv %s failed err %v", pv, err)
		}
	}

	return nil
}
