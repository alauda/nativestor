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
	"bytes"
	"os"
	"os/exec"
	"strings"
)

func MakeLoopbackDevice(name string) (string, error) {
	command := exec.Command("losetup", "-f")
	command.Stderr = os.Stderr
	loop := bytes.Buffer{}
	command.Stdout = &loop
	err := command.Run()
	if err != nil {
		return "", err
	}
	loopDev := strings.TrimRight(loop.String(), "\n")
	out, err := exec.Command("truncate", "--size=4G", name).CombinedOutput()
	if err != nil {
		logger.Error("failed to truncate", map[string]interface{}{
			"output": string(out),
		})
		return "", err
	}
	out, err = exec.Command("losetup", loopDev, name).CombinedOutput()
	if err != nil {
		logger.Error("failed to losetup", map[string]interface{}{
			"output": string(out),
		})
		return "", err
	}
	return loopDev, nil
}

// MakeLoopbackVG creates a VG made from loopback device by losetup
func MakeLoopbackVG(name string, devices ...string) error {
	args := append([]string{name}, devices...)
	out, err := exec.Command("vgcreate", args...).CombinedOutput()
	if err != nil {
		logger.Error("failed to vgcreate", map[string]interface{}{
			"output": string(out),
		})
		return err
	}
	return nil
}

func CleanLoopbackPv(pvs []string, loops []string, files []string) error {

	for _, pv := range pvs {
		err := exec.Command("pvremove", "-f", pv).Run()
		if err != nil {
			return err
		}
	}

	for _, loop := range loops {
		err := exec.Command("losetup", "-d", loop).Run()
		if err != nil {
			return err
		}
	}

	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil

}

// CleanLoopbackVG deletes a VG made by MakeLoopbackVG
func CleanLoopbackVG(name string, pvs []string, loops []string, files []string) error {
	err := exec.Command("vgremove", "-f", name).Run()
	if err != nil {
		logger.Errorf("failed to remove vg %s", name)
		return err
	}

	for _, pv := range pvs {
		err := exec.Command("pvremove", "-f", pv).Run()
		if err != nil {
			logger.Errorf("failed to remove pv %s", pv)
			return err
		}
	}

	for _, loop := range loops {
		err = exec.Command("losetup", "-d", loop).Run()
		if err != nil {
			logger.Errorf("failed to delete loop %s", loop)
			return err
		}
	}

	for _, file := range files {
		err = os.Remove(file)
		if err != nil {
			logger.Errorf("failed to delete file %s", file)
			return err
		}
	}
	return nil
}
