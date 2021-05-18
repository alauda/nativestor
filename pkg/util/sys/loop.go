package sys

import (
	"errors"
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/util/exec"
	"strconv"
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

	res := parseLines(out)
	if len(res) != 1 {
		return "", errors.New("get loop name failed for file" + filename)
	}
	return res[0]["name"], nil
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
