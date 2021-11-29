/*
Copyright 2016 The Rook Authors. All rights reserved.

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

package sys

import (
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/util/exec"
	"github.com/pkg/errors"
	"path"
	"strconv"
)

const (
	diskPrefix = "/dev/"
)

func GetAllDevices(dcontext *cluster.Context) ([]*LocalDiskAppendInfo, error) {

	res := make([]*LocalDiskAppendInfo, 0)

	var disks []*LocalDisk

	var err error

	if disks, err = DiscoverDevices(dcontext.Executor); err != nil {
		return nil, err
	}

	for _, device := range disks {
		// Ignore device with filesystem signature since c-v inventory
		// cannot detect that correctly
		if device.Size < uint64(2*(1<<30)) {
			logger.Infof("skipping device %q because it size less than 2G", device.Name)
			res = append(res, &LocalDiskAppendInfo{
				LocalDisk: *device,
				Available: false,
				Message:   "size less than 2G",
			})
			continue
		}

		if device.Filesystem != "" {
			res = append(res, &LocalDiskAppendInfo{
				LocalDisk: *device,
				Available: false,
				Message:   fmt.Sprintf("containe a filesystem %s", device.Filesystem),
			})
			logger.Infof("skipping device %q because it contains a filesystem %q", device.Name, device.Filesystem)
			continue
		}
		if device.MountPoint != "" {
			res = append(res, &LocalDiskAppendInfo{
				LocalDisk: *device,
				Available: false,
				Message:   fmt.Sprintf("has a mount point %s", device.MountPoint),
			})
			logger.Infof("skipping device %q because it has a mount point %q", device.Name, device.MountPoint)
			continue
		}

		logger.Debugf("device:%s is available", device.Name)
		res = append(res, &LocalDiskAppendInfo{
			LocalDisk: *device,
			Available: true,
		})

	}

	return res, nil

}

func GetAvailableDevices(dcontext *cluster.Context) (map[string]*LocalDisk, error) {

	availableDevices := make(map[string]*LocalDisk)

	var disks []*LocalDisk

	var err error

	if disks, err = DiscoverDevices(dcontext.Executor); err != nil {
		return nil, err
	}

	for _, device := range disks {
		// Ignore device with filesystem signature since c-v inventory
		// cannot detect that correctly
		if device.Size < uint64(2*(1<<30)) {
			logger.Infof("skipping device %q because it size less than 2G", device.Name)
			continue
		}

		if device.Filesystem != "" {
			logger.Infof("skipping device %q because it contains a filesystem %q", device.Name, device.Filesystem)
			continue
		}

		if device.MountPoint != "" {
			logger.Infof("skipping device %q because it has a mount point %q", device.Name, device.MountPoint)
			continue
		}

		logger.Debugf("device:%s is available", device.Name)
		availableDevices[diskPrefix+device.Name] = device

	}

	return availableDevices, nil

}

// DiscoverDevices returns all the details of devices available on the local node
func DiscoverDevices(executor exec.Executor) ([]*LocalDisk, error) {
	var disks []*LocalDisk
	devices, err := ListDevices(executor)
	if err != nil {
		return nil, err
	}

	for _, d := range devices {

		// Populate device information coming from lsblk
		disk, err := populateDeviceInfo(d, executor)
		if err != nil {
			logger.Warningf("skipping device %q. %v", d, err)
			continue
		}

		// Populate udev information coming from udev
		disk, err = populateDeviceUdevInfo(d, executor, disk)
		if err != nil {
			// go on without udev info
			// not ideal for our filesystem check later but we can't really fail either...
			logger.Warningf("failed to get udev info for device %q. %v", d, err)
		}

		// Test if device has child, if so we skip it and only consider the partitions
		// which will come in later iterations of the loop
		// We only test if the type is 'disk', this is a property reported by lsblk
		// and means it's a parent block device
		if disk.Type == DiskType {
			deviceChild, err := ListDevicesChild(executor, d)
			if err != nil {
				logger.Warningf("failed to detect child devices for device %q, assuming they are none. %v", d, err)
			}
			// lsblk will output at least 2 lines if they are partitions, one for the parent
			// and N for the child
			if len(deviceChild) > 1 {
				logger.Infof("skipping device %q because it has child, considering the child instead.", d)
				continue
			}
		}

		disks = append(disks, disk)
	}
	logger.Debugf("discovered disks are %v", disks)

	return disks, nil
}

// PopulateDeviceInfo returns the information of the specified block device
func populateDeviceInfo(d string, executor exec.Executor) (*LocalDisk, error) {
	diskProps, err := GetDeviceProperties(d, executor)
	if err != nil {
		return nil, err
	}

	diskType, ok := diskProps["TYPE"]
	if !ok {
		return nil, errors.New("diskType is empty")
	}
	if !supportedDeviceType(diskType) {
		return nil, fmt.Errorf("unsupported diskType %+s", diskType)
	}

	// get the UUID for disks
	var diskUUID string
	if diskType != PartType {
		diskUUID, err = GetDiskUUID(d, executor)
		if err != nil {
			return nil, err
		}
	}

	disk := &LocalDisk{Name: d, UUID: diskUUID}

	if val, ok := diskProps["TYPE"]; ok {
		disk.Type = val
	}
	if val, ok := diskProps["SIZE"]; ok {
		if size, err := strconv.ParseUint(val, 10, 64); err == nil {
			disk.Size = size
		}
	}
	if val, ok := diskProps["ROTA"]; ok {
		if rotates, err := strconv.ParseBool(val); err == nil {
			disk.Rotational = rotates
		}
	}
	if val, ok := diskProps["RO"]; ok {
		if ro, err := strconv.ParseBool(val); err == nil {
			disk.Readonly = ro
		}
	}
	if val, ok := diskProps["PKNAME"]; ok {
		if val != "" {
			disk.Parent = path.Base(val)
		}
	}
	if val, ok := diskProps["NAME"]; ok {
		disk.RealPath = val
	}
	if val, ok := diskProps["KNAME"]; ok {
		disk.KernelName = path.Base(val)
	}
	if val, ok := diskProps["MOUNTPOINT"]; ok {
		disk.MountPoint = path.Base(val)
	}

	return disk, nil
}

// PopulateDeviceUdevInfo fills the udev info into the block device information
func populateDeviceUdevInfo(d string, executor exec.Executor, disk *LocalDisk) (*LocalDisk, error) {
	udevInfo, err := GetUdevInfo(d, executor)
	if err != nil {
		return disk, err
	}
	// parse udev info output
	if val, ok := udevInfo["DEVLINKS"]; ok {
		disk.DevLinks = val
	}
	if val, ok := udevInfo["ID_FS_TYPE"]; ok {
		disk.Filesystem = val
	}
	if val, ok := udevInfo["ID_SERIAL"]; ok {
		disk.Serial = val
	}

	if val, ok := udevInfo["ID_VENDOR"]; ok {
		disk.Vendor = val
	}

	if val, ok := udevInfo["ID_MODEL"]; ok {
		disk.Model = val
	}

	if val, ok := udevInfo["ID_WWN_WITH_EXTENSION"]; ok {
		disk.WWNVendorExtension = val
	}

	if val, ok := udevInfo["ID_WWN"]; ok {
		disk.WWN = val
	}

	return disk, nil
}

func supportedDeviceType(device string) bool {
	return device == DiskType ||
		device == SSDType ||
		device == CryptType ||
		device == MultiPath ||
		device == PartType ||
		device == LinearType ||
		device == LoopType
}
