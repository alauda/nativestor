package csi

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator"
	controllerutil "github.com/alauda/topolvm-operator/pkg/operator/controller"
	"github.com/alauda/topolvm-operator/pkg/operator/csi"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strconv"
)

const (
	controllerName = "raw-device-csi-controller"
)

var (
	logger = capnslog.NewPackageLogger("github.com/alauda/topolvm-operator", "raw-device-csi")
)

func Add(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig operator.OperatorConfig) error {
	return add(mgr, newReconciler(mgr, context, opManagerContext, opConfig))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig operator.OperatorConfig) reconcile.Reconciler {
	return &CSIRawDeviceController{
		client:           mgr.GetClient(),
		context:          context,
		opConfig:         opConfig,
		opManagerContext: opManagerContext,
	}
}

type CSIRawDeviceController struct {
	client           client.Client
	context          *cluster.Context
	opManagerContext context.Context
	opConfig         operator.OperatorConfig
}

func (r *CSIRawDeviceController) Reconcile(context context.Context, request reconcile.Request) (reconcile.Result, error) {
	// workaround because the rook logging mechanism is not compatible with the controller-runtime logging interface
	reconcileResponse, err := r.reconcile(request)
	if err != nil {
		logger.Errorf("failed to reconcile %v", err)
	}

	return reconcileResponse, err
}

func (r *CSIRawDeviceController) reconcile(request reconcile.Request) (reconcile.Result, error) {

	opNamespaceName := types.NamespacedName{Name: operator.OperatorSettingConfigMapName, Namespace: r.opConfig.OperatorNamespace}
	opConfig := &v1.ConfigMap{}
	err := r.client.Get(r.opManagerContext, opNamespaceName, opConfig)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("operator's configmap resource not found. will use default value or env var.")
			r.opConfig.Parameters = make(map[string]string)
		} else {
			// Error reading the object - requeue the request.
			return controllerutil.ImmediateRetryResult, errors.Wrap(err, "failed to get operator's configmap")
		}
	} else {
		// Populate the operator's config
		r.opConfig.Parameters = opConfig.Data
	}

	ownerRef, err := k8sutil.GetDeploymentOwnerReference(r.opManagerContext, r.context.Clientset, os.Getenv(k8sutil.PodNameEnvVar), r.opConfig.OperatorNamespace)
	if err != nil {
		logger.Warningf("could not find deployment owner reference to assign to csi drivers. %v", err)
	}
	if ownerRef != nil {
		blockOwnerDeletion := false
		ownerRef.BlockOwnerDeletion = &blockOwnerDeletion
	}

	ownerInfo := k8sutil.NewOwnerInfoWithOwnerRef(ownerRef, r.opConfig.OperatorNamespace)

	serverVersion, err := r.context.Clientset.Discovery().ServerVersion()
	if err != nil {
		return controllerutil.ImmediateRetryResult, errors.Wrap(err, "failed to get server version")
	}

	err = r.validateAndConfigureDrivers(serverVersion, ownerInfo)
	if err != nil {
		return controllerutil.ImmediateRetryResult, errors.Wrap(err, "failed configure raw device csi")
	}
	return reconcile.Result{}, nil

}

func (r *CSIRawDeviceController) validateAndConfigureDrivers(serverVersion *version.Info, ownerInfo *k8sutil.OwnerInfo) error {
	var (
		err error
	)

	if err = r.setParams(); err != nil {
		return errors.Wrapf(err, "failed to configure CSI parameters")
	}

	if err = validateCSIParam(); err != nil {
		return errors.Wrapf(err, "failed to validate CSI parameters")
	}

	if EnableRawDevice {
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			if err = r.startDrivers(serverVersion, ownerInfo); err != nil {
				logger.Errorf("failed to start raw device csi drivers, will retry starting csi drivers %d more times. %v", maxRetries-i-1, err)
			} else {
				break
			}
		}

		return errors.Wrap(err, "failed to start raw device csi drivers")
	}
	r.stopDrivers(serverVersion)

	return nil
}

func (r *CSIRawDeviceController) setParams() error {
	var err error

	if EnableRawDevice, err = strconv.ParseBool(k8sutil.GetValue(r.opConfig.Parameters, "RAW_DEVICE_ENABLE", "false")); err != nil {
		return errors.Wrap(err, "unable to parse value for 'OPERATOR_CSI_RAW_DEVICE_ENABLE'")
	}

	CSIParam.RawDeviceImage = k8sutil.GetValue(r.opConfig.Parameters, "RAW_DEVICE_IMAGE", DefaultRawDevicePluginImage)
	CSIParam.RegistrarImage = k8sutil.GetValue(r.opConfig.Parameters, "CSI_REGISTRAR_IMAGE", csi.DefaultRegistrarImage)
	CSIParam.ProvisionerImage = k8sutil.GetValue(r.opConfig.Parameters, "CSI_PROVISIONER_IMAGE", csi.DefaultProvisionerImage)
	CSIParam.LivenessImage = k8sutil.GetValue(r.opConfig.Parameters, "CSI_LIVENESS_IMAGE", csi.DefaultLivenessImage)
	CSIParam.KubeletDirPath = k8sutil.GetValue(r.opConfig.Parameters, "KUBELET_ROOT_DIR", csi.DefaultKubeletDir)

	return nil
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	logger.Infof("%s successfully started", controllerName)

	// Watch for ConfigMap (operator config)
	err = c.Watch(&source.Kind{
		Type: &v1.ConfigMap{TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: v1.SchemeGroupVersion.String()}}}, &handler.EnqueueRequestForObject{}, predicateController())
	if err != nil {
		return err
	}

	return nil
}

func validateCSIParam() error {
	if len(CSIParam.RawDeviceImage) == 0 {
		return errors.New("missing csi raw device plugin image")
	}
	if len(CSIParam.RegistrarImage) == 0 {
		return errors.New("missing csi registrar image")
	}
	if len(CSIParam.ProvisionerImage) == 0 {
		return errors.New("missing csi provisioner image")
	}
	if len(CSIParam.LivenessImage) == 0 {
		return errors.New("missing csi liveness image")
	}
	return nil
}
