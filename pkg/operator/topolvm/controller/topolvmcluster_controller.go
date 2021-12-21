package controller

import (
	"context"
	"encoding/json"
	"fmt"
	topolvmv2 "github.com/alauda/topolvm-operator/apis/topolvm/v2"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"
	"github.com/alauda/topolvm-operator/pkg/operator"
	ctr "github.com/alauda/topolvm-operator/pkg/operator/controller"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/alauda/topolvm-operator/pkg/operator/topolvm/csidriver"
	"github.com/alauda/topolvm-operator/pkg/operator/topolvm/metric"
	"github.com/alauda/topolvm-operator/pkg/operator/topolvm/monitor"
	"github.com/alauda/topolvm-operator/pkg/operator/topolvm/psp"
	"github.com/alauda/topolvm-operator/pkg/operator/topolvm/volumegroup"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"sync"
	"time"
)

const (
	controllerName                = "topolvm-controller"
	ResyncPeriodOfTopolvmInformer = time.Hour
)

var (
	logger = capnslog.NewPackageLogger("github.com/alauda/topolvm-operator", "topolvm-controller")
)

func Add(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig operator.OperatorConfig) error {

	metricsCh := make(chan *topolvm.Metrics)
	if err := mgr.Add(metric.NewMetricsExporter(metricsCh)); err != nil {
		return err
	}
	return add(mgr, newReconciler(mgr, context, opManagerContext, opConfig, metricsCh))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig operator.OperatorConfig, metric chan *topolvm.Metrics) reconcile.Reconciler {

	topolvmCtr := &TopolvmController{
		scheme:           mgr.GetScheme(),
		client:           mgr.GetClient(),
		context:          context,
		opConfig:         opConfig,
		opManagerContext: opManagerContext,
		metric:           metric,
	}
	topolvmCtr.lvmdController = newLvmdController(topolvmCtr)
	return topolvmCtr
}

func (r *TopolvmController) Reconcile(context context.Context, request reconcile.Request) (reconcile.Result, error) {
	// workaround because the rook logging mechanism is not compatible with the controller-runtime logging interface
	reconcileResponse, err := r.reconcile(request)
	if err != nil {
		logger.Errorf("failed to reconcile %v", err)
	}

	return reconcileResponse, err
}

func (r *TopolvmController) reconcile(request reconcile.Request) (reconcile.Result, error) {

	if err := r.verifyTopolvmCluster(request); err != nil {
		logger.Errorf("verify topolvm cluster failed %v", err)
		return reconcile.Result{}, err
	}

	// Fetch the topolvmCluster instance
	topolvmCluster := &topolvmv2.TopolvmCluster{}
	err := r.context.Client.Get(context.TODO(), request.NamespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("topolvm cluster resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, errors.Wrap(err, "failed to get topolvm cluster")
	}

	topolvm.ClusterName = request.NamespacedName.Name
	topolvm.TopolvmImage = topolvmCluster.Spec.TopolvmVersion
	topolvm.CertsSecret = topolvmCluster.Spec.CertsSecret

	// Set a finalizer so we can do cleanup before the object goes away
	err = ctr.AddFinalizerIfNotPresent(r.context.Client, topolvmCluster)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
	}

	r.namespacedName = request.NamespacedName
	// DELETE: the CR was deleted
	if !topolvmCluster.GetDeletionTimestamp().IsZero() {

		err = r.reconcileDelete(topolvmCluster)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to clean cluster")
		}
		return reconcile.Result{}, nil
	}

	// Create the controller owner ref
	ref, err := ctr.GetControllerObjectOwnerReference(topolvmCluster, r.scheme)
	if err != nil || ref == nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get controller %q owner reference", topolvmCluster.Name)
	}
	r.updateRef(ref)
	if err := r.checkStorageConfig(topolvmCluster); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "check storage config failed")
	}
	if err := r.onAdd(topolvmCluster, ref); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile cluster %q", topolvmCluster.Name)
	}

	return reconcile.Result{}, nil
}

func (r *TopolvmController) reconcileDelete(topolvmCluster *topolvmv2.TopolvmCluster) error {

	nsName := r.namespacedName
	logger.Infof("deleting topolvm cluster %q", topolvmCluster.Name)
	r.stopCheckClusterStatus()
	// Remove finalizer
	err := removeFinalizer(r.context.Client, nsName)
	if err != nil {
		return errors.Wrap(err, "failed to remove finalize")
	}
	r.updateCluster(nil)
	err = r.cleanCluster()
	if err != nil {
		return errors.Wrap(err, "clean cluster failed")
	}
	return nil

}

func (r *TopolvmController) startClusterMonitor() error {
	if r.health == nil {
		internalCtx, internalCancel := context.WithCancel(r.opManagerContext)
		r.health = &clusterHealth{
			internalCtx:    internalCtx,
			internalCancel: internalCancel,
		}

		r.statusChecker = monitor.NewStatusChecker(r.health.internalCtx, r.context, &r.statusLock, r.metric, r.namespacedName)
		go r.statusChecker.CheckClusterStatus()
	}

	return nil
}

func (r *TopolvmController) stopCheckClusterStatus() {
	if r.health != nil {
		if r.health.internalCtx.Err() == nil {
			r.health.internalCancel()
			r.health = nil
		}
	}
}

func (r *TopolvmController) verifyTopolvmCluster(request reconcile.Request) error {
	// Pass object name and namespace
	if request.Namespace != r.opConfig.OperatorNamespace {
		return fmt.Errorf("namespace %s of topovlm cluster:%s is not equal to operator namespace:%s", request.Namespace, request.NamespacedName.Name, topolvm.NameSpace)

	}
	topolvmClusters := &topolvmv2.TopolvmClusterList{}
	err := r.context.Client.List(context.TODO(), topolvmClusters, &client.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list topolvm cluster")
	}

	if topolvmClusters == nil || topolvmClusters.Items == nil {
		return errors.New("no topolvm cluster instance existing")
	}

	oldest := topolvmv2.TopolvmCluster{}
	if len(topolvmClusters.Items) > 1 {
		for index, c := range topolvmClusters.Items {
			if index == 0 {
				oldest = c
			}
			if c.CreationTimestamp.Before(&oldest.CreationTimestamp) {
				oldest = c
			}
		}

		if request.NamespacedName.Name != oldest.Name {
			return errors.New("only support one cluster instance")
		}
	}
	return nil

}

type TopolvmController struct {
	scheme           *runtime.Scheme
	client           client.Client
	context          *cluster.Context
	opManagerContext context.Context
	opConfig         operator.OperatorConfig
	statusChecker    *monitor.ClusterStatusChecker
	namespacedName   types.NamespacedName
	cluster          *topolvmv2.TopolvmCluster
	health           *clusterHealth
	statusLock       sync.Mutex
	refLock          sync.Mutex
	ref              *metav1.OwnerReference
	metric           chan *topolvm.Metrics
	lvmdController   *lvmdConfigController
}

type clusterHealth struct {
	internalCtx    context.Context
	internalCancel context.CancelFunc
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	logger.Infof("%s successfully started", controllerName)

	err = c.Watch(&source.Kind{
		Type: &topolvmv2.TopolvmCluster{TypeMeta: metav1.TypeMeta{Kind: "TopolvmCluster", APIVersion: topolvmv2.SchemeGroupVersion.String()}}}, &handler.EnqueueRequestForObject{}, predicateController())
	if err != nil {
		return err
	}

	return nil
}

func (r *TopolvmController) cleanCluster() error {

	err := RemoveNodeCapacityAnnotations(r.context.Clientset)
	if err != nil {
		logger.Errorf("failed to remove node capacity annotations err %v", err)
		return errors.Wrap(err, "failed to remove node capacity annotations")
	}

	if err = csidriver.DeleteTopolvmCsiDriver(r.context.Clientset); err != nil {
		logger.Errorf("clean csi driver failed err:%s", err.Error())
		return err
	}
	return nil
}

func (r *TopolvmController) checkStorageConfig(topolvmCluster *topolvmv2.TopolvmCluster) error {

	if topolvmCluster.Spec.Storage.UseAllNodes && topolvmCluster.Spec.Storage.DeviceClasses != nil {
		return errors.New("should not both config use all node and deviceclasses ")
	}

	if !topolvmCluster.Spec.Storage.UseAllNodes && topolvmCluster.Spec.UseAllDevices {
		return errors.New("should not config useAllNodes false but useAllDevice true")
	}

	if topolvmCluster.Spec.UseAllDevices && topolvmCluster.Spec.Storage.Devices != nil {
		return errors.New("should not config useAllNodes true but config storage.devices")
	}

	if topolvmCluster.Spec.Storage.UseAllNodes {
		if topolvmCluster.Spec.Storage.VolumeGroupName == "" || topolvmCluster.Spec.Storage.ClassName == "" {
			return errors.New("if use all nodes ,volumeGroupName and className must be define")
		}
	}

	return nil
}

func (r *TopolvmController) UpdateStatus(state *topolvmv2.NodeStorageState) error {
	r.statusLock.Lock()
	defer r.statusLock.Unlock()
	topolvmCluster := &topolvmv2.TopolvmCluster{}

	err := r.context.Client.Get(context.TODO(), r.namespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("TopolvmCluster resource not found. Ignoring since object must be deleted.")
			return nil
		}
		logger.Errorf("failed to retrieve topolvm cluster %q to update topolvm cluster status. %v", r.namespacedName.Name, err)
		return errors.Wrapf(err, "failed to retrieve topolvm cluster %q to update topolvm cluster status ", r.namespacedName.Name)
	}

	length := len(topolvmCluster.Status.NodeStorageStatus)
	nodeFound := false
	for i := 0; i < length; i++ {
		if topolvmCluster.Status.NodeStorageStatus[i].Node == state.Node {
			nodeFound = true
			topolvmCluster.Status.NodeStorageStatus[i] = *state
			break
		}
	}

	if !nodeFound {
		topolvmCluster.Status.NodeStorageStatus = append(topolvmCluster.Status.NodeStorageStatus, *state)
	}

	logger.Debugf("UpdateStatus phase:%s", topolvmCluster.Status.Phase)
	if err := k8sutil.UpdateStatus(r.context.Client, topolvmCluster); err != nil {
		logger.Errorf("failed to update cluster %q status. %v", r.namespacedName.Name, err)
		return errors.Wrapf(err, "failed to update cluster %q status", r.namespacedName.Name)
	}
	return nil

}

func (r *TopolvmController) UseAllNodeAndDevices() bool {
	return r.getCluster().Spec.UseAllNodes
}

func (r *TopolvmController) onAdd(topolvmCluster *topolvmv2.TopolvmCluster, ref *metav1.OwnerReference) error {
	if topolvm.IsOperatorHub {

		err := csidriver.CheckTopolvmCsiDriverExisting(r.context.Clientset, ref)
		if err != nil {
			logger.Errorf("CheckTopolvmCsiDriverExisting failed err %v", err)
			return err
		}
		err = checkAndCreatePsp(r.context.Clientset, ref)
		if err != nil {
			logger.Errorf("checkAndCreatePsp failed err %v", err)
			return err
		}
	}
	// Start the main topolvm cluster orchestration
	if err := r.startPrepareVolumeGroupJob(topolvmCluster, ref); err != nil {
		return errors.Wrap(err, "start prepare volume group failed")
	}

	if r.getCluster() == nil {
		r.updateCluster(topolvmCluster.DeepCopy())
		r.lvmdController.start()
		err := r.startClusterMonitor()
		if err != nil {
			return errors.Wrap(err, "start cluster monitor failed")
		}
	}

	r.updateCluster(topolvmCluster.DeepCopy())

	return nil

}

func (r *TopolvmController) startPrepareVolumeGroupJob(topolvmCluster *topolvmv2.TopolvmCluster, ref *metav1.OwnerReference) error {

	storage := topolvmCluster.Spec.Storage
	// if device class not change then check if has fail class that should be recreate
	if r.getCluster() != nil && reflect.DeepEqual(r.getCluster().DeepCopy().Spec.Storage, storage) {
		go func() {
			for _, ele := range topolvmCluster.Status.NodeStorageStatus {
				if len(ele.FailClasses) > 0 || len(ele.SuccessClasses) == 0 {
					logger.Infof("node%s has fail classes recreate job again", ele.Node)
					if err := volumegroup.MakeAndRunJob(r.context.Clientset, ele.Node, r.opConfig.Image, ref); err != nil {
						logger.Errorf("create job for node failed %s", ele.Node)
					}
				} else {
					logger.Infof("class info nothing change no need to start prepare volumegroup job")
				}
			}
		}()
		return nil
	}

	// first should create job anyway
	logger.Info("start make prepare volume group job")
	go func() {
		if topolvmCluster.Spec.UseAllNodes {
			nodes, err := r.context.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Errorf("list node failed err %v", err)
			}
			for _, ele := range nodes.Items {
				if err := volumegroup.MakeAndRunJob(r.context.Clientset, ele.Name, r.opConfig.Image, ref); err != nil {
					logger.Errorf("create job for node failed %s", ele.Name)
				}
			}
		}

		if storage.DeviceClasses != nil {
			for _, ele := range storage.DeviceClasses {
				if err := volumegroup.MakeAndRunJob(r.context.Clientset, ele.NodeName, r.opConfig.Image, ref); err != nil {
					logger.Errorf("create job for node failed %s", ele.NodeName)
				}
			}
		}
	}()

	return nil
}

func (r *TopolvmController) RestartJob(node string, ref *metav1.OwnerReference) error {

	return volumegroup.MakeAndRunJob(r.context.Clientset, node, r.opConfig.Image, ref)
}

func (r *TopolvmController) updateCluster(c *topolvmv2.TopolvmCluster) {
	r.refLock.Lock()
	defer r.refLock.Unlock()
	r.cluster = c
}

func (r *TopolvmController) getCluster() *topolvmv2.TopolvmCluster {
	r.refLock.Lock()
	defer r.refLock.Unlock()
	return r.cluster.DeepCopy()
}

func (r *TopolvmController) updateRef(ref *metav1.OwnerReference) {
	r.refLock.Lock()
	defer r.refLock.Unlock()
	r.ref = ref
}

func (r *TopolvmController) getRef() *metav1.OwnerReference {
	r.refLock.Lock()
	defer r.refLock.Unlock()
	return r.ref
}

// removeFinalizer removes a finalizer
func removeFinalizer(client client.Client, name types.NamespacedName) error {
	topolvmCluster := &topolvmv2.TopolvmCluster{}
	err := client.Get(context.TODO(), name, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("TopolvmCluster resource not found. Ignoring since object must be deleted.")
			return nil
		}
		return errors.Wrapf(err, "failed to retrieve topolvm cluster %q to remove finalizer", name.Name)
	}
	err = ctr.RemoveFinalizer(client, topolvmCluster)
	if err != nil {
		return errors.Wrap(err, "failed to remove finalizer")
	}

	return nil
}

func RemoveNodeCapacityAnnotations(clientset kubernetes.Interface) error {

	ctx := context.TODO()
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		errors.Wrapf(err, "failed list node")
	}
	nodes := nodeList.DeepCopy()
	for index, n := range nodes.Items {
		for key := range n.Annotations {
			if strings.HasPrefix(key, topolvm.CapacityKeyPrefix) {

				delete(nodes.Items[index].Annotations, key)
				oldJSON, err := json.Marshal(nodeList.Items[index])
				if err != nil {
					return err
				}
				newJSON, err := json.Marshal(nodes.Items[index])
				if err != nil {
					return err
				}
				patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, v1.Node{})
				if err != nil {
					return err
				}
				_, err = clientset.CoreV1().Nodes().Patch(ctx, nodes.Items[index].Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
				if err != nil {
					logger.Errorf("patch node %s capacity annotations failed err: %v", nodeList.Items[index].Name, err)
				}
			}
		}
	}
	return err
}

func checkAndCreatePsp(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	existing, err := psp.CheckPspExisting(clientset, topolvm.TopolvmNodePsp)
	if err != nil {
		return errors.Wrapf(err, "check psp %s failed", topolvm.TopolvmNodePsp)
	}

	if !existing {
		err = psp.CreateTopolvmNodePsp(clientset, ref)
		if err != nil {
			return errors.Wrapf(err, "create psp %s failed", topolvm.TopolvmNodePsp)
		}
	} else {
		logger.Infof("psp %s existing", topolvm.TopolvmNodePsp)
	}

	existing, err = psp.CheckPspExisting(clientset, topolvm.TopolvmPrepareVgPsp)
	if err != nil {
		return errors.Wrapf(err, "check psp %s failed", topolvm.TopolvmPrepareVgPsp)
	}

	if !existing {
		err = psp.CreateTopolvmPrepareVgPsp(clientset, ref)
		if err != nil {
			return errors.Wrapf(err, "create psp %s failed", topolvm.TopolvmPrepareVgPsp)
		}
	} else {
		logger.Infof("psp %s existing", topolvm.TopolvmPrepareVgPsp)
	}

	return nil
}
