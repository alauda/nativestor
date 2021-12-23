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

	"github.com/alauda/nativestor/pkg/util/exec"
)

const (
	losetup  = "losetup"
	truncate = "truncate"
)

func CreateLoop(executor exec.Executor, filename string, size uint64) (string, error) {

	s := strconv.Itoa(int(size)) + "G"
	err := wrapExecCommand(executor, truncate, fmt.Sprintf("--size=%s", s), filename)
	if err != nil {
		return "", err
	}
	err = wrapExecCommand(executor, losetup, "-f", filename)
	if err != nil {
		wrapExecCommand(executor, "rm", "-f", filename)
		return "", err
	}
	out, err := wrapExecCommandWithOutput(executor, losetup, "-j", filename, "-O", "name")
	if err != nil {
		return "", err
	}

	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		return "", errors.New("get loop name failed for file" + filename)
	}
	device := strings.TrimSpace(lines[1])
	return device, nil
}

func GetLoopBackFile(executor exec.Executor, loop string) (string, error) {
	logger.Debugf("get loop %s back file", loop)
	out, err := wrapExecCommandWithOutput(executor, losetup, loop, "-O", "back-file")
	if err != nil {
		return "", err
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		return "", errors.New("get loop %s back file name failed " + loop)
	}
	file := strings.TrimSpace(lines[1])
	return file, nil
}

func ReSetupLoop(executor exec.Executor, filename string, loop string) error {

	devices, err := DiscoverDevices(executor, false)
	if err != nil {
		return err
	}

	for _, ele := range devices {
		if "/dev/"+ele.Name == loop {
			return nil
		}
	}
	return wrapExecCommand(executor, losetup, loop, filename)
}
