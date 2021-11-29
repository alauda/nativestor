/*
Copyright 2018 The Rook Authors. All rights reserved.

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

package discover

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	topolvmv2 "github.com/alauda/topolvm-operator/api/v2"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/alauda/topolvm-operator/pkg/util/sys"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	discoverDaemonUdev = "DISCOVER_DAEMON_UDEV_BLACKLIST"
	resetupLoopPeriod  = time.Second * 5
)

var (
	logger          = capnslog.NewPackageLogger("topolvm/operator", "discover")
	nodeName        string
	namespace       string
	cmName          string
	udevEventPeriod = time.Duration(5) * time.Second
	useLoop         bool
)

// Monitors udev for block device changes, and collapses these events such that
// only one event is emitted per period in order to deal with flapping.
func udevBlockMonitor(c chan string, period time.Duration) {
	defer close(c)
	var udevFilter []string

	// return any add or remove events, but none that match device mapper
	// events. string matching is case-insensitive
	events := make(chan string)

	// get discoverDaemonUdevBlacklist from the environment variable
	// if user doesn't provide any regex; generate the default regex
	// else use the regex provided by user
	discoverUdev := os.Getenv(discoverDaemonUdev)
	if discoverUdev == "" {
		discoverUdev = "(?i)dm-[0-9]+,(?i)rbd[0-9]+,(?i)nbd[0-9]+"
	}
	udevFilter = strings.Split(discoverUdev, ",")
	logger.Infof("using the regular expressions %q", udevFilter)

	go rawUdevBlockMonitor(events,
		[]string{"(?i)add", "(?i)remove"},
		udevFilter)

	for {
		event, ok := <-events
		if !ok {
			return
		}
		timeout := time.NewTimer(period)
		for {
			select {
			case <-timeout.C:
			case _, ok := <-events:
				if !ok {
					return
				}
				continue
			}
			break
		}
		c <- event
	}
}

func Run(context *cluster.Context, probeInterval time.Duration) error {
	if context == nil {
		return fmt.Errorf("nil context")
	}
	logger.Debugf("device discovery interval is %q", probeInterval.String())
	nodeName = os.Getenv(cluster.NodeNameEnv)
	namespace = os.Getenv(cluster.PodNameSpaceEnv)
	if os.Getenv(cluster.UseLoopEnv) == cluster.UseLoop {
		useLoop = true
	} else {
		useLoop = false
	}

	cmName = k8sutil.TruncateNodeName(cluster.LvmdConfigMapFmt, nodeName)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)

	err := updateDeviceCM(context)
	if err != nil {
		logger.Infof("failed to update device configmap: %v", err)
		return err
	}

	if useLoop {
		err := checkLoopDevice(context)
		if err != nil {
			go retryLoopDevice(context)
		}
	}

	udevEvents := make(chan string)
	go udevBlockMonitor(udevEvents, udevEventPeriod)
	for {
		select {
		case <-sigc:
			logger.Infof("shutdown signal received, exiting...")
			return nil
		case <-time.After(probeInterval):
			if err := updateDeviceCM(context); err != nil {
				logger.Errorf("failed to update device configmap during probe interval. %v", err)
			}
			checkDeviceClass(context)
		case _, ok := <-udevEvents:
			if ok {
				logger.Info("trigger probe from udev event")
				if err := updateDeviceCM(context); err != nil {
					logger.Errorf("failed to update device configmap triggered from udev event. %v", err)
				}
			} else {
				logger.Warningf("disabling udev monitoring")
				udevEvents = nil
			}
		}
	}
}

func retryLoopDevice(clusterdContext *cluster.Context) {
	for {
		err := checkLoopDevice(clusterdContext)
		if err == nil {
			return
		}
		logger.Errorf("check loop device failed %v retry", err)
		time.Sleep(resetupLoopPeriod)
	}
}

func checkLoopDevice(clusterdContext *cluster.Context) error {
	ctx := context.TODO()
	cmTemp, err := clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err == nil {
		if loopDevices, ok := cmTemp.Data[cluster.VgStatusConfigMapKey]; ok {

			nodeStatus := topolvmv2.NodeStorageState{}
			err := json.Unmarshal([]byte(loopDevices), &nodeStatus)
			if err != nil {
				logger.Errorf("unmarshal confimap status data failed %+v ", err)
				return err
			}

			failed := false
			for _, ele := range nodeStatus.Loops {
				if ele.Status == cluster.LoopCreateSuccessful {

					err := sys.ReSetupLoop(clusterdContext.Executor, ele.File, ele.DeviceName)
					if err != nil {
						failed = true
						logger.Errorf("losetup device %s file %s failed %v", ele.DeviceName, ele.File, err)
					}
				}
			}

			if failed {
				return errors.New("some loop device resetup failed")
			}
		}

	} else {

		if !kerrors.IsNotFound(err) {
			logger.Infof("failed to get configmap: %v", err)
			return err
		}

		return nil
	}

	return nil
}

func updateDeviceCM(clusterdContext *cluster.Context) error {
	ctx := context.TODO()
	logger.Infof("updating device configmap")
	devices, err := sys.GetAllDevices(clusterdContext)
	if err != nil {
		logger.Errorf("can not list disk err:%s", err)
		return err
	}
	deviceJSON, err := json.Marshal(devices)
	if err != nil {
		logger.Infof("failed to marshal: %v", err)
		return err
	}
	deviceStr := string(deviceJSON)
	cm, err := clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err == nil {
		lastDevice := cm.Data[cluster.LocalDiskCMData]
		logger.Debugf("last devices %s", lastDevice)
		if lastDevice != deviceStr {
			newcm := cm.DeepCopy()
			newcm.Data[cluster.LocalDiskCMData] = deviceStr
			err = k8sutil.PatchConfigMap(clusterdContext.Clientset, newcm.Namespace, cm, newcm)
			if err != nil {
				logger.Errorf("failed to update configmap %s: %v", cmName, err)
				return err
			}
		}
	} else {
		if !kerrors.IsNotFound(err) {
			logger.Infof("failed to get configmap: %v", err)
			return err
		}

		data := make(map[string]string, 1)
		data[cluster.LocalDiskCMData] = deviceStr

		// the map doesn't exist yet, create it now
		annotations := make(map[string]string)
		annotations[cluster.LvmdAnnotationsNodeKey] = nodeName
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: namespace,
				Labels: map[string]string{
					cluster.LvmdConfigMapLabelKey: cluster.LvmdConfigMapLabelValue,
				},
				Annotations: annotations,
			},
			Data: data,
		}

		_, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			logger.Infof("failed to create configmap: %v", err)
			return fmt.Errorf("failed to create local device map %s: %+v", cmName, err)
		}
		return nil
	}
	return nil
}

func checkDeviceClass(clusterdContext *cluster.Context) error {
	logger.Info("check device status")
	ctx := context.TODO()
	cm, err := clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err == nil {
		newcm := cm.DeepCopy()
		status := newcm.Data[cluster.VgStatusConfigMapKey]
		if status == "" {
			return nil
		}

		nodeStatus := &topolvmv2.NodeStorageState{}
		err = json.Unmarshal([]byte(status), nodeStatus)
		if err != nil {
			logger.Errorf("unmarshal node status failed err %v", err)
			return err
		}

		for index1, ele := range nodeStatus.SuccessClasses {
			pvs, err := sys.GetPhysicalVolume(clusterdContext.Executor, ele.VgName)
			if err != nil {
				logger.Errorf("list pvs of vg %s failed err %v", ele.VgName, err)
				return err
			}
			for index2, d := range nodeStatus.SuccessClasses[index1].DeviceStates {
				if _, ok := pvs[d.Name]; ok {
					continue
				} else {
					nodeStatus.SuccessClasses[index1].DeviceStates[index2].State = topolvmv2.DeviceStateOffline
				}
			}
		}

		res, err := json.Marshal(nodeStatus)
		if err != nil {
			logger.Errorf("marshal node status failed %v", err)
			return err
		}
		newcm.Data[cluster.VgStatusConfigMapKey] = string(res)
		err = k8sutil.PatchConfigMap(clusterdContext.Clientset, newcm.Namespace, cm, newcm)
		if err != nil {
			logger.Errorf("failed to update configmap %s: %v", cmName, err)
			return err
		}
	}

	return nil
}

// Scans `udevadm monitor` output for block sub-system events. Each line of
// output matching a set of substrings is sent to the provided channel. An event
// is returned if it passes any matches tests, and passes all exclusion tests.
func rawUdevBlockMonitor(c chan string, matches, exclusions []string) {
	defer close(c)

	// stdbuf -oL performs line buffered output
	cmd := exec.Command("stdbuf", "-oL", "udevadm", "monitor", "-u", "-k", "-s", "block")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Warningf("Cannot open udevadm stdout: %v", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		logger.Warningf("Cannot start udevadm monitoring: %v", err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		logger.Debugf("udevadm monitor: %s", text)
		match, err := matchUdevEvent(text, matches, exclusions)
		if err != nil {
			logger.Warningf("udevadm filtering failed: %v", err)
			return
		}
		if match {
			c <- text
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Warningf("udevadm monitor scanner error: %v", err)
	}

	logger.Info("udevadm monitor finished")
}

func matchUdevEvent(text string, matches, exclusions []string) (bool, error) {
	for _, match := range matches {
		matched, err := regexp.MatchString(match, text)
		if err != nil {
			return false, fmt.Errorf("failed to search string: %v", err)
		}
		if matched {
			hasExclusion := false
			for _, exclusion := range exclusions {
				matched, err = regexp.MatchString(exclusion, text)
				if err != nil {
					return false, fmt.Errorf("failed to search string: %v", err)
				}
				if matched {
					hasExclusion = true
					break
				}
			}
			if !hasExclusion {
				logger.Infof("udevadm monitor: matched event: %s", text)
				return true, nil
			}
		}
	}
	return false, nil
}
