package controllers

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/discover"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/alauda/topolvm-operator/pkg/operator/node"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	opcontroller "github.com/rook/rook/pkg/operator/ceph/controller"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "topolvm-operator-config-controller"
)

var configLogger = capnslog.NewPackageLogger("topolvm/operator", "config-setting")

type ReconcileConfig struct {
	client           client.Client
	context          *cluster.Context
	config           cluster.OperatorConfig
	opManagerContext context.Context
}

// predicateOpController is the predicate function to trigger reconcile on operator configuration cm change
func predicateController(client client.Client) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if cm, ok := e.Object.(*v1.ConfigMap); ok {
				return cm.Name == cluster.OperatorSettingConfigMapName
			}
			return false
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			if old, ok := e.ObjectOld.(*v1.ConfigMap); ok {
				if new, ok := e.ObjectNew.(*v1.ConfigMap); ok {
					if old.Name == cluster.OperatorSettingConfigMapName && new.Name == cluster.OperatorSettingConfigMapName {
						// We still want to reconcile the operator manager if the configmap is updated
						return true
					}
				}
			}

			return false
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			if cm, ok := e.Object.(*v1.ConfigMap); ok {
				if cm.Name == cluster.OperatorSettingConfigMapName {
					configLogger.Debug("operator configmap deleted, not reconciling")
					return false
				}
			}
			return false
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func newReconciler(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, config cluster.OperatorConfig) reconcile.Reconciler {
	return &ReconcileConfig{
		client:           mgr.GetClient(),
		context:          context,
		config:           config,
		opManagerContext: opManagerContext,
	}
}

// Add creates a new Operator configuration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig cluster.OperatorConfig) error {
	return add(mgr, newReconciler(mgr, context, opManagerContext, opConfig))
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	configLogger.Infof("%s successfully started", controllerName)

	// Watch for ConfigMap (operator config)
	err = c.Watch(&source.Kind{
		Type: &v1.ConfigMap{TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: v1.SchemeGroupVersion.String()}}}, &handler.EnqueueRequestForObject{}, predicateController(mgr.GetClient()))
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a CephClient object and makes changes based on the state read
// and what is in the CephClient.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ReconcileConfig) Reconcile(context context.Context, request reconcile.Request) (reconcile.Result, error) {
	// workaround because the rook logging mechanism is not compatible with the controller-runtime logging interface
	reconcileResponse, err := c.reconcile(request)
	if err != nil {
		configLogger.Errorf("failed to reconcile %v", err)
	}

	return reconcileResponse, err
}

func (c *ReconcileConfig) reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the operator's configmap
	opConfig := &v1.ConfigMap{}
	configLogger.Debugf("reconciling %s", request.NamespacedName)
	err := c.client.Get(c.opManagerContext, request.NamespacedName, opConfig)
	if err != nil {
		if kerrors.IsNotFound(err) {
			configLogger.Debug("operator's configmap resource not found. will use default value or env var.")
		} else {
			// Error reading the object - requeue the request.
			return opcontroller.ImmediateRetryResult, errors.Wrap(err, "failed to get operator's configmap")
		}
	} else {
		// Populate the operator's config
		c.config.Parameters = opConfig.Data
	}

	// Reconcile discovery daemon
	err = c.updateCsiDriver()
	if err != nil {
		return opcontroller.ImmediateRetryResult, err
	}

	err = c.starDiscoverDaemonset()
	if err != nil {
		return opcontroller.ImmediateRetryResult, err
	}
	// Reconcile webhook secret
	// This is done in the predicate function

	configLogger.Infof("%s done reconciling", controllerName)
	return reconcile.Result{}, nil
}

func (c *ReconcileConfig) updateCsiDriver() error {
	kubeletRootDir := k8sutil.GetValue(c.config.Parameters, cluster.KubeletRootPathEnv, cluster.CSIKubeletRootDir)
	if kubeletRootDir != cluster.CSIKubeletRootDir {
		if err := node.UpdateNodeDeploymentCSIKubeletRootPath(c.context.Clientset, kubeletRootDir); err != nil {
			clusterLogger.Errorf("updater csi kubelet path failed err:%s", err.Error())
			return err
		}
	}
	return nil
}

func (c *ReconcileConfig) starDiscoverDaemonset() error {
	enableDiscoverDevices := k8sutil.GetValue(c.config.Parameters, cluster.DiscoverDevicesEnv, cluster.EnableDiscoverDevices)
	if enableDiscoverDevices == "true" {
		if err := discover.MakeDiscoverDevicesDaemonset(c.context.Clientset, cluster.DiscoverAppName, c.config.Image, false); err != nil {
			return err
		}
	}
	return nil

}
