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
	"github.com/alauda/topolvm-operator/pkg/operator/node"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var logger = capnslog.NewPackageLogger("topolvm/operator", "lvmd-config")

type ConfigMapController struct {
	context    *cluster.Context
	namespace  string
	ref        *metav1.OwnerReference
	clusterCtr *TopolvmClusterReconciler
}

// NewClientController create controller for watching client custom resources created
func NewConfigMapController(context *cluster.Context, namespace string, ref *metav1.OwnerReference, controller *TopolvmClusterReconciler) *ConfigMapController {
	return &ConfigMapController{
		context:    context,
		namespace:  namespace,
		ref:        ref,
		clusterCtr: controller,
	}
}

func (c *ConfigMapController) Start() {
	go func() {
		stopChan := make(chan struct{})
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGTERM)
		c.StartWatch(stopChan)
		<-sigc
		logger.Infof("shutdown signal received, exiting...")
		close(stopChan)
	}()

}

func (c *ConfigMapController) UpdateRef(ref *metav1.OwnerReference) {
	c.ref = ref
}

// Watch watches for instances of Client custom resources and acts on them
func (c *ConfigMapController) StartWatch(stopCh <-chan struct{}) {

	resourceHandlerFuncs := cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAdd,
		UpdateFunc: c.onUpdate,
		DeleteFunc: c.onDelete,
	}

	watchlist := cache.NewListWatchFromClient(c.context.Clientset.CoreV1().RESTClient(), string(v1.ResourceConfigMaps), c.namespace, fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.ConfigMap{},
		time.Second*0,
		resourceHandlerFuncs,
	)

	go controller.Run(stopCh)

}

func (c *ConfigMapController) onAdd(obj interface{}) {

	logger.Debugf("got configmap start process")

	cm, err := getClientObject(obj)
	if err != nil {
		logger.Errorf("failed to get client object. %v", err)
		return
	}

	if !strings.HasPrefix(cm.ObjectMeta.Name, cluster.LvmdConfigMapNamePrefix) {
		logger.Debugf("configmap %s is not for lvmd", cm.ObjectMeta.Name)
		return
	}

	cm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*c.ref}
	_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		logger.Errorf("failed update cm:%s  own ref", cm.Name)
	}

	c.updateClusterStatus(cm)

	if _, ok := cm.Data[cluster.LvmdConfigMapKey]; !ok {
		return
	}

	nodeName := getNodeName(cm)
	if nodeName == "" {
		logger.Error("can not get node name")
		return
	}

	if checkingDeploymentExisting(c.context, nodeName) {
		return
	}

	createNodeDeployment(c.context, cm.ObjectMeta.Name, nodeName, c.ref)
}

func (c *ConfigMapController) onUpdate(oldObj, newobj interface{}) {

	oldCm, err := getClientObject(oldObj)
	if err != nil {
		logger.Errorf("failed to get old client object. %v", err)
		return
	}
	if !strings.HasPrefix(oldCm.ObjectMeta.Name, cluster.LvmdConfigMapNamePrefix) {
		logger.Debugf("configmap %s is not for lvmd", oldCm.ObjectMeta.Name)
		return
	}

	newCm, err := getClientObject(newobj)
	if err != nil {
		logger.Errorf("failed to get new client object. %v", err)
		return
	}

	if newCm.OwnerReferences == nil {
		newCm.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*c.ref}
		_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespace).Update(context.TODO(), newCm, metav1.UpdateOptions{})
		if err != nil {
			logger.Errorf("failed update cm:%s  own ref", newCm.Name)
		}
	}

	err = c.checkUpdateClusterStatus(oldCm, newCm)
	if err != nil {
		logger.Errorf("update cluster failed err %v", err)
	}

	nodeName := getNodeName(newCm)
	if nodeName == "" {
		logger.Error("can not get node name")
		return
	}

	if _, ok := oldCm.Data[cluster.LocalDiskCMData]; ok {
		if oldCm.Data[cluster.LocalDiskCMData] != newCm.Data[cluster.LocalDiskCMData] && c.clusterCtr.clusterController.UseAllNodeAndDevices() {
			c.clusterCtr.clusterController.RestartJob(nodeName, c.ref)
		}
	}

	if _, ok := newCm.Data[cluster.LvmdConfigMapKey]; !ok {

		nodeName := getNodeName(newCm)
		if nodeName == "" {
			return
		}
		logger.Errorf("node %s all volume groups are not available", nodeName)
		return
	}

	if checkingDeploymentExisting(c.context, nodeName) {
		if oldCm.Data[cluster.LvmdConfigMapKey] == newCm.Data[cluster.LvmdConfigMapKey] {
			logger.Infof("cm%s  update but data not change no need to update node deployment", oldCm.ObjectMeta.Name)
			return
		}
		replaceNodePod(c.context, nodeName)
	} else {
		createNodeDeployment(c.context, newCm.ObjectMeta.Name, nodeName, c.ref)
	}
}

func (c *ConfigMapController) checkUpdateClusterStatus(old, new *v1.ConfigMap) error {

	if old.Data[cluster.VgStatusConfigMapKey] != new.Data[cluster.VgStatusConfigMapKey] {
		status := new.Data[cluster.VgStatusConfigMapKey]
		nodeStatus := &topolvmv1.NodeStorageState{}
		err := json.Unmarshal([]byte(status), nodeStatus)
		if err != nil {
			logger.Errorf("unmarshal node status failed err %v", err)
			return err
		}
		if err := c.clusterCtr.UpdateStatus(nodeStatus); err != nil {
			return errors.Wrapf(err, "update node %s status failed", nodeStatus.Node)
		}
	}
	return nil

}

func (c *ConfigMapController) onDelete(obj interface{}) {
	// nothing
}

func checkingDeploymentExisting(contextd *cluster.Context, nodeName string) bool {

	deploymentName := cluster.TopolvmNodeDeploymentNamePrefix + nodeName
	existing, err := node.CheckNodeDeploymentIsExisting(contextd.Clientset, deploymentName)
	if err != nil {
		return false
	}
	return existing
}

func replaceNodePod(contextd *cluster.Context, nodeName string) {

	deploymentName := cluster.TopolvmNodeDeploymentNamePrefix + nodeName
	ctx := context.TODO()
	pods, err := contextd.Clientset.CoreV1().Pods(cluster.NameSpace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cluster.AppAttr, deploymentName)})
	if err != nil {
		logger.Errorf("list pod with label %s failed %v", deploymentName, err)
	}
	for _, val := range pods.Items {
		if err := contextd.Clientset.CoreV1().Pods(cluster.NameSpace).Delete(ctx, val.ObjectMeta.Name, metav1.DeleteOptions{}); err != nil {
			logger.Errorf("delete pod %s failed err %v", val.ObjectMeta.Name, err)
		}
	}
}

func createNodeDeployment(context *cluster.Context, configmap, nodeName string, ref *metav1.OwnerReference) {

	deploymentName := cluster.TopolvmNodeDeploymentNamePrefix + nodeName
	err := node.CreateReplaceDeployment(context.Clientset, deploymentName, configmap, nodeName, ref)
	if err != nil {
		logger.Errorf("create topolvm node deployment %s  failed, err:%v ", deploymentName, err)
	}
}

func getNodeName(cm *v1.ConfigMap) string {

	nodeName, ok := cm.Labels[cluster.NodeAttr]
	if !ok {
		logger.Error("can not get node name")
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

func (c *ConfigMapController) updateClusterStatus(cm *v1.ConfigMap) {

	status := cm.Data[cluster.VgStatusConfigMapKey]
	nodeStatus := &topolvmv1.NodeStorageState{}
	err := json.Unmarshal([]byte(status), nodeStatus)
	if err != nil {
		logger.Errorf("unmarshal node status failed err %v", err)
		return
	}
	if err := c.clusterCtr.UpdateStatus(nodeStatus); err != nil {
		logger.Errorf("update status failed err %v", err)
	}
}
