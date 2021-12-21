package csi

import (
	_ "embed"
	"github.com/alauda/topolvm-operator/pkg/operator/csi"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"strconv"
)

var (
	DefaultRawDevicePluginImage = "quay.io/cephcsi/cephcsi:v3.4.0"
)

const (
	defaultLogLevel            uint8 = 0
	provisionerTolerationsEnv        = "CSI_PROVISIONER_TOLERATIONS"
	provisionerNodeAffinityEnv       = "CSI_PROVISIONER_NODE_AFFINITY"
	pluginTolerationsEnv             = "CSI_PLUGIN_TOLERATIONS"
	pluginNodeAffinityEnv            = "CSI_PLUGIN_NODE_AFFINITY"

	rawDevicePluginTolerationsEnv       = "CSI_RAW_DEVICE_PLUGIN_TOLERATIONS"
	rawDevicePluginNodeAffinityEnv      = "CSI_RAW_DEVICE_PLUGIN_NODE_AFFINITY"
	rawDeviceProvisionerTolerationsEnv  = "CSI_RAW_DEVICE_PROVISIONER_TOLERATIONS"
	rawDeviceProvisionerNodeAffinityEnv = "CSI_RAW_DEVICE_PROVISIONER_NODE_AFFINITY"

	rawDeviceProvisionerResource = "CSI_RAW_DEVICE_PROVISIONER_RESOURCE"
	rawDevicePluginResource      = "CSI_RAW_DEVICE_PLUGIN_RESOURCE"
	// default provisioner replicas
	defaultProvisionerReplicas int32 = 2

	csiRawDevicePlugin = "raw-device-plugin"

	csiRawDeviceProvisioner = "raw-device-provisioner"
)

var (
	CSIParam csi.Param

	EnableRawDevice = false
	//driver names
	RawDeviceDriverName string
)

var (
	//go:embed template/csi-rawdevice-plugin.yaml
	CSIRawDeviceNodeTemplatePath string
	//go:embed template/csi-rawdevice-provisioner.yaml
	CSIRawDeviceControllerTemplatePath string
	//go:embed template/raw-device-csi-driver.yaml
	RawDeviceCSIDriverTemplatePath string
)

func (r *CSIRawDeviceController) startDrivers(ver *version.Info, ownerInfo *k8sutil.OwnerInfo) error {
	logger.Info("start csi raw device driver")
	var (
		err                  error
		rawDevicePlugin      *apps.DaemonSet
		rawDeviceProvisioner *apps.Deployment
		rawDeviceCSIDriver   *storagev1.CSIDriver
	)

	tp := csi.TemplateParam{
		Param:     CSIParam,
		Namespace: r.opConfig.OperatorNamespace,
	}

	// default value `system-node-critical` is the highest available priority
	tp.PluginPriorityClassName = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PLUGIN_PRIORITY_CLASSNAME", "")

	// default value `system-cluster-critical` is applied for some
	// critical pods in cluster but less priority than plugin pods
	tp.ProvisionerPriorityClassName = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PROVISIONER_PRIORITY_CLASSNAME", "")

	logger.Infof("Kubernetes version is %s.%s", ver.Major, ver.Minor)

	logLevel := k8sutil.GetValue(r.opConfig.Parameters, "CSI_LOG_LEVEL", "")
	tp.LogLevel = defaultLogLevel
	if logLevel != "" {
		l, err := strconv.ParseUint(logLevel, 10, 8)
		if err != nil {
			logger.Errorf("failed to parse CSI_LOG_LEVEL. Defaulting to %d. %v", defaultLogLevel, err)
		} else {
			tp.LogLevel = uint8(l)
		}
	}

	tp.ProvisionerReplicas = defaultProvisionerReplicas
	nodes, err := r.context.Clientset.CoreV1().Nodes().List(r.opManagerContext, metav1.ListOptions{})
	if err == nil {
		if len(nodes.Items) == 1 {
			tp.ProvisionerReplicas = 1
		} else {
			replicas := k8sutil.GetValue(r.opConfig.Parameters, "CSI_PROVISIONER_REPLICAS", "2")
			r, err := strconv.ParseInt(replicas, 10, 32)
			if err != nil {
				logger.Errorf("failed to parse CSI_PROVISIONER_REPLICAS. Defaulting to %d. %v", defaultProvisionerReplicas, err)
			} else {
				tp.ProvisionerReplicas = int32(r)
			}
		}
	} else {
		logger.Errorf("failed to get nodes. Defaulting the number of replicas of provisioner pods to %d. %v", tp.ProvisionerReplicas, err)
	}

	if EnableRawDevice {
		rawDevicePlugin, err = csi.TemplateToDaemonSet("raw-device-node", CSIRawDeviceNodeTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load raw device plugin daemonset template")
		}

		rawDeviceProvisioner, err = csi.TemplateToDeployment("raw-device-provisioner", CSIRawDeviceControllerTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load raw device provisioner deployment template")
		}

		rawDeviceCSIDriver, err = csi.TemplateToCSIDriver("raw-device-csi-driver", RawDeviceCSIDriverTemplatePath, tp)
		if err != nil {
			return errors.Wrap(err, "failed to load raw device csi driver template")
		}
	}

	// get common provisioner tolerations and node affinity
	provisionerTolerations := csi.GetToleration(r.opConfig.Parameters, provisionerTolerationsEnv, []corev1.Toleration{})
	provisionerNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, provisionerNodeAffinityEnv, &corev1.NodeAffinity{})
	// get common plugin tolerations and node affinity
	pluginTolerations := csi.GetToleration(r.opConfig.Parameters, pluginTolerationsEnv, []corev1.Toleration{})
	pluginNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, pluginNodeAffinityEnv, &corev1.NodeAffinity{})

	if rawDevicePlugin != nil {
		rawDevicePluginTolerations := csi.GetToleration(r.opConfig.Parameters, rawDevicePluginTolerationsEnv, pluginTolerations)
		rawDevicePluginNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, rawDevicePluginNodeAffinityEnv, pluginNodeAffinity)
		csi.ApplyToPodSpec(&rawDevicePlugin.Spec.Template.Spec, rawDevicePluginNodeAffinity, rawDevicePluginTolerations)
		csi.ApplyResourcesToContainers(r.opConfig.Parameters, rawDevicePluginResource, &rawDevicePlugin.Spec.Template.Spec)
		err = ownerInfo.SetControllerReference(rawDevicePlugin)
		if err != nil {
			return errors.Wrapf(err, "failed to set owner reference to raw device plugin daemonset %q", rawDevicePlugin.Name)
		}
		err = k8sutil.CreateDaemonSet(r.opManagerContext, csiRawDevicePlugin, r.opConfig.OperatorNamespace, r.context.Clientset, rawDevicePlugin)
		if err != nil {
			return errors.Wrapf(err, "failed to start raw device daemonset %q", rawDevicePlugin.Name)
		}
	}

	if rawDeviceProvisioner != nil {
		rawDeviceProvisionerTolerations := csi.GetToleration(r.opConfig.Parameters, rawDeviceProvisionerTolerationsEnv, provisionerTolerations)
		rawDeviceProvisionerNodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, rawDeviceProvisionerNodeAffinityEnv, provisionerNodeAffinity)
		csi.ApplyToPodSpec(&rawDeviceProvisioner.Spec.Template.Spec, rawDeviceProvisionerNodeAffinity, rawDeviceProvisionerTolerations)
		csi.ApplyResourcesToContainers(r.opConfig.Parameters, rawDeviceProvisionerResource, &rawDeviceProvisioner.Spec.Template.Spec)
		err = ownerInfo.SetControllerReference(rawDeviceProvisioner)
		if err != nil {
			return errors.Wrapf(err, "failed to set owner reference to raw device provisioner deployment %q", rawDeviceProvisioner.Name)
		}
		antiAffinity := csi.GetPodAntiAffinity("app", csiRawDeviceProvisioner)
		rawDeviceProvisioner.Spec.Template.Spec.Affinity.PodAntiAffinity = &antiAffinity
		rawDeviceProvisioner.Spec.Strategy = apps.DeploymentStrategy{
			Type: apps.RecreateDeploymentStrategyType,
		}
		_, err = k8sutil.CreateOrUpdateDeployment(r.opManagerContext, r.context.Clientset, rawDeviceProvisioner)
		if err != nil {
			return errors.Wrapf(err, "failed to start raw device provisioner deployment %q", rawDeviceProvisioner.Name)
		}
		logger.Info("successfully started CSI raw device driver")
	}

	if rawDeviceCSIDriver != nil {
		err = k8sutil.CreateCSIDriver(r.opManagerContext, r.context.Clientset, rawDeviceCSIDriver)
		if err != nil {
			return errors.Wrapf(err, "failed to start raw device driver %q", rawDeviceProvisioner.Name)
		}
	}

	return nil
}

func (r *CSIRawDeviceController) stopDrivers(ver *version.Info) {
	if !EnableRawDevice {
		logger.Info("CSI raw device driver disabled")
		succeeded := r.deleteCSIDriverResources(ver, csiRawDevicePlugin, csiRawDeviceProvisioner, "rawdevice.nativestor.io")
		if succeeded {
			logger.Info("successfully removed CSI raw device driver")
		} else {
			logger.Error("failed to remove CSI raw device driver")
		}
	}
}

func (r *CSIRawDeviceController) deleteCSIDriverResources(ver *version.Info, daemonset, deployment, driverName string) bool {
	succeeded := true

	err := k8sutil.DeleteDaemonset(r.opManagerContext, r.context.Clientset, r.opConfig.OperatorNamespace, daemonset)
	if err != nil {
		logger.Errorf("failed to delete the %q. %v", daemonset, err)
		succeeded = false
	}

	err = k8sutil.DeleteDeployment(r.opManagerContext, r.context.Clientset, r.opConfig.OperatorNamespace, deployment)
	if err != nil {
		logger.Errorf("failed to delete the %q. %v", deployment, err)
		succeeded = false
	}

	err = k8sutil.DeleteCSIDriver(r.opManagerContext, r.context.Clientset, driverName)
	if err != nil {
		logger.Errorf("failed to delete the %q. %v", driverName, err)
		succeeded = false
	}
	return succeeded
}
