package controller

import (
	"context"
	"encoding/json"
	"fmt"
	topolvmv2 "github.com/alauda/topolvm-operator/apis/topolvm/v2"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"
	"github.com/alauda/topolvm-operator/pkg/operator"
	"github.com/alauda/topolvm-operator/pkg/operator/csi"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	csitopo "github.com/alauda/topolvm-operator/pkg/operator/topolvm/csi"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

var lvmdLogger = capnslog.NewPackageLogger("topolvm/operator", "lvmd-config")

type lvmdConfigController struct {
	topolvmController *TopolvmController
	lvmdController    cache.Controller
}

func newLvmdController(topolvmController *TopolvmController) *lvmdConfigController {

	lvmd := &lvmdConfigController{
		topolvmController: topolvmController,
	}

	_, lvmd.lvmdController = cache.NewInformer(
		cache.NewFilteredListWatchFromClient(topolvmController.context.Clientset.CoreV1().RESTClient(),
			string(v1.ResourceConfigMaps),
			topolvmController.opConfig.OperatorNamespace,
			func(options *metav1.ListOptions) {
				options.LabelSelector = fmt.Sprintf("%s=%s", topolvm.LvmdConfigMapLabelKey, topolvm.LvmdConfigMapLabelValue)
			}), &v1.ConfigMap{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    lvmd.onAdd,
			UpdateFunc: lvmd.onUpdate,
			DeleteFunc: lvmd.onDelete,
		},
	)

	return lvmd
}

func (l *lvmdConfigController) start() {
	go l.lvmdController.Run(l.topolvmController.opManagerContext.Done())
}

func (l *lvmdConfigController) onAdd(obj interface{}) {
	lvmdLogger.Debugf("got configmap start process")

	cm, err := getClientObject(obj)
	if err != nil {
		lvmdLogger.Errorf("failed to get client object. %v", err)
		return
	}

	if l.topolvmController.getRef() == nil {
		lvmdLogger.Info("waiting fot topolvm cluster created")
		return
	}

	cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*l.topolvmController.getRef()}
	_, err = l.topolvmController.context.Clientset.CoreV1().ConfigMaps(cm.Namespace).Update(l.topolvmController.opManagerContext, cm, metav1.UpdateOptions{})
	if err != nil {
		lvmdLogger.Errorf("failed update cm:%s  own ref", cm.Name)
	}

	l.updateClusterStatus(cm)

	err = l.startTopolvmNodePlugin(cm)
	if err != nil {
		lvmdLogger.Errorf("failed to start topolvm node plugin %v", err)
	}

}

func (l *lvmdConfigController) startTopolvmNodePlugin(configMap *v1.ConfigMap) error {

	if _, ok := configMap.Data[topolvm.LvmdConfigMapKey]; !ok {
		return nil
	}

	nodeName := getNodeName(configMap)
	if nodeName == "" {
		lvmdLogger.Error("can not get node name")
		return nil
	}

	if l.checkingTopolvmNodePluginDeploymentExisting(nodeName) {
		return nil
	}

	err := l.createTopolvmPluginNodeDeployment(nodeName)
	if err != nil {

		return err
	}

	return nil
}

func (l *lvmdConfigController) onUpdate(oldObj, newobj interface{}) {

	oldCm, err := getClientObject(oldObj)
	if err != nil {
		lvmdLogger.Errorf("failed to get old client object. %v", err)
		return
	}

	newCm, err := getClientObject(newobj)
	if err != nil {
		lvmdLogger.Errorf("failed to get new client object. %v", err)
		return
	}

	if l.topolvmController.getRef() == nil {
		lvmdLogger.Info("waiting for topolvm cluster created")
		return
	}
	if newCm.OwnerReferences == nil {
		newCm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*l.topolvmController.getRef()}
		_, err = l.topolvmController.context.Clientset.CoreV1().ConfigMaps(newCm.Namespace).Update(l.topolvmController.opManagerContext, newCm, metav1.UpdateOptions{})
		if err != nil {
			lvmdLogger.Errorf("failed update cm:%s  own ref", newCm.Name)
		}
	}

	err = l.checkUpdateClusterStatus(oldCm, newCm)
	if err != nil {
		lvmdLogger.Errorf("update cluster failed err %v", err)
	}

	nodeName := getNodeName(newCm)
	if nodeName == "" {
		lvmdLogger.Error("can not get node name")
		return
	}

	if _, ok := oldCm.Data[topolvm.LocalDiskCMData]; ok {
		if oldCm.Data[topolvm.LocalDiskCMData] != newCm.Data[topolvm.LocalDiskCMData] && l.topolvmController.UseAllNodeAndDevices() {
			l.topolvmController.RestartJob(nodeName, l.topolvmController.getRef())
		}
	}

	if _, ok := newCm.Data[topolvm.LvmdConfigMapKey]; !ok {
		lvmdLogger.Errorf("node %s all volume groups are not available", nodeName)
		return
	}

	if l.checkingTopolvmNodePluginDeploymentExisting(nodeName) {
		if oldCm.Data[topolvm.LvmdConfigMapKey] == newCm.Data[topolvm.LvmdConfigMapKey] {
			lvmdLogger.Infof("cm%s  update but data not change no need to update node deployment", oldCm.ObjectMeta.Name)
			return
		}
		replaceNodePod(l.topolvmController.context, nodeName)

	} else {

		err = l.createTopolvmPluginNodeDeployment(nodeName)
		if err != nil {
			lvmdLogger.Errorf("create topolvm plugin failed %v", err)
		}
	}
}

func (l *lvmdConfigController) onDelete(obj interface{}) {
	// nothing
}
func (l *lvmdConfigController) checkingTopolvmNodePluginDeploymentExisting(nodeName string) bool {

	deploymentName := k8sutil.TruncateNodeName(topolvm.TopolvmNodeDeploymentFmt, nodeName)
	existing, err := k8sutil.CheckDeploymentIsExisting(l.topolvmController.opManagerContext, l.topolvmController.context.Clientset, deploymentName, l.topolvmController.namespacedName.Namespace)
	if err != nil {
		return false
	}
	return existing
}

func replaceNodePod(contextd *cluster.Context, nodeName string) {

	deploymentName := k8sutil.TruncateNodeName(topolvm.TopolvmNodeDeploymentFmt, nodeName)
	ctx := context.TODO()
	pods, err := contextd.Clientset.CoreV1().Pods(topolvm.NameSpace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", topolvm.AppAttr, deploymentName)})
	if err != nil {
		lvmdLogger.Errorf("list pod with label %s failed %v", deploymentName, err)
	}
	for _, val := range pods.Items {
		if err := contextd.Clientset.CoreV1().Pods(topolvm.NameSpace).Delete(ctx, val.ObjectMeta.Name, metav1.DeleteOptions{}); err != nil {
			lvmdLogger.Errorf("delete pod %s failed err %v", val.ObjectMeta.Name, err)
		}
	}
}

func getNodeName(cm *v1.ConfigMap) string {

	nodeName, ok := cm.GetAnnotations()[topolvm.LvmdAnnotationsNodeKey]
	if !ok {
		lvmdLogger.Error("can not get node name")
		return ""
	}
	return nodeName
}

func getClientObject(obj interface{}) (cm *v1.ConfigMap, err error) {

	var ok bool
	cm, ok = obj.(*v1.ConfigMap)
	if ok {
		// the client object is of the latest type, simply return it
		return cm.DeepCopy(), nil
	}
	return nil, errors.Errorf("not a known configmap: %+v", obj)
}

func (l *lvmdConfigController) updateClusterStatus(cm *v1.ConfigMap) {

	status := cm.Data[topolvm.VgStatusConfigMapKey]
	if status == "" {
		return
	}
	nodeStatus := &topolvmv2.NodeStorageState{}
	err := json.Unmarshal([]byte(status), nodeStatus)
	if err != nil {
		lvmdLogger.Errorf("unmarshal node status failed err %v", err)
		return
	}
	if err := l.topolvmController.UpdateStatus(nodeStatus); err != nil {
		lvmdLogger.Errorf("update status failed err %v", err)
	}
}

func (l *lvmdConfigController) checkUpdateClusterStatus(old, new *v1.ConfigMap) error {

	if new.Data[topolvm.VgStatusConfigMapKey] == "" {
		return nil
	}
	if old.Data[topolvm.VgStatusConfigMapKey] != new.Data[topolvm.VgStatusConfigMapKey] {
		status := new.Data[topolvm.VgStatusConfigMapKey]
		nodeStatus := &topolvmv2.NodeStorageState{}
		err := json.Unmarshal([]byte(status), nodeStatus)
		if err != nil {
			lvmdLogger.Errorf("unmarshal node status failed err %v", err)
			return err
		}
		if err := l.topolvmController.UpdateStatus(nodeStatus); err != nil {
			return errors.Wrapf(err, "update node %s status failed", nodeStatus.Node)
		}
	}
	return nil

}

func (l *lvmdConfigController) createTopolvmPluginNodeDeployment(node string) error {

	opNamespaceName := types.NamespacedName{Name: operator.OperatorSettingConfigMapName, Namespace: l.topolvmController.opConfig.OperatorNamespace}
	opConfig := &v1.ConfigMap{}
	err := l.topolvmController.client.Get(l.topolvmController.opManagerContext, opNamespaceName, opConfig)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("operator's configmap resource not found. will use default value or env var.")
			l.topolvmController.opConfig.Parameters = make(map[string]string)
		} else {
			// Error reading the object - requeue the request.
			return errors.Wrap(err, "failed to get operator's configmap")
		}
	} else {
		// Populate the operator's config
		l.topolvmController.opConfig.Parameters = opConfig.Data
	}

	param := csi.Param{}
	param.TopolvmImage = k8sutil.GetValue(l.topolvmController.opConfig.Parameters, "TOPOLVM_IMAGE", csitopo.DefaultTopolvmImage)
	param.RegistrarImage = k8sutil.GetValue(l.topolvmController.opConfig.Parameters, "CSI_REGISTRAR_IMAGE", csi.DefaultRegistrarImage)
	param.LivenessImage = k8sutil.GetValue(l.topolvmController.opConfig.Parameters, "CSI_LIVENESS_IMAGE", csi.DefaultLivenessImage)
	param.KubeletDirPath = k8sutil.GetValue(l.topolvmController.opConfig.Parameters, "KUBELET_ROOT_DIR", csi.DefaultKubeletDir)

	tp := csi.TemplateParam{
		Param:     param,
		Namespace: l.topolvmController.opConfig.OperatorNamespace,
	}

	topolvmPlugin, err := csi.TemplateToDeployment("topolvm-plugin", csitopo.CSITopolvmPluginTemplatePath, tp)
	if err != nil {
		return errors.Wrap(err, "failed to load topolvm provisioner deployment template")
	}

	topolvmPluginTolerations := csi.GetToleration(l.topolvmController.opConfig.Parameters, csitopo.TopolvmPluginTolerationsEnv, []v1.Toleration{})
	topolvmPluginNodeAffinity := csi.GetNodeAffinity(l.topolvmController.opConfig.Parameters, csitopo.TopolvmPluginNodeAffinityEnv, &v1.NodeAffinity{})
	csi.ApplyToPodSpec(&topolvmPlugin.Spec.Template.Spec, topolvmPluginNodeAffinity, topolvmPluginTolerations)
	csi.ApplyResourcesToContainers(l.topolvmController.opConfig.Parameters, csitopo.TopolvmPluginResource, &topolvmPlugin.Spec.Template.Spec)

	topolvmPlugin.Name = k8sutil.TruncateNodeName(topolvm.TopolvmNodeDeploymentFmt, node)
	nodeSelector := map[string]string{
		v1.LabelHostname: node,
	}
	topolvmPlugin.Spec.Template.Spec.NodeSelector = nodeSelector
	topolvmPlugin.OwnerReferences = []metav1.OwnerReference{*l.topolvmController.getRef()}
	lvmdName := k8sutil.TruncateNodeName(topolvm.LvmdConfigMapFmt, node)
	v := v1.Volume{Name: "lvmd-config-dir", VolumeSource: v1.VolumeSource{ConfigMap: &v1.ConfigMapVolumeSource{LocalObjectReference: v1.LocalObjectReference{Name: lvmdName}}}}
	topolvmPlugin.Spec.Template.Spec.Volumes = append(topolvmPlugin.Spec.Template.Spec.Volumes, v)
	_, err = k8sutil.CreateOrUpdateDeployment(l.topolvmController.opManagerContext, l.topolvmController.context.Clientset, topolvmPlugin)
	if err != nil {
		return errors.Wrapf(err, "failed to update topolvm provisioner deployment %q", topolvmPlugin.Name)
	}

	return nil
}
