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
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/alauda/topolvm-operator/pkg/util/sys"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ClassExpandWaring     = "expand warning"
	ClassExpandError      = "expand error"
	ClassCreateSuccessful = "create successful"
	ClassCreateFail       = "create failed"
	DeviceStateError      = "error"
)

var vgLogger = capnslog.NewPackageLogger("topolvm/operator", "prepare-vg-controller")

type PrePareVg struct {
	nodeName    string
	namespace   string
	context     *cluster.Context
	nodeDevices topolvmv1.NodeDevices
}

func NewPrepareVgController(nodeName string, nameSpace string, context *cluster.Context) *PrePareVg {
	return &PrePareVg{
		nodeName:  nodeName,
		namespace: nameSpace,
		context:   context,
	}
}

func (c *PrePareVg) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	defer func() {
		os.Exit(0)
	}()

	topolvmCluster := &topolvmv1.TopolvmCluster{}
	err := c.context.Client.Get(context.TODO(), req.NamespacedName, topolvmCluster)
	if err != nil {
		if kerrors.IsNotFound(err) {
			vgLogger.Debug("topolvm cluster resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, errors.Wrap(err, "failed to get topolvm cluster")
	}

	// get node class info
	for _, dev := range topolvmCluster.Spec.DeviceClasses {
		if dev.NodeName == c.nodeName {
			c.nodeDevices = dev
		}
	}

	logger.Info("start provision vg")
	err = c.provision()
	if err != nil {
		logger.Errorf("provision vg has some err %v", err)
	}

	return ctrl.Result{}, err
}

func (c *PrePareVg) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topolvmv1.TopolvmCluster{}).
		Complete(c)

}

func (c *PrePareVg) provision() error {

	// check existing vg and check if need expand

	nodeStatus := topolvmv1.NodeStorageState{}

	disks, err := sys.GetAvailableDevices(c.context)
	if err != nil {
		vgLogger.Errorf("can not list disk err:%s", err)
		return err
	}

	sucVgs := make([]topolvmv1.DeviceClass, 0)

	// record the status of each class
	sucClassMap := make(map[string]*topolvmv1.ClassState)
	failClassMap := make(map[string]*topolvmv1.ClassState)

	// get current class status
	ctx := context.TODO()
	cmname := k8sutil.TruncateNodeName(cluster.LvmdConfigMapFmt, c.nodeDevices.NodeName)
	cm, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, cmname, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		vgLogger.Warningf("failed to detect configmap %s. %+v", cmname, err)
		return err
	} else if err == nil {

		logger.Info("cm is existing check if need update")

		err := json.Unmarshal([]byte(cm.Data[cluster.VgStatusConfigMapKey]), &nodeStatus)
		if err != nil {
			vgLogger.Errorf("unmarshal confimap status data failed %+v ", err)
			return err
		}

		sucClassMap = getVgNameMap(nodeStatus.SuccessClasses)
		failClassMap = getVgNameMap(nodeStatus.FailClasses)

		for _, dev := range c.nodeDevices.DeviceClasses {

			if _, ok := sucClassMap[dev.VgName]; ok {
				// check if need expand
				err := c.checkVgIfExpand(&dev, sucClassMap)
				if err != nil {
					logger.Errorf("checkVgIfExpand vg:%s failed err %v", dev.VgName, err)
				}
				continue
			}

			if _, ok := failClassMap[dev.VgName]; ok {
				// check need recreate
				suc := c.createVgRetry(disks, &dev, sucClassMap, failClassMap)
				if suc {
					sucVgs = append(sucVgs, dev)
				}
				continue
			}

			// create new vg
			suc := c.createVg(disks, &dev, sucClassMap, failClassMap)
			if suc {
				sucVgs = append(sucVgs, dev)
			}

		}

		err = c.updateLvmdConf(cm, sucVgs)
		if err != nil {
			return errors.Wrap(err, "update lvmd conf failed")
		}

		err = updateVgStatus(cm, &nodeStatus, sucClassMap, failClassMap)
		if err != nil {
			return errors.Wrap(err, "update vg status failed")
		}

		_, err = c.context.Clientset.CoreV1().ConfigMaps(c.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			vgLogger.Errorf("update lvmd configmap failed err:%+v", err)
			return errors.Wrap(err, "update lvmd configmap failed")
		}

		return nil

	}

	//todo should distinguish the created vg between cluster and other user

	logger.Info("start create lvmd configmap")
	// list existing volume group
	vgs, err := sys.GetVolumeGroups(c.context.Executor)
	if err != nil {
		vgLogger.Errorf("list volume groups failed err %v", err)
		return err
	}

	for _, dev := range c.nodeDevices.DeviceClasses {

		if _, ok := vgs[dev.VgName]; ok {

			failClassMap[dev.VgName] = &topolvmv1.ClassState{Name: dev.ClassName, State: ClassCreateFail + " vg existing"}

		} else {
			suc := c.createVg(disks, &dev, sucClassMap, failClassMap)
			if suc {
				sucVgs = append(sucVgs, dev)
			}
		}

	}

	// create cm for node to notify operator to create or update node deployment and update TopolvmCluster status

	annotations := make(map[string]string)
	annotations[cluster.LvmdAnnotationsNodeKey] = c.nodeName
	nodeStatus.Node = c.nodeName
	vgNodeConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sutil.TruncateNodeName(cluster.LvmdConfigMapFmt, c.nodeDevices.NodeName),
			Namespace: c.namespace,
			Labels: map[string]string{
				cluster.LvmdConfigMapLabelKey: cluster.LvmdConfigMapLabelValue,
			},
			Annotations: annotations,
		},
		Data: make(map[string]string),
	}

	err = updateVgStatus(vgNodeConfigMap, &nodeStatus, sucClassMap, failClassMap)
	if err != nil {
		return errors.Wrap(err, "create vg status failed")
	}

	err = createLvmdConf(vgNodeConfigMap, sucVgs)
	if err != nil {
		return errors.Wrap(err, "create lvmd conf failed")
	}

	if err := k8sutil.CreateReplaceableConfigmap(c.context.Clientset, vgNodeConfigMap); err != nil {
		vgLogger.Errorf("create configmap failed err:+%v", err)
		return err
	}

	return nil

}

func getVgNameMap(classes []topolvmv1.ClassState) map[string]*topolvmv1.ClassState {

	vgMap := make(map[string]*topolvmv1.ClassState)
	for index, dev := range classes {
		vgMap[dev.VgName] = &classes[index]
	}
	return vgMap

}

func (c *PrePareVg) checkVgIfExpand(class *topolvmv1.DeviceClass, sucClass map[string]*topolvmv1.ClassState) error {

	pv, err := sys.GetPhysicalVolume(c.context.Executor, class.VgName)
	if err != nil {
		vgLogger.Errorf("list pv for vg %s failed err:%+v", class.VgName, err)
		return err
	}

	newPvs := make([]string, 0)

	for _, d := range class.Device {

		if _, ok := pv[d.Name]; !ok {

			if err = sys.CreatePhysicalVolume(c.context.Executor, d.Name); err != nil {
				sucClass[class.VgName].State = ClassExpandWaring
				deviceStatus := topolvmv1.DeviceState{Name: d.Name, State: DeviceStateError, Message: err.Error()}
				sucClass[class.VgName].DeviceStates = append(sucClass[class.VgName].DeviceStates, deviceStatus)
				vgLogger.Errorf("create pv:%s for vg:%s failed err:%+v", d.Name, class.VgName, err)
				continue
			} else {
				newPvs = append(newPvs, d.Name)
			}
		}
	}

	if len(newPvs) > 0 {
		err := sys.ExpandVolumeGroup(c.context.Executor, class.VgName, newPvs)
		if err != nil {
			sucClass[class.VgName].State = ClassExpandError
			return err
		}
	}

	return err
}

func (c *PrePareVg) createVgRetry(availaDisks map[string]*sys.LocalDisk, class *topolvmv1.DeviceClass, sucClass map[string]*topolvmv1.ClassState, failClass map[string]*topolvmv1.ClassState) bool {

	available := true

	for _, disk := range class.Device {
		if _, ok := availaDisks[disk.Name]; !ok {
			message := "disk may has filesystem or is not raw disk please check"
			devStatus := topolvmv1.DeviceState{Name: disk.Name, Message: message}
			if index, ok := checkDeviceStatusIsExisting(failClass[class.VgName].DeviceStates, disk.Name); ok {
				failClass[class.VgName].DeviceStates[index] = devStatus
			} else {
				failClass[class.VgName].DeviceStates = append(failClass[class.VgName].DeviceStates, devStatus)
			}

			vgLogger.Errorf("device:%s is not available", disk.Name)
			available = false
		}
	}
	if available {
		if err := sys.CreateVolumeGroup(c.context.Executor, class.Device, class.VgName); err != nil {
			vgLogger.Errorf("create vg %s retry failed err:%v", class.VgName, err)
			return false

		}
		logger.Infof("create vg %s retry successful", class.VgName)
		sucClass[class.VgName] = &topolvmv1.ClassState{Name: class.ClassName, VgName: class.VgName, State: ClassCreateSuccessful}
		delete(failClass, class.VgName)
		return true
	}
	return false

}

func checkDeviceStatusIsExisting(devs []topolvmv1.DeviceState, disk string) (int, bool) {
	length := len(devs)
	for i := 0; i < length; i++ {
		if devs[i].Name == disk {
			return i, true
		}
	}
	return -1, false
}

func (c *PrePareVg) createVg(availaDisks map[string]*sys.LocalDisk, class *topolvmv1.DeviceClass, sucClass map[string]*topolvmv1.ClassState, failClass map[string]*topolvmv1.ClassState) bool {

	classState := &topolvmv1.ClassState{Name: class.ClassName, VgName: class.VgName}

	available := true

	for _, disk := range class.Device {

		if _, ok := availaDisks[disk.Name]; !ok {
			message := "disk may has filesystem or is not raw disk please check"
			devStatus := topolvmv1.DeviceState{Name: disk.Name, Message: message}
			classState.DeviceStates = append(classState.DeviceStates, devStatus)
			vgLogger.Errorf("device:%s is not available", disk.Name)
			available = false
		}
	}
	if available {
		if err := sys.CreateVolumeGroup(c.context.Executor, class.Device, class.VgName); err != nil {
			classState.State = ClassCreateFail
			failClass[class.VgName] = classState
			vgLogger.Errorf("create vg %s retry failed err:%v", class.VgName, err)
			return false

		} else {
			classState.State = ClassCreateSuccessful
			sucClass[class.VgName] = classState
			logger.Infof("create vg %s retry successful", class.VgName)
			return true
		}
	} else {
		failClass[class.VgName] = classState
		return false
	}

}

func (c *PrePareVg) updateLvmdConf(cm *v1.ConfigMap, newVgs []topolvmv1.DeviceClass) error {

	lvmdConf := cluster.LmvdConf{}
	dataLvmd, ok := cm.Data[cluster.LvmdConfigMapKey]
	if ok {
		err := yaml.Unmarshal([]byte(dataLvmd), &lvmdConf)
		if err != nil {
			return err
		}
	} else {
		return errors.New("lvmd configmap has not config info")
	}

	for _, dev := range c.nodeDevices.DeviceClasses {
		for index, ele := range lvmdConf.DeviceClasses {
			if (ele.Name == dev.ClassName) && (ele.Default != dev.Default) {
				lvmdConf.DeviceClasses[index].Default = dev.Default
			}
		}
	}

	// add new vgs
	for _, dev := range newVgs {
		devClass, err := convertConfig(&dev)
		if err != nil {
			return err
		}
		lvmdConf.DeviceClasses = append(lvmdConf.DeviceClasses, *devClass)
	}

	value, err := yaml.Marshal(lvmdConf)
	if err != nil {
		return err
	}
	cm.Data[cluster.LvmdConfigMapKey] = string(value)
	return nil

}

func updateVgStatus(cm *v1.ConfigMap, state *topolvmv1.NodeStorageState, sucClass map[string]*topolvmv1.ClassState, failClass map[string]*topolvmv1.ClassState) error {

	sucClassSlice := make([]topolvmv1.ClassState, 0)
	for _, dev := range sucClass {
		sucClassSlice = append(sucClassSlice, *dev)
	}

	failClassSlice := make([]topolvmv1.ClassState, 0)
	for _, dev := range failClass {
		failClassSlice = append(failClassSlice, *dev)
	}

	state.FailClasses = failClassSlice
	state.SuccessClasses = sucClassSlice

	value, err := json.Marshal(state)
	if err != nil {
		return err
	}

	cm.Data[cluster.VgStatusConfigMapKey] = string(value)
	return nil
}

func createLvmdConf(cm *v1.ConfigMap, newVgs []topolvmv1.DeviceClass) error {

	if len(newVgs) == 0 {
		return nil
	}

	lvmdConf := cluster.LmvdConf{}
	lvmdConf.SocketName = cluster.LvmdSocketPath
	for _, dev := range newVgs {

		devClass, err := convertConfig(&dev)
		if err != nil {
			return err
		}
		lvmdConf.DeviceClasses = append(lvmdConf.DeviceClasses, *devClass)
	}

	value, err := yaml.Marshal(lvmdConf)
	if err != nil {
		return err
	}
	cm.Data[cluster.LvmdConfigMapKey] = string(value)

	return nil
}

func convertConfig(dev *topolvmv1.DeviceClass) (*cluster.DeviceClass, error) {

	data, err := yaml.Marshal(dev)
	if err != nil {
		vgLogger.Error(err)
		return nil, err
	}

	devClass := &cluster.DeviceClass{}
	err = yaml.Unmarshal(data, devClass)
	if err != nil {
		vgLogger.Error(err)
		return nil, err
	}

	return devClass, nil

}
