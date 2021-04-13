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
	topolvmv1 "github.com/alauda/topolvm-operator/api/v1"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/node"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"strings"
	"time"
)

var logger = capnslog.NewPackageLogger("topolvm/operator", "lvmd-config")

type ConfigMapController struct {
	context    *cluster.Context
	namespace  string
	ref        *metav1.OwnerReference
	clusterCtr *ClusterController
}

// NewClientController create controller for watching client custom resources created
func NewConfigMapController(context *cluster.Context, namespace string, ref *metav1.OwnerReference, controller *ClusterController) *ConfigMapController {
	return &ConfigMapController{
		context:    context,
		namespace:  namespace,
		ref:        ref,
		clusterCtr: controller,
	}
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

	if _, ok := cm.Data[cluster.LvmdConfigMapKey]; !ok {

		status := cm.Data[cluster.VgStatusConfigMapKey]
		nodeStatus := &topolvmv1.NodeStorageState{}
		err = json.Unmarshal([]byte(status), nodeStatus)
		if err != nil {
			logger.Errorf("unmarshal node status failed err %v", err)
			return
		}
		if err := c.clusterCtr.UpdateStatus(nodeStatus); err != nil {
			logger.Errorf("update node %s status failed err %v", nodeStatus.Node, err)
		}
		return

	}

	c.createReplaceDeployment(cm)
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

	if _, ok := newCm.Data[cluster.LvmdConfigMapKey]; !ok {

		err := c.updateClusterStatus(oldCm, newCm)
		if err != nil {
			logger.Errorf("update cluster failed err %v", err)
		}
		return

	}

	if oldCm.Data[cluster.LvmdConfigMapKey] == newCm.Data[cluster.LvmdConfigMapKey] {

		err := c.updateClusterStatus(oldCm, newCm)
		if err != nil {
			logger.Errorf("update cluster failed err %v", err)
		}
		logger.Infof("cm%s  update but data not change no need to update node deployment", oldCm.ObjectMeta.Name)
		return
	}

	c.createReplaceDeployment(newCm)

}

func (c *ConfigMapController) updateClusterStatus(old, new *v1.ConfigMap) error {

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

func (c *ConfigMapController) createReplaceDeployment(cm *v1.ConfigMap) {

	nodeName, ok := cm.Annotations[cluster.LvmdAnnotationsNodeKey]
	if !ok {
		logger.Error("can not get node name")
		return
	}
	logger.Debugf("start create node deployment for node:%s", nodeName)

	if nodeName != "" {
		deploymentName := cluster.TopolvmNodeDeploymentNamePrefix + nodeName
		err := node.CreateReplaceDeployment(c.context.Clientset, deploymentName, cm.ObjectMeta.Name, nodeName, c.ref)
		if err != nil {
			logger.Errorf("create topolvm node deployment %s  failed, err:%v ", deploymentName, err)
		}
		status := cm.Data[cluster.VgStatusConfigMapKey]
		nodeStatus := &topolvmv1.NodeStorageState{}
		err = json.Unmarshal([]byte(status), nodeStatus)
		if err != nil {
			logger.Errorf("unmarshal node status failed err %v", err)
			return
		}
		if err := c.clusterCtr.UpdateStatus(nodeStatus); err != nil {
			logger.Errorf("update status failed err %v", err)
		}
	}

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
