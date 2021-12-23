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

package sys

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	"github.com/alauda/nativestor/pkg/util/exec"
	perrors "github.com/pkg/errors"
)

const (
	nsenter = "nsenter"
	lvm     = "lvm"
)

type VolumeGroup struct {
	Name    string
	Size    uint64
	PvCount uint
	Pvs     []PhysicalVolume
}

type PhysicalVolume struct {
	Name            string
	VolumeGroupName string
}

type LVInfo map[string]string

func wrapExecCommandWithOutput(executor exec.Executor, cmd string, args ...string) (string, error) {
	args = append([]string{"-u", "-i", "-n", "-m", "-p", "-t", "1", cmd}, args...)
	return executor.ExecuteCommandWithOutput(nsenter, args...)
}

func wrapExecCommand(executor exec.Executor, cmd string, args ...string) error {

	args = append([]string{"-u", "-i", "-n", "-m", "-p", "-t", "1", cmd}, args...)
	return executor.ExecuteCommand(nsenter, args...)
}

func GetVolumeGroups(executor exec.Executor) (LVInfo, error) {

	lvInfo := LVInfo{}
	field := "vg_name"
	infoList, err := parseOutput(executor, "vgs", field)
	if err != nil {
		return nil, perrors.Wrapf(err, "parse out failed cmd:%s %s", "vgs", field)
	}
	for _, info := range infoList {
		vgName := info[field]
		lvInfo[vgName] = vgName
	}

	return lvInfo, nil

}

func GetPhysicalVolume(executor exec.Executor, vgname string) (LVInfo, error) {

	lvInfo := LVInfo{}
	field := "pv_name"
	infoList, err := parseOutput(executor, "vgs", field, vgname)
	if err != nil {
		return nil, perrors.Wrapf(err, "parse out failed cmd:%s %s %s", "vgs", field, vgname)
	}
	for _, info := range infoList {
		pvName := info[field]
		lvInfo[pvName] = pvName
	}
	return lvInfo, nil

}

func CheckPVHasLogicalVolume(executor exec.Executor, pvname string) (bool, error) {

	field := "+lv_name"
	infoList, err := parseOutput(executor, "pvs", field, pvname, "--segments")
	if err != nil {
		return false, perrors.Wrapf(err, "parse out failed cmd:%s %s %s", "pvs", field, pvname)
	}
	for _, info := range infoList {
		if info["lv_name"] != "" {
			return true, nil
		}

	}
	return false, nil
}

func CheckVgHasLogicalVolume(executor exec.Executor, vgname string) (bool, error) {

	field := "lv_name"
	infoList, err := parseOutput(executor, "vgs", field, vgname)
	if err != nil {
		return false, perrors.Wrapf(err, "parse out failed cmd:%s %s %s", "vgs", field, vgname)
	}
	for _, info := range infoList {
		if info[field] != "" {
			return true, nil
		}

	}
	return false, nil
}

func RemoveVolumeGroup(executor exec.Executor, vgName string) error {

	ok, err := CheckVgHasLogicalVolume(executor, vgName)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("vg %s has some lv can not remove", vgName)
	}

	infoList, err := GetPhysicalVolume(executor, vgName)
	if err != nil {
		return err
	}
	args := []string{"vgremove", vgName}
	err = wrapExecCommand(executor, lvm, args...)
	if err != nil {
		return err
	}

	for key := range infoList {
		args := []string{"pvremove", key}
		err = wrapExecCommand(executor, lvm, args...)
		if err != nil {
			return err
		}
	}
	return nil

}

func ShrinkVolumeGroup(executor exec.Executor, vgName string, pvs []string) error {

	for _, dev := range pvs {
		ok, err := CheckPVHasLogicalVolume(executor, dev)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("pv %s has some lv can not remove", dev)
		}
	}
	args := []string{"vgreduce", vgName}

	args = append(args, pvs...)

	err := wrapExecCommand(executor, lvm, args...)
	if err != nil {
		return err
	}

	args = []string{"pvremove"}
	args = append(args, pvs...)
	err = wrapExecCommand(executor, lvm, args...)
	if err != nil {
		return err
	}
	return nil

}

func ExpandVolumeGroup(executor exec.Executor, vgName string, pvs []string) error {

	args := []string{"vgextend", vgName}

	args = append(args, pvs...)

	return wrapExecCommand(executor, lvm, args...)

}

func CreatePhysicalVolume(executor exec.Executor, diskName string) error {

	return wrapExecCommand(executor, lvm, "pvcreate", diskName)

}

func GetVolumeGroupSize(executor exec.Executor, vgname string) (uint64, error) {

	infoList, err := parseOutput(executor, "vgs", "vg_size", vgname)
	if err != nil {
		return 0, err
	}
	if len(infoList) != 1 {
		return 0, errors.New("volume group not found: " + vgname)
	}

	info := infoList[0]
	vgSize, err := strconv.ParseUint(info["vg_size"], 10, 64)
	if err != nil {
		return 0, err
	}
	return vgSize, nil
}

func CreateVolumeGroup(executor exec.Executor, disks []topolvmv2.Disk, volumeGroupName string) error {

	diskList := make([]string, 0)
	for _, dev := range disks {
		err := wrapExecCommand(executor, lvm, "pvcreate", dev.Name)
		if err != nil {
			return err
		}
		diskList = append(diskList, dev.Name)
	}

	cmd := []string{"vgcreate", volumeGroupName}
	cmd = append(cmd, diskList...)

	err := wrapExecCommand(executor, lvm, cmd...)

	return err
}

// parseOutput calls lvm family and parses output from it.
//
// cmd is a command name of lvm family.
// fields are comma separated field names.
// args is optional arguments for lvm command.
func parseOutput(executor exec.Executor, cmd string, fields string, args ...string) ([]LVInfo, error) {
	arg := []string{
		cmd, "-o", fields,
		"--noheadings", "--separator= ",
		"--units=b", "--nosuffix",
		"--unbuffered", "--nameprefixes",
	}
	arg = append(arg, args...)
	out, err := wrapExecCommandWithOutput(executor, lvm, arg...)
	if err != nil {
		return nil, err
	}

	return parseLines(out), nil
}

func parseLines(output string) []LVInfo {
	var ret []LVInfo
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		info := parseOneLine(line)
		ret = append(ret, info)
	}
	return ret
}

func parseOneLine(line string) LVInfo {
	ret := LVInfo{}
	line = strings.TrimSpace(line)
	for _, token := range strings.Split(line, " ") {
		if len(token) == 0 {
			continue
		}
		// assume token is "k=v"
		kv := strings.Split(token, "=")
		k, v := kv[0], kv[1]
		// k[5:] removes "LVM2_" prefix.
		k = strings.ToLower(k[5:])
		// assume v is 'some-value'
		v = strings.Trim(v, "'")
		ret[k] = v
	}
	return ret
}
