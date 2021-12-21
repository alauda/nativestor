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

package cluster

import (
	"fmt"
	rawclient "github.com/alauda/topolvm-operator/generated/nativestore/rawdevice/clientset/versioned"
	"os"

	topolvmclient "github.com/alauda/topolvm-operator/generated/nativestore/topolvm/clientset/versioned"
	"github.com/alauda/topolvm-operator/pkg/util/exec"
	"github.com/coreos/pkg/capnslog"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "cluster-info")
)

const (
	terminationLog = "/dev/termination-log"
)

type Context struct {

	// The kubernetes config used for this context
	KubeConfig *rest.Config

	// Clientset is a connection to the core kubernetes API
	Clientset kubernetes.Interface

	RawDeviceClientset rawclient.Interface

	TopolvmClusterClientset topolvmclient.Interface

	Client client.Client

	// APIExtensionClientset is a connection to the API Extension kubernetes API
	APIExtensionClientset apiextensionsclient.Interface

	// The implementation of executing a console command
	Executor exec.Executor

	// A value indicating the desired logging/tracing level
	LogLevel capnslog.LogLevel
}

func NewContext() *Context {
	var err error

	context := &Context{
		Executor: &exec.CommandExecutor{},
	}

	// Try to read config from in-cluster env
	context.KubeConfig, err = rest.InClusterConfig()
	if err != nil {

		// **Not** running inside a cluster - running the operator outside of the cluster.
		// This mode is for developers running the operator on their dev machines
		// for faster development, or to run operator cli tools manually to a remote cluster.
		// We setup the API server config from default user file locations (most notably ~/.kube/config),
		// and also change the executor to work remotely and run kubernetes jobs.
		logger.Info("setting up the context to outside of the cluster")

		// Try to read config from user config files
		context.KubeConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{}).ClientConfig()
		TerminateOnError(err, "failed to get k8s config")
	}

	context.Clientset, err = kubernetes.NewForConfig(context.KubeConfig)
	TerminateOnError(err, "failed to create k8s clientset")

	context.APIExtensionClientset, err = apiextensionsclient.NewForConfig(context.KubeConfig)
	TerminateOnError(err, "failed to create k8s API extension clientset")

	return context
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
