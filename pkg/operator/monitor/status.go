package monitor

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	topolvmv2 "github.com/alauda/topolvm-operator/api/v2"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/coreos/pkg/capnslog"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var logger = capnslog.NewPackageLogger("topolvm/operator", "status")

type ClusterStatusChecker struct {
	context    *cluster.Context
	interval   time.Duration
	client     client.Client
	statusLock *sync.Mutex
	metric     chan *cluster.Metrics
}

func NewStatusChecker(context *cluster.Context, statusLock *sync.Mutex, metric chan *cluster.Metrics) *ClusterStatusChecker {
	return &ClusterStatusChecker{
		context:    context,
		client:     context.Client,
		interval:   cluster.CheckStatusInterval,
		statusLock: statusLock,
		metric:     metric,
	}
}

func (c *ClusterStatusChecker) CheckClusterStatus(namespacedName *types.NamespacedName, stopCh chan struct{}, ref *metav1.OwnerReference) {
	// check the status immediately before starting the loop
	c.checkStatus(namespacedName)
	for {
		select {
		case <-stopCh:
			logger.Infof("stopping monitoring of cluster status")
			return

		case <-time.After(c.interval):
			c.checkStatus(namespacedName)
			if err := EnableServiceMonitor(ref); err != nil {
				logger.Errorf("monitor failed err %s", err.Error())
			}

			if err := CreateOrUpdatePrometheusRule(ref); err != nil {
				logger.Errorf("create rule failed err %s", err.Error())
			}
		}
	}
}

func (c *ClusterStatusChecker) checkStatus(namespacedName *types.NamespacedName) {

	ctx := context.TODO()
	pods, err := c.context.Clientset.CoreV1().Pods(cluster.NameSpace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cluster.TopolvmComposeAttr, cluster.TopolvmComposeNode)})
	if err != nil && !kerrors.IsNotFound(err) {
		logger.Errorf("list topolvm node pod  failed %v", err)
	}

	var clusterMetric cluster.Metrics
	c.statusLock.Lock()
	defer c.statusLock.Unlock()
	topolvmCluster := &topolvmv2.TopolvmCluster{}
	err = c.context.Client.Get(ctx, *namespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("topolvm cluster resource not found. Ignoring since object must be deleted.")
			return
		}
		logger.Errorf("failed to get topolvm cluster %v", err)
		return
	}

	ready := false
	nodesStatus := make(map[string]*topolvmv2.NodeStorageState)
	nodes := make([]string, 0)

	if topolvmCluster.Spec.UseAllNodes {

		cms, err := c.context.Clientset.CoreV1().ConfigMaps(cluster.NameSpace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cluster.LvmdConfigMapLabelKey, cluster.LvmdConfigMapLabelValue)})
		if err != nil && !kerrors.IsNotFound(err) {
			logger.Errorf("list lvmd configmap failed %v", err)
		}
		for _, cm := range cms.Items {
			node := cm.GetAnnotations()[cluster.LvmdAnnotationsNodeKey]
			nodes = append(nodes, node)
		}

	} else {
		for _, item := range topolvmCluster.Spec.DeviceClasses {
			nodes = append(nodes, item.NodeName)
		}
	}

	for _, node := range nodes {
		found := false
		for _, item := range pods.Items {
			if item.Spec.NodeName == node {
				found = true
			} else {
				continue
			}

			n := topolvmv2.NodeStorageState{
				Node: item.Spec.NodeName,
			}

			nodeMetric := cluster.NodeStatusMetrics{
				Node: item.Spec.NodeName,
			}

			switch item.Status.Phase {
			case corev1.PodRunning:
				n.Phase = topolvmv2.ConditionReady
				nodeMetric.Status = 0
				ready = true
			case corev1.PodUnknown:
				n.Phase = topolvmv2.ConditionUnknown
				nodeMetric.Status = 1
			case corev1.PodFailed:
				n.Phase = topolvmv2.ConditionFailure
				nodeMetric.Status = 1
			case corev1.PodPending:
				n.Phase = topolvmv2.ConditionPending
				nodeMetric.Status = 1
			default:
				n.Phase = topolvmv2.ConditionUnknown
				nodeMetric.Status = 1
			}

			for _, s := range item.Status.ContainerStatuses {
				if !s.Ready {
					n.Phase = topolvmv2.ConditionFailure
					nodeMetric.Status = 1
					break
				}
			}

			nodesStatus[item.Spec.NodeName] = &n
			clusterMetric.NodeStatus = append(clusterMetric.NodeStatus, nodeMetric)
		}

		if !found {
			n := topolvmv2.NodeStorageState{
				Node:  node,
				Phase: topolvmv2.ConditionFailure,
			}
			nodeMetric := cluster.NodeStatusMetrics{
				Node:   node,
				Status: 1,
			}
			nodesStatus[node] = &n
			clusterMetric.NodeStatus = append(clusterMetric.NodeStatus, nodeMetric)
		}

	}

	for key := range nodesStatus {
		n, err := c.context.Clientset.CoreV1().Nodes().Get(ctx, key, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("failed to get node  %v", err)
			continue
		}
		for _, ele := range n.Status.Conditions {
			if ele.Type == corev1.NodeReady && ele.Status == corev1.ConditionUnknown {

				nodesStatus[key].Phase = topolvmv2.ConditionUnknown
				for index, n := range clusterMetric.NodeStatus {
					if n.Node == key {
						clusterMetric.NodeStatus[index].Status = 1
					}
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
			logger.Debugf("node %s, phase: %s", item.Node, topolvmCluster.Status.NodeStorageStatus[index].Phase)

		}
		if !found {
			clusterStatus.NodeStorageStatus = append(clusterStatus.NodeStorageStatus, *val)
		}
	}

	if ready {
		clusterStatus.Phase = topolvmv2.ConditionReady
		clusterMetric.ClusterStatus = 0
	} else {
		clusterMetric.ClusterStatus = 1
		clusterStatus.Phase = topolvmv2.ConditionFailure
	}

	if reflect.DeepEqual(topolvmCluster.Status, *clusterStatus) {
		logger.Debugf("no need to update cluster status")
		return
	} else {
		topolvmCluster.Status = *clusterStatus
	}

	logger.Debugf("start update cluster status and metric")
	clusterMetric.Cluster = topolvmCluster.Name
	c.metric <- &clusterMetric
	if err := k8sutil.UpdateStatus(c.context.Client, topolvmCluster); err != nil {
		logger.Errorf("failed to update cluster %q status. %v", namespacedName.Name, err)
	}

}
