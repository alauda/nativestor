/*
Copyright 2021 The Topolvm-Operator Authors. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	topolvmv1 "github.com/alauda/topolvm-operator/api/v2"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/controller"
	"github.com/alauda/topolvm-operator/pkg/operator/csidriver"
	"github.com/alauda/topolvm-operator/pkg/operator/discover"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/alauda/topolvm-operator/pkg/operator/monitor"
	"github.com/alauda/topolvm-operator/pkg/operator/node"
	"github.com/alauda/topolvm-operator/pkg/operator/psp"
	"github.com/alauda/topolvm-operator/pkg/operator/volumectr"
	"github.com/alauda/topolvm-operator/pkg/operator/volumegroup"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"sync"
	"time"
)

var (
	clusterLogger = capnslog.NewPackageLogger("topolvm/operator", "topolvm-cluster-reconciler")
)

// TopolvmClusterReconciler reconciles a TopolvmCluster object
type TopolvmClusterReconciler struct {
	scheme              *runtime.Scheme
	context             *cluster.Context
	namespacedName      *types.NamespacedName
	ClusterController   *ClusterController
	configMapController *ConfigMapController
	statusLock          sync.Mutex
	interval            time.Duration
	stopCh              chan struct{}
	metric              chan *cluster.Metrics
	reflock             sync.Mutex
	clusterRef          *metav1.OwnerReference
}

func NewTopolvmClusterReconciler(scheme *runtime.Scheme, context *cluster.Context, operatorImage string, checkInterval time.Duration, metricUpdater chan *cluster.Metrics) *TopolvmClusterReconciler {

	return &TopolvmClusterReconciler{
		scheme:            scheme,
		context:           context,
		ClusterController: NewClusterContoller(context, operatorImage),
		interval:          checkInterval,
		metric:            metricUpdater,
	}
}

// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=topolvmclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=topolvmclusters/status,verbs=get;update;patch

func (r *TopolvmClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// your logic here
	clusterLogger.Debugf("start reconcile")
	return r.reconcile(req)
}

func (r *TopolvmClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topolvmv1.TopolvmCluster{}).
		Complete(r)
}

func (r *TopolvmClusterReconciler) updateRef(ref *metav1.OwnerReference) {
	r.reflock.Lock()
	defer r.reflock.Unlock()
	r.clusterRef = ref
}

func (r *TopolvmClusterReconciler) getRef() *metav1.OwnerReference {
	r.reflock.Lock()
	defer r.reflock.Unlock()
	return r.clusterRef
}

func (r *TopolvmClusterReconciler) reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Pass object name and namespace
	if request.Namespace != cluster.NameSpace {
		clusterLogger.Errorf("namespace %s of topovlm cluster:%s is not equal to operator namespace:%s", request.Namespace, request.NamespacedName.Name, cluster.NameSpace)
		return reconcile.Result{}, nil
	}

	topolvmClusters := &topolvmv1.TopolvmClusterList{}
	err := r.context.Client.List(context.TODO(), topolvmClusters, &client.ListOptions{})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to list topolvm cluster")
	}

	if topolvmClusters == nil || topolvmClusters.Items == nil {
		return reconcile.Result{}, errors.New("no topolvm cluster instance existing")
	}

	oldest := topolvmv1.TopolvmCluster{}
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
			clusterLogger.Error("only support one cluster instance")
			return reconcile.Result{}, nil
		}
	}

	// Fetch the topolvmCluster instance
	topolvmCluster := &topolvmv1.TopolvmCluster{}
	err = r.context.Client.Get(context.TODO(), request.NamespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			clusterLogger.Debug("topolvm cluster resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, errors.Wrap(err, "failed to get topolvm cluster")
	}

	if r.namespacedName == nil {
		r.namespacedName = &request.NamespacedName
	}
	cluster.ClusterName = request.NamespacedName.Name
	cluster.TopolvmImage = topolvmCluster.Spec.TopolvmVersion

	// Set a finalizer so we can do cleanup before the object goes away
	err = controller.AddFinalizerIfNotPresent(r.context.Client, topolvmCluster)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to add finalizer")
	}

	// DELETE: the CR was deleted
	if !topolvmCluster.GetDeletionTimestamp().IsZero() {
		clusterLogger.Infof("deleting topolvm cluster %q", topolvmCluster.Name)
		r.ClusterController.lastCluster = nil
		// Remove finalizer
		err = removeFinalizer(r.context.Client, request.NamespacedName)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to remove finalize")
		}

		err = RemoveNodeCapacityAnnotations(r.context.Clientset)
		if err != nil {
			clusterLogger.Errorf("failed to remove node capacity annotations err %v", err)
			return reconcile.Result{}, errors.Wrap(err, "failed to remove node capacity annotations")
		}

		close(r.stopCh)
		// Return and do not requeue. Successful deletion.
		return reconcile.Result{}, nil
	}

	// Create the controller owner ref
	ref, err := controller.GetControllerObjectOwnerReference(topolvmCluster, r.scheme)
	if err != nil || ref == nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to get controller %q owner reference", topolvmCluster.Name)
	}
	r.updateRef(ref)

	if err := r.checkStorageConfig(topolvmCluster); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "check storage config failed")
	}
	//start configmap controller
	if r.configMapController == nil {
		r.configMapController = NewConfigMapController(cluster.NewContext(), cluster.NameSpace, ref, r)
		r.configMapController.Start()
	}
	r.configMapController.UpdateRef(ref)

	if r.stopCh == nil {
		r.stopCh = make(chan struct{})
		go r.checkClusterStatus()
		go r.ClusterController.StartOperatorSettingsWatch(r.stopCh)
	}

	// Do reconcile here!
	if err := r.ClusterController.onAdd(topolvmCluster, ref); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to reconcile cluster %q", topolvmCluster.Name)
	}
	// Return and do not requeue
	return reconcile.Result{}, nil
}

func (r *TopolvmClusterReconciler) checkStorageConfig(topolvmCluster *topolvmv1.TopolvmCluster) error {

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

func (r *TopolvmClusterReconciler) checkClusterStatus() {

	r.checkStatus()
	for {
		select {
		case <-r.stopCh:
			clusterLogger.Infof("stopping monitoring of ceph status")
			return

		case <-time.After(r.interval):
			r.checkStatus()
			if err := monitor.EnableServiceMonitor(r.getRef()); err != nil {
				clusterLogger.Errorf("monitor failed err %s", err.Error())
			}

			k8sClusterName, err := r.getK8sClusterName()
			if err != nil {
				clusterLogger.Error(err.Error())
			} else {
				if err := monitor.CreateOrUpdatePrometheusRule(r.getRef(), k8sClusterName); err != nil {
					clusterLogger.Errorf("create rule failed err %s", err.Error())
				}
			}

		}
	}
}

func (r *TopolvmClusterReconciler) getK8sClusterName() (string, error) {
	cm, err := r.context.Clientset.CoreV1().ConfigMaps(cluster.K8sClusterNamespace).Get(context.TODO(), cluster.K8sClusterConfigmap, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "get k8s cluster info configmap failed")
	}
	return cm.Data["clusterName"], nil
}

func (r *TopolvmClusterReconciler) checkStatus() {

	ctx := context.TODO()
	pods, err := r.context.Clientset.CoreV1().Pods(cluster.NameSpace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cluster.TopolvmComposeAttr, cluster.TopolvmComposeNode)})
	if err != nil && !kerrors.IsNotFound(err) {
		clusterLogger.Errorf("list topolvm node pod  failed %v", err)
	}

	var clusterMetric cluster.Metrics
	r.statusLock.Lock()
	defer r.statusLock.Unlock()
	topolvmCluster := &topolvmv1.TopolvmCluster{}
	err = r.context.Client.Get(ctx, *r.namespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			clusterLogger.Debug("topolvm cluster resource not found. Ignoring since object must be deleted.")
			return
		}
		clusterLogger.Errorf("failed to get topolvm cluster %v", err)
		return
	}

	ready := false
	nodesStatus := make(map[string]*topolvmv1.NodeStorageState, 0)
	if len(pods.Items) == 0 {
		for _, item := range topolvmCluster.Spec.DeviceClasses {

			node := topolvmv1.NodeStorageState{
				Node:  item.NodeName,
				Phase: topolvmv1.ConditionFailure,
			}
			nodeMetric := cluster.NodeStatusMetrics{
				Node:   item.NodeName,
				Status: 1,
			}
			nodesStatus[item.NodeName] = &node
			clusterMetric.NodeStatus = append(clusterMetric.NodeStatus, nodeMetric)

		}
	} else {
		for _, deviceclass := range topolvmCluster.Spec.DeviceClasses {
			found := false
			for _, item := range pods.Items {
				if item.Spec.NodeName == deviceclass.NodeName {
					found = true
				} else {
					continue
				}

				node := topolvmv1.NodeStorageState{
					Node: item.Spec.NodeName,
				}

				nodeMetric := cluster.NodeStatusMetrics{
					Node: item.Spec.NodeName,
				}

				switch item.Status.Phase {
				case corev1.PodRunning:
					node.Phase = topolvmv1.ConditionReady
					nodeMetric.Status = 0
					ready = true
				case corev1.PodUnknown:
					node.Phase = topolvmv1.ConditionUnknown
					nodeMetric.Status = 1
				case corev1.PodFailed:
					node.Phase = topolvmv1.ConditionFailure
					nodeMetric.Status = 1
				case corev1.PodPending:
					node.Phase = topolvmv1.ConditionPending
					nodeMetric.Status = 1
				default:
					node.Phase = topolvmv1.ConditionUnknown
					nodeMetric.Status = 1
				}
				nodesStatus[item.Spec.NodeName] = &node
				clusterMetric.NodeStatus = append(clusterMetric.NodeStatus, nodeMetric)
			}

			if !found {
				node := topolvmv1.NodeStorageState{
					Node:  deviceclass.NodeName,
					Phase: topolvmv1.ConditionFailure,
				}
				nodeMetric := cluster.NodeStatusMetrics{
					Node:   deviceclass.NodeName,
					Status: 1,
				}
				nodesStatus[deviceclass.NodeName] = &node
				clusterMetric.NodeStatus = append(clusterMetric.NodeStatus, nodeMetric)
			}

		}

	}

	for key, _ := range nodesStatus {
		node, err := r.context.Clientset.CoreV1().Nodes().Get(ctx, key, metav1.GetOptions{})
		if err != nil {
			clusterLogger.Errorf("failed to get node  %v", err)
			continue
		}
		switch node.Status.Phase {
		case corev1.NodeTerminated:
			nodesStatus[key].Phase = topolvmv1.ConditionUnknown
			for index, ele := range clusterMetric.NodeStatus {
				if ele.Node == key {
					clusterMetric.NodeStatus[index].Status = 1
				}
			}

		}
	}
	clusterStatus := topolvmCluster.Status.DeepCopy()
	for key, val := range nodesStatus {
		found := false
		for index, item := range clusterStatus.NodeStorageStatus {
			if item.Node == key {
				found = true
			} else {
				continue
			}
			clusterStatus.NodeStorageStatus[index].Phase = val.Phase
			clusterLogger.Debugf("node %s, phase: %s", item.Node, topolvmCluster.Status.NodeStorageStatus[index].Phase)

		}
		if !found {
			clusterStatus.NodeStorageStatus = append(clusterStatus.NodeStorageStatus, *val)
		}
	}

	if ready {
		clusterStatus.Phase = topolvmv1.ConditionReady
		clusterMetric.ClusterStatus = 0
	} else {
		clusterMetric.ClusterStatus = 1
		clusterStatus.Phase = topolvmv1.ConditionFailure
	}

	if reflect.DeepEqual(topolvmCluster.Status, *clusterStatus) {
		clusterLogger.Debugf("no need to update cluster status")
		return
	} else {
		topolvmCluster.Status = *clusterStatus
	}

	clusterLogger.Debugf("start update cluster status and metric")
	clusterMetric.Cluster = topolvmCluster.Name
	r.metric <- &clusterMetric
	if err := k8sutil.UpdateStatus(r.context.Client, topolvmCluster); err != nil {
		clusterLogger.Errorf("failed to update cluster %q status. %v", r.namespacedName.Name, err)
	}

}

func (r *TopolvmClusterReconciler) UpdateStatus(state *topolvmv1.NodeStorageState) error {
	r.statusLock.Lock()
	defer r.statusLock.Unlock()
	topolvmCluster := &topolvmv1.TopolvmCluster{}

	err := r.context.Client.Get(context.TODO(), *r.namespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			clusterLogger.Debug("TopolvmCluster resource not found. Ignoring since object must be deleted.")
			return nil
		}
		clusterLogger.Errorf("failed to retrieve topolvm cluster %q to update topolvm cluster status. %v", r.namespacedName.Name, err)
		return errors.Wrapf(err, "failed to retrieve topolvm cluster %q to update topolvm cluster status ", r.namespacedName.Name)
	}

	length := len(topolvmCluster.Status.NodeStorageStatus)
	nodeFound := false
	for i := 0; i < length; i++ {
		if topolvmCluster.Status.NodeStorageStatus[i].Node == state.Node {
			nodeFound = true
			topolvmCluster.Status.NodeStorageStatus[i].FailClasses = state.FailClasses
			topolvmCluster.Status.NodeStorageStatus[i].SuccessClasses = state.SuccessClasses
			break
		}
	}

	if !nodeFound {
		topolvmCluster.Status.NodeStorageStatus = append(topolvmCluster.Status.NodeStorageStatus, *state)
	}

	clusterLogger.Debugf("UpdateStatus phase:%s", topolvmCluster.Status.Phase)
	if err := k8sutil.UpdateStatus(r.context.Client, topolvmCluster); err != nil {
		clusterLogger.Errorf("failed to update cluster %q status. %v", r.namespacedName.Name, err)
		return errors.Wrapf(err, "failed to update cluster %q status", r.namespacedName.Name)
	}
	return nil

}

// ClusterController controls an instance of a topolvm cluster
type ClusterController struct {
	context       *cluster.Context
	lastCluster   *topolvmv1.TopolvmCluster
	operatorImage string
}

func NewClusterContoller(ctx *cluster.Context, operatorImage string) *ClusterController {

	return &ClusterController{
		context:       ctx,
		operatorImage: operatorImage,
	}

}

func (c *ClusterController) UseAllNodeAndDevices() bool {
	return c.lastCluster.Spec.UseAllNodes
}

func (c *ClusterController) onAdd(topolvmCluster *topolvmv1.TopolvmCluster, ref *metav1.OwnerReference) error {

	if cluster.IsOperatorHub {

		err := csidriver.CheckTopolvmCsiDriverExisting(c.context.Clientset, ref)
		if err != nil {
			logger.Errorf("CheckTopolvmCsiDriverExisting failed err %v", err)
			return err
		}
		err = checkAndCreatePsp(c.context.Clientset, ref)
		if err != nil {
			logger.Errorf("checkAndCreatePsp failed err %v", err)
			return err
		}
	}

	if err := c.startDiscoverDaemonset(topolvmCluster, ref, topolvmCluster.Spec.UseLoop); err != nil {
		return errors.Wrap(err, "start discover daemonset failed")
	}

	// Start the main topolvm cluster orchestration
	if err := c.startPrepareVolumeGroupJob(topolvmCluster, ref); err != nil {
		return errors.Wrap(err, "start prepare volume group failed")
	}

	if err := c.startTopolvmControllerDeployment(topolvmCluster, ref); err != nil {
		return errors.Wrap(err, "start create or update topolvm controller deployment  failed")
	}

	if err := c.startReplaceNodeDeployment(topolvmCluster, ref); err != nil {
		return errors.Wrap(err, "start replace node deployment  failed")
	}

	c.lastCluster = topolvmCluster

	return nil

}

func (c *ClusterController) startReplaceNodeDeployment(topolvmCluster *topolvmv1.TopolvmCluster, ref *metav1.OwnerReference) error {

	ctx := context.TODO()
	deploys, err := c.context.Clientset.AppsV1().Deployments(cluster.NameSpace).List(ctx, metav1.ListOptions{})
	if err != nil {
		clusterLogger.Errorf("list deployment failed err:%v", err)
		return err
	}

	for i := 0; i < len(deploys.Items); i++ {

		if strings.HasPrefix(deploys.Items[i].ObjectMeta.Name, cluster.TopolvmNodeDeploymentNamePrefix) {

			if deploys.Items[i].Spec.Template.Spec.Containers[0].Image == topolvmCluster.Spec.TopolvmVersion {
				clusterLogger.Info("node deployment no change need not reconcile")
				return nil
			}
			containers := deploys.Items[i].Spec.Template.Spec.Containers
			for j := 0; j < len(containers); j++ {
				containers[j].Image = topolvmCluster.Spec.TopolvmVersion
			}
			_, err := c.context.Clientset.AppsV1().Deployments(cluster.NameSpace).Update(ctx, &deploys.Items[i], metav1.UpdateOptions{})
			if err != nil {
				clusterLogger.Errorf("update deployment:%s image failed err:%v", deploys.Items[i].ObjectMeta.Name, err)

			}
		}
	}

	return nil
}

func (c *ClusterController) startPrepareVolumeGroupJob(topolvmCluster *topolvmv1.TopolvmCluster, ref *metav1.OwnerReference) error {

	storage := topolvmCluster.Spec.Storage

	// if device class not change then check if has fail class that should be recreate
	if c.lastCluster != nil && reflect.DeepEqual(c.lastCluster.Spec.Storage, storage) {
		go func() {
			for _, ele := range topolvmCluster.Status.NodeStorageStatus {
				if len(ele.FailClasses) > 0 || len(ele.SuccessClasses) == 0 {
					clusterLogger.Infof("node%s has fail classes recreate job again", ele.Node)
					if err := volumegroup.MakeAndRunJob(c.context.Clientset, ele.Node, c.operatorImage, ref); err != nil {
						clusterLogger.Errorf("create job for node failed %s", ele.Node)
					}
				} else {
					clusterLogger.Infof("class info nothing change no need to start prepare volumegroup job")
				}
			}
		}()
		return nil
	}

	if topolvmCluster.Spec.UseAllNodes {
		nodes, err := c.context.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			clusterLogger.Errorf("list node failed err %v", err)
			return err
		}
		for _, ele := range nodes.Items {
			if err := volumegroup.MakeAndRunJob(c.context.Clientset, ele.Name, c.operatorImage, ref); err != nil {
				clusterLogger.Errorf("create job for node failed %s", ele.Name)
			}
		}
		return nil
	}

	// first should create job anyway
	logger.Info("start make prepare volume group job")
	go func() {
		if storage.DeviceClasses != nil {
			for _, ele := range storage.DeviceClasses {
				if err := volumegroup.MakeAndRunJob(c.context.Clientset, ele.NodeName, c.operatorImage, ref); err != nil {
					clusterLogger.Errorf("create job for node failed %s", ele.NodeName)
				}
			}
		}
	}()

	return nil
}

func (c *ClusterController) RestartJob(node string, ref *metav1.OwnerReference) error {

	return volumegroup.MakeAndRunJob(c.context.Clientset, node, c.operatorImage, ref)
}

func (c *ClusterController) startDiscoverDaemonset(topolvmCluster *topolvmv1.TopolvmCluster, ref *metav1.OwnerReference, useLoop bool) error {

	ctx := context.TODO()
	daemonset, err := c.context.Clientset.AppsV1().DaemonSets(cluster.NameSpace).Get(ctx, cluster.DiscoverAppName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		logger.Errorf("failed to detect daemonset:%s. err:%v", cluster.DiscoverAppName, err)
		return errors.Wrap(err, "failed to detect daemonset")
	} else if err == nil {
		if daemonset.Spec.Template.Spec.Containers[0].Image == c.operatorImage {
			clusterLogger.Info("discover daemonset no change need not reconcile")
			return nil
		}
		length := len(daemonset.Spec.Template.Spec.Containers)
		for i := 0; i < length; i++ {
			daemonset.Spec.Template.Spec.Containers[i].Image = c.operatorImage
		}
		_, err := c.context.Clientset.AppsV1().DaemonSets(daemonset.Namespace).Update(ctx, daemonset, metav1.UpdateOptions{})
		if err != nil {
			clusterLogger.Errorf("update discover daemonset image failed err %v", err)
			return errors.Wrap(err, "update discover daemonset image failed")
		} else {
			clusterLogger.Infof("update discover daemonset image to %s", c.operatorImage)
			return nil
		}
	}

	return discover.MakeDiscoverDevicesDaemonset(c.context.Clientset, cluster.DiscoverAppName, c.operatorImage, useLoop, ref)
}

func (c *ClusterController) startTopolvmControllerDeployment(topolvmCluster *topolvmv1.TopolvmCluster, ref *metav1.OwnerReference) error {

	ctx := context.TODO()
	deployment, err := c.context.Clientset.AppsV1().Deployments(cluster.NameSpace).Get(ctx, cluster.TopolvmControllerDeploymentName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		clusterLogger.Errorf("failed to detect deployment:%s. err:%v", cluster.TopolvmControllerDeploymentName, err)
		return errors.Wrap(err, "failed to detect deployment")
	} else if err == nil {
		if deployment.Spec.Template.Spec.Containers[0].Image == topolvmCluster.Spec.TopolvmVersion {
			clusterLogger.Info("controller deployment no change need not reconcile")
			return nil
		}
		length := len(deployment.Spec.Template.Spec.Containers)
		for i := 0; i < length; i++ {
			deployment.Spec.Template.Spec.Containers[i].Image = topolvmCluster.Spec.TopolvmVersion
		}
		_, err := c.context.Clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			clusterLogger.Errorf("update topolvm controller image failed err %v", err)
			return errors.Wrap(err, "update topolvm controller image failed")
		} else {
			clusterLogger.Infof("update topolvm contorller image to %s", topolvmCluster.Spec.TopolvmVersion)
			return nil
		}

	}

	clusterLogger.Info("start create controller deployment")
	if err := volumectr.CreateReplaceTopolvmControllerDeployment(c.context.Clientset, ref); err != nil {
		clusterLogger.Errorf("create and replace controller deployment failed err: %v", err)
		return errors.Wrap(err, "create and replace controller deployment failed")
	}

	return nil
}

// StartOperatorSettingsWatch starts the operator settings watcher
func (c *ClusterController) StartOperatorSettingsWatch(stopCh chan struct{}) {
	k8sutil.StartOperatorSettingsWatch(c.context, cluster.NameSpace, cluster.OperatorSettingConfigMapName,
		c.operatorConfigChange,
		func(oldObj, newObj interface{}) {
			if reflect.DeepEqual(oldObj, newObj) {
				return
			}
			c.operatorConfigChange(newObj)
		}, nil, stopCh)
}

func (c *ClusterController) operatorConfigChange(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		clusterLogger.Warningf("Expected ConfigMap but handler received %T. %#v", obj, obj)
		return
	}

	clusterLogger.Infof("ConfigMap %q changes detected. Updating configurations", cm.Name)

	c.updateCsiDriver()

}

func (c *ClusterController) updateCsiDriver() error {
	kubeletRootDir, err := k8sutil.GetOperatorSetting(c.context.Clientset, cluster.OperatorSettingConfigMapName, cluster.KubeletRootPathEnv, cluster.CSIKubeletRootDir)
	if err != nil {
		return err
	}

	if kubeletRootDir != cluster.CSIKubeletRootDir {
		if err := node.UpdateNodeDeploymentCSIKubeletRootPath(c.context.Clientset, kubeletRootDir); err != nil {
			clusterLogger.Errorf("updater csi kubelet path failed err:%s", err.Error())
			return err
		}
	}
	return nil

}

// removeFinalizer removes a finalizer
func removeFinalizer(client client.Client, name types.NamespacedName) error {
	topolvmCluster := &topolvmv1.TopolvmCluster{}
	err := client.Get(context.TODO(), name, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			clusterLogger.Debug("TopolvmCluster resource not found. Ignoring since object must be deleted.")
			return nil
		}
		return errors.Wrapf(err, "failed to retrieve topolvm cluster %q to remove finalizer", name.Name)
	}
	err = controller.RemoveFinalizer(client, topolvmCluster)
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
	for index, node := range nodes.Items {
		for key := range node.Annotations {
			if strings.HasPrefix(key, cluster.CapacityKeyPrefix) {

				delete(nodes.Items[index].Annotations, key)
				oldJSON, err := json.Marshal(nodeList.Items[index])
				if err != nil {
					return err
				}
				newJSON, err := json.Marshal(nodes.Items[index])
				if err != nil {
					return err
				}
				patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, corev1.Node{})
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

	existing, err := psp.CheckPspExisting(clientset, cluster.TopolvmNodePsp)
	if err != nil {
		return errors.Wrapf(err, "check psp %s failed", cluster.TopolvmNodePsp)
	}

	if !existing {
		err = psp.CreateTopolvmNodePsp(clientset, ref)
		if err != nil {
			return errors.Wrapf(err, "create psp %s failed", cluster.TopolvmNodePsp)
		}
	} else {
		logger.Infof("psp %s existing", cluster.TopolvmNodePsp)
	}

	existing, err = psp.CheckPspExisting(clientset, cluster.TopolvmPrepareVgPsp)
	if err != nil {
		return errors.Wrapf(err, "check psp %s failed", cluster.TopolvmPrepareVgPsp)
	}

	if !existing {
		err = psp.CreateTopolvmPrepareVgPsp(clientset, ref)
		if err != nil {
			return errors.Wrapf(err, "create psp %s failed", cluster.TopolvmPrepareVgPsp)
		}
	} else {
		logger.Infof("psp %s existing", cluster.TopolvmPrepareVgPsp)
	}

	return nil
}
