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

package cleanup

import (
	"fmt"
	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	topolvmclient "github.com/alauda/nativestor/generated/nativestore/topolvm/clientset/versioned"
	"github.com/alauda/nativestor/pkg/cluster"
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	"github.com/alauda/nativestor/pkg/operator/topolvm/clean"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "clean-cmd")
	scheme = runtime.NewScheme()
)

var CleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "clean lvm info",
}

var (
	namespace   string
	clusterName string
	node        string
)

func init() {
	utilruntime.Must(topolvmv2.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	viper.AutomaticEnv()
	flags := CleanCmd.Flags()
	flags.String("pod_namespace", "", "namespace of operator")
	viper.BindPFlag("pod_namespace", flags.Lookup("pod_namespace"))
	flags.String("cluster_name", "", "topolvm cluster name")
	viper.BindPFlag("cluster_name", flags.Lookup("cluster_name"))
	flags.String("node_name", "", "node name")
	viper.BindPFlag("node_name", flags.Lookup("node_name"))
	namespace = viper.GetString("pod_namespace")
	clusterName = viper.GetString("cluster_name")
	node = viper.GetString("node_name")
	CleanCmd.RunE = cleanup
}

func cleanup(cmd *cobra.Command, args []string) error {
	topolvm.SetLogLevel()
	fmt.Printf("namespace %s cluster %s node %s", namespace, clusterName, node)
	context := cluster.NewContext()

	topolvmClientset, err := topolvmclient.NewForConfig(context.KubeConfig)
	if err != nil {
		return err
	}
	context.TopolvmClusterClientset = topolvmClientset
	c := clean.NewCleanUp(node, namespace, clusterName, context)
	return c.Start()
}
