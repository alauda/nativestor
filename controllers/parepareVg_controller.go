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
	"github.com/alauda/topolvm-operator/pkg/util/exec"
	"github.com/alauda/topolvm-operator/pkg/util/sys"
	"github.com/coreos/pkg/capnslog"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

const (
	ClassExpandWaring     = "expand warning"
	ClassExpandError      = "expand error"
	ClassCreateSuccessful = "create successful"
	ClassCreateFail       = "create failed"
	DeviceStateError      = "error"
	LoopCreateFailed      = "failed"
	Loop                  = "loop"
)

var vgLogger = capnslog.NewPackageLogger("topolvm/operator", "prepare-vg-controller")

type PrePareVg struct {
	nodeName    string
	namespace   string
	context     *cluster.Context
	nodeDevices topolvmv1.NodeDevices
	loopsState  []topolvmv1.LoopState
	loopMap     map[string]topolvmv1.LoopState
}

func NewPrepareVgController(nodeName string, nameSpace string, context *cluster.Context) *PrePareVg {
	return &PrePareVg{
		nodeName:  nodeName,
		namespace: nameSpace,
		context:   context,
		loopMap:   make(map[string]topolvmv1.LoopState),
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

	logger.Info("start provision vg")
	err = c.provision(topolvmCluster)
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

func (c *PrePareVg) provision(topolvmCluster *topolvmv1.TopolvmCluster) error {

	// check existing vg and check if need expand

	// get node class info

	for _, dev := range topolvmCluster.Status.NodeStorageStatus {
		if dev.Node == c.nodeName {
			c.loopsState = dev.Loops
		}
	}

	if topolvmCluster.Spec.Storage.UseLoop && topolvmCluster.Spec.Storage.DeviceClasses != nil {

		for _, ele := range c.nodeDevices.DeviceClasses {
			checkLoopDevice(c.context.Executor, ele.Device, &c.loopsState, c.loopMap)
		}

	}

	disks, err := sys.GetAvailableDevices(c.context)
	if err != nil {
		vgLogger.Errorf("can not list disk err:%s", err)
		return err
	}
	if topolvmCluster.Spec.UseAllDevices {
		deviceClass := topolvmv1.DeviceClass{ClassName: topolvmCluster.Spec.Storage.ClassName, VgName: topolvmCluster.Spec.Storage.VolumeGroupName, Default: true}
		for key, dev := range disks {
			deviceClass.Device = append(deviceClass.Device, topolvmv1.Disk{Name: key, Type: dev.Type})
		}
		deviceClasses := []topolvmv1.DeviceClass{deviceClass}
		c.nodeDevices = topolvmv1.NodeDevices{NodeName: c.nodeName, DeviceClasses: deviceClasses}

		if topolvmCluster.Spec.Storage.UseLoop {
			checkLoopDevice(c.context.Executor, c.nodeDevices.DeviceClasses[0].Device, &c.loopsState, c.loopMap)
			disks, err = sys.GetAvailableDevices(c.context)
			if err != nil {
				vgLogger.Errorf("can not list disk err:%s", err)
				return err
			}
		}

	} else if topolvmCluster.Spec.Devices != nil {

		if topolvmCluster.Spec.Storage.UseLoop {
			checkLoopDevice(c.context.Executor, topolvmCluster.Spec.Devices, &c.loopsState, c.loopMap)
			disks, err = sys.GetAvailableDevices(c.context)
			if err != nil {
				vgLogger.Errorf("can not list disk err:%s", err)
				return err
			}
		}
		deviceClass := topolvmv1.DeviceClass{ClassName: topolvmCluster.Spec.Storage.ClassName, VgName: topolvmCluster.Spec.Storage.VolumeGroupName, Default: true, Device: topolvmCluster.Spec.Storage.Devices}
		deviceClasses := []topolvmv1.DeviceClass{deviceClass}
		c.nodeDevices = topolvmv1.NodeDevices{NodeName: c.nodeName, DeviceClasses: deviceClasses}

	} else if topolvmCluster.Spec.DeviceClasses != nil {
		for _, dev := range topolvmCluster.Spec.DeviceClasses {
			if dev.NodeName == c.nodeName {
				c.nodeDevices = dev
			}
		}
	}

	// get current class status
	ctx := context.TODO()
	cmname := k8sutil.TruncateNodeName(cluster.LvmdConfigMapFmt, c.nodeDevices.NodeName)
	cm, err := c.context.Clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, cmname, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		vgLogger.Warningf("failed to detect configmap %s. %+v", cmname, err)
		return err
	} else if err == nil {

		logger.Info("cm is existing check if need update")
		if vgStatus, ok := cm.Data[cluster.VgStatusConfigMapKey]; ok {
			logger.Debug("start provision with node status")
			if err := c.provisionWithNodeStatus(cm, vgStatus, disks); err != nil {
				vgLogger.Errorf("provisionWithNodeStatus failed err %v", err)
				return err
			}
		} else {
			logger.Debug("start provision with cm")
			if err := c.provisionFirst(disks, cm); err != nil {
				vgLogger.Errorf("provisionFirst failed with cm err %v", err)
				return err
			}
		}
		return nil
	}

	logger.Info("provision vg and create configmap")

	//todo should distinguish the created vg between cluster and other user

	return c.provisionFirst(disks, nil)
}

func getVgNameMap(classes []topolvmv1.ClassState) map[string]*topolvmv1.ClassState {

	vgMap := make(map[string]*topolvmv1.ClassState)
	for index, dev := range classes {
		vgMap[dev.VgName] = &classes[index]
	}
	return vgMap

}

func (c *PrePareVg) provisionFirst(disks map[string]*sys.LocalDisk, cm *v1.ConfigMap) error {

	nodeStatus := topolvmv1.NodeStorageState{}
	nodeStatus.Node = c.nodeName
	sucVgs := make([]topolvmv1.DeviceClass, 0)
	// record the status of each class
	sucClassMap := make(map[string]*topolvmv1.ClassState)
	failClassMap := make(map[string]*topolvmv1.ClassState)

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

	cmNew := &v1.ConfigMap{}
	if cm == nil {

		annotations := make(map[string]string)
		annotations[cluster.LvmdAnnotationsNodeKey] = c.nodeName
		cmNew = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sutil.TruncateNodeName(cluster.LvmdConfigMapFmt, c.nodeDevices.NodeName),
				Namespace: c.namespace,
				Labels: map[string]string{
					cluster.LvmdConfigMapLabelKey: cluster.LvmdConfigMapLabelValue,
					cluster.NodeAttr:              c.nodeName,
				},
				Annotations: annotations,
			},
			Data: make(map[string]string),
		}
	} else {

		cmNew = cm.DeepCopy()
	}

	// create cm for node to notify operator to create or update node deployment and update TopolvmCluster status

	err = updateVgStatus(cmNew, &nodeStatus, sucClassMap, failClassMap, c.loopsState)
	if err != nil {
		return errors.Wrap(err, "create vg status failed")
	}

	err = createLvmdConf(cmNew, sucVgs)
	if err != nil {
		return errors.Wrap(err, "create lvmd conf failed")
	}

	if cm == nil {
		if err := k8sutil.CreateOrPatchConfigmap(c.context.Clientset, cmNew); err != nil {
			vgLogger.Errorf("create configmap failed err:+%v", err)
			return err
		}
	} else {
		err = k8sutil.PatchConfigMap(c.context.Clientset, c.namespace, cm, cmNew)
		if err != nil {
			return errors.Wrap(err, "patch configmap failed")
		}
	}
	return nil
}

func (c *PrePareVg) provisionWithNodeStatus(cm *v1.ConfigMap, vgStatus string, disks map[string]*sys.LocalDisk) error {

	nodeStatus := topolvmv1.NodeStorageState{}
	sucVgs := make([]topolvmv1.DeviceClass, 0)
	err := json.Unmarshal([]byte(vgStatus), &nodeStatus)
	if err != nil {
		vgLogger.Errorf("unmarshal confimap status data failed %+v ", err)
		return err
	}

	newCm := cm.DeepCopy()

	sucClassMap := getVgNameMap(nodeStatus.SuccessClasses)
	failClassMap := getVgNameMap(nodeStatus.FailClasses)

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

	err = c.updateLvmdConf(newCm, sucVgs)
	if err != nil {
		return errors.Wrap(err, "update lvmd conf failed")
	}

	err = updateVgStatus(newCm, &nodeStatus, sucClassMap, failClassMap, c.loopsState)
	if err != nil {
		return errors.Wrap(err, "update vg status failed")
	}

	err = k8sutil.PatchConfigMap(c.context.Clientset, c.namespace, cm, newCm)
	if err != nil {
		return errors.Wrap(err, "patch configmap failed")
	}
	return nil

}

func (c *PrePareVg) checkVgIfExpand(class *topolvmv1.DeviceClass, sucClass map[string]*topolvmv1.ClassState) error {

	pv, err := sys.GetPhysicalVolume(c.context.Executor, class.VgName)
	if err != nil {
		vgLogger.Errorf("list pv for vg %s failed err:%+v", class.VgName, err)
		return err
	}

	newPvs := make([]string, 0)

	for _, d := range class.Device {

		if d.Type == Loop && d.Auto {
			name := c.getDeviceName(d.Name)
			if name == "" {
				continue
			}
			d.Name = name
		}
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

	for index, disk := range class.Device {

		if disk.Type == Loop && disk.Auto {
			name := c.getDeviceName(disk.Name)
			if name == "" {
				continue
			}
			class.Device[index].Name = name
			disk.Name = name
		}
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

	for index, disk := range class.Device {

		if disk.Type == Loop && disk.Auto {
			name := c.getDeviceName(disk.Name)
			if name == "" {
				continue
			}
			disk.Name = name
			class.Device[index].Name = name
		}

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
		return createLvmdConf(cm, newVgs)
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

func (c *PrePareVg) getDeviceName(name string) string {

	if loop, ok := c.loopMap[name]; ok {
		return loop.DeviceName
	} else {
		return ""
	}
}

func updateVgStatus(cm *v1.ConfigMap, state *topolvmv1.NodeStorageState, sucClass map[string]*topolvmv1.ClassState, failClass map[string]*topolvmv1.ClassState, loopsState []topolvmv1.LoopState) error {

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
	state.Loops = loopsState

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

func checkLoopDevice(executor exec.Executor, disks []topolvmv1.Disk, loops *[]topolvmv1.LoopState, loopMap map[string]topolvmv1.LoopState) {
	vgLogger.Debug("check loop device")
	for _, ele := range disks {
		if ele.Type == Loop {
			created := false
			failedLoopIndex := 0
			retry := false
			for index, loop := range *loops {
				if loop.Name == ele.Name {
					if loop.Status == cluster.LoopCreateSuccessful {
						loopMap[loop.Name] = loop
						created = true
					} else {
						failedLoopIndex = index
						retry = true
					}
					break
				}
			}

			if ele.Auto {
				//no created before
				if !created {
					file := uuid.New().String()
					loopName, err := sys.CreateLoop(executor, ele.Path+"/"+file, ele.Size)
					s := topolvmv1.LoopState{Name: ele.Name, File: getAbsoluteFileName(ele.Path, file)}
					if err != nil {
						vgLogger.Errorf("create loop %s failed %v", ele.Name, err)
						s.Status = LoopCreateFailed
						s.Message = err.Error()
					}
					s.Status = cluster.LoopCreateSuccessful
					s.DeviceName = loopName
					if retry {
						(*loops)[failedLoopIndex] = s

					} else {
						*loops = append(*loops, s)
					}
					loopMap[ele.Name] = s
				}

			} else {
				if !created {
					vgLogger.Debugf("get loop %s back file", ele.Name)
					s := topolvmv1.LoopState{Name: ele.Name, Status: cluster.LoopCreateSuccessful}
					file, err := sys.GetLoopBackFile(executor, ele.Name)
					if err != nil {
						vgLogger.Errorf("get loop %s back file failed %v", ele.Name, err)
						s.Message = err.Error()
					}
					vgLogger.Debugf("loop %s backfile %s", ele.Name, file)
					s.File = file
					*loops = append(*loops, s)
					vgLogger.Debug("get loop back file done")
				}
			}
		}
	}
}

func getAbsoluteFileName(path, file string) string {
	if strings.HasSuffix(path, "/") {
		return path + file
	}
	return path + "/" + file
}
