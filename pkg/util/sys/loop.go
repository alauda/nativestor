package sys

import (
	"errors"
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/util/exec"
	"strconv"
	"strings"
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

	devices, err := DiscoverDevices(executor)
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
