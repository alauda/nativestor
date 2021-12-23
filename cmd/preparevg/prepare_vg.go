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

package preparevg

import (
	"fmt"
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	"github.com/alauda/nativestor/pkg/operator/topolvm/volumegroup"
	"os"

	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	topolvmclient "github.com/alauda/nativestor/generated/nativestore/topolvm/clientset/versioned"
	"github.com/alauda/nativestor/pkg/cluster"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "prepare-vg-cmd")
	scheme = runtime.NewScheme()
)

var PrepareVgCmd = &cobra.Command{
	Use:   "prepareVg",
	Short: "Check Disk and Create Volume group",
}

func init() {
	utilruntime.Must(topolvmv2.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	PrepareVgCmd.RunE = prepareVg
}

func prepareVg(cmd *cobra.Command, args []string) error {

	topolvm.SetLogLevel()

	nodeName := os.Getenv(topolvm.NodeNameEnv)
	if nodeName == "" {
		logger.Errorf("get env:%s failed", topolvm.NodeNameEnv)
		return fmt.Errorf("get env %s failed", topolvm.NodeNameEnv)
	}

	namespace := os.Getenv(topolvm.PodNameSpaceEnv)
	if namespace == "" {
		logger.Errorf("get env %s failed", topolvm.PodNameSpaceEnv)
		return fmt.Errorf("get env %s failed", topolvm.PodNameSpaceEnv)
	}

	topolvmClusterName := os.Getenv(topolvm.ClusterNameEnv)
	if namespace == "" {
		logger.Errorf("get env %s failed", topolvm.ClusterNameEnv)
		return fmt.Errorf("get env %s failed", topolvm.ClusterNameEnv)
	}

	context := cluster.NewContext()

	topolvmClientset, err := topolvmclient.NewForConfig(context.KubeConfig)
	if err != nil {
		return err
	}
	context.TopolvmClusterClientset = topolvmClientset
	c := volumegroup.NewPrepareVg(nodeName, namespace, topolvmClusterName, context)
	return c.Start()
}
