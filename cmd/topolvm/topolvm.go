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

package topolvm

import (
	"fmt"
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	"os"
	"time"

	"github.com/alauda/nativestor/pkg/operator/k8sutil"
	"github.com/alauda/nativestor/pkg/util/flags"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

var (
	operatorImage string
	logger        = capnslog.NewPackageLogger("topolvm/operator", "topolvm-cmd")
)

const (
	TopolvmEnvVarPrefix = "TOPOLVM"
	terminationLog      = "/dev/termination-log"
)

var RootCmd = &cobra.Command{
	Use: "topolvm",
}

func init() {
	RootCmd.PersistentFlags().StringVar(&topolvm.LogLevelRaw, "log-level", "INFO", "logging level for logging/tracing output (valid values: CRITICAL,ERROR,WARNING,NOTICE,INFO,DEBUG,TRACE)")
	RootCmd.Flags().StringVar(&topolvm.CSIKubeletRootDir, "csi-kubelet-root-dir", "/var/lib/kubelet/", "csi kubelet root dir")
	RootCmd.Flags().StringVar(&topolvm.EnableDiscoverDevices, "enable-discover-devices", "false", "enable discover devices")
	RootCmd.Flags().BoolVar(&topolvm.IsOperatorHub, "is-operator-hub", true, "is operator or not")
	RootCmd.Flags().DurationVar(&topolvm.CheckStatusInterval, "check-status-interval", 10*time.Second, "check cluster status interval")
	flags.SetFlagsFromEnv(RootCmd.Flags(), TopolvmEnvVarPrefix)
	flags.SetFlagsFromEnv(RootCmd.PersistentFlags(), TopolvmEnvVarPrefix)
}

func GetOperatorImage(clientset kubernetes.Interface, containerName string) string {

	// If provided as a flag then use that value
	if operatorImage != "" {
		return operatorImage
	}

	// Getting the info of the operator pod
	pod, err := k8sutil.GetRunningPod(clientset)
	TerminateOnError(err, "failed to get pod")

	// Get the actual operator container image name
	containerImage, err := k8sutil.GetContainerImage(pod, containerName)
	TerminateOnError(err, "failed to get container image")

	return containerImage
}

// TerminateOnError terminates if err is not nil
func TerminateOnError(err error, msg string) {
	if err != nil {
		TerminateFatal(fmt.Errorf("%s: %+v", msg, err))
	}
}

// TerminateFatal terminates the process with an exit code of 1
// and writes the given reason to stderr and the termination log file.
func TerminateFatal(reason error) {
	fmt.Fprintln(os.Stderr, reason)

	file, err := os.OpenFile(terminationLog, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("failed to write message to termination log: %+v", err))
	} else {
		// #nosec G307 Calling defer to close the file without checking the error return is not a risk for a simple file open and close
		defer file.Close()
		if _, err = file.WriteString(reason.Error()); err != nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf("failed to write message to termination log: %+v", err))
		}
		if err := file.Close(); err != nil {
			logger.Errorf("failed to close file. %v", err)
		}
	}

	os.Exit(1)
}
