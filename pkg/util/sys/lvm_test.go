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
	osexec "os/exec"
	"testing"

	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	"github.com/alauda/nativestor/pkg/util/exec"
	"github.com/stretchr/testify/assert"
)

func TestCreatePhysicalVolume(t *testing.T) {

	file := "test.img"
	loop, err := MakeLoopbackDevice(file)
	if err != nil {
		t.Fatal(err)
	}

	defer CleanLoopbackPv([]string{loop}, []string{loop}, []string{file})
	executor := &exec.CommandExecutor{}
	err = CreatePhysicalVolume(executor, loop)
	if err != nil {
		t.Fatal(err)
	}

	field := "pv_name"
	info, err := parseOutput(executor, "pvs", field, loop)
	if err != nil {
		t.Fatal(err)
	}

	if len(info) != 1 {
		t.Errorf("num pv must be 0: %d", len(info))
	}

	pvName := info[0][field]
	assert.Equal(t, loop, pvName)

}

func TestCreateVolumeGroup(t *testing.T) {
	file := "test.img"
	loop, err := MakeLoopbackDevice(file)
	if err != nil {
		t.Fatal(err)
	}
	vgName := "hello"
	defer CleanLoopbackVG(vgName, []string{loop}, []string{loop}, []string{file})

	err = osexec.Command("pvcreate", loop).Run()
	if err != nil {
		t.Fatal(err)
	}
	executor := &exec.CommandExecutor{}
	err = CreateVolumeGroup(executor, []topolvmv2.Disk{{Name: loop}}, vgName)
	if err != nil {
		t.Fatal(err)
	}

	info, err := GetVolumeGroups(executor)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := info[vgName]; !ok {
		t.Fatalf("must have vg name %s", vgName)
	}

}

func TestExpandVolumeGroup(t *testing.T) {

	file1 := "test1.img"
	loop1, err := MakeLoopbackDevice(file1)
	if err != nil {
		t.Fatal(err)
	}
	file2 := "test2.img"
	loop2, err := MakeLoopbackDevice(file2)
	if err != nil {
		t.Fatal(err)
	}
	vgName := "hello"
	defer CleanLoopbackVG(vgName, []string{loop1, loop2}, []string{loop1, loop2}, []string{file1, file2})

	executor := &exec.CommandExecutor{}
	err = CreateVolumeGroup(executor, []topolvmv2.Disk{{Name: loop1}}, vgName)
	if err != nil {
		t.Fatal(err)
	}

	info, err := GetPhysicalVolume(executor, vgName)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := info[loop1]; !ok {
		t.Fatalf("must have pv %s", loop1)
	}

	err = ExpandVolumeGroup(executor, vgName, []string{loop2})
	if err != nil {
		t.Fatal(err)
	}

	info, err = GetPhysicalVolume(executor, vgName)
	if err != nil {
		t.Fatal(err)
	}
	_, ok1 := info[loop1]
	_, ok2 := info[loop2]

	if !ok1 || !ok2 {
		t.Fatalf("must have pv %s and %s", loop1, loop2)
	}
}

func TestGetPhysicalVolume(t *testing.T) {
	file := "test.img"
	loop, err := MakeLoopbackDevice(file)
	if err != nil {
		t.Fatal(err)
	}
	vgName := "hello"
	defer CleanLoopbackVG(vgName, []string{loop}, []string{loop}, []string{file})

	err = osexec.Command("pvcreate", loop).Run()
	if err != nil {
		t.Fatal(err)
	}
	executor := &exec.CommandExecutor{}
	err = CreateVolumeGroup(executor, []topolvmv2.Disk{{Name: loop}}, vgName)
	if err != nil {
		t.Fatal(err)
	}

	info, err := GetPhysicalVolume(executor, vgName)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := info[loop]; !ok {
		t.Fatalf("must have pv %s", loop)
	}
}
