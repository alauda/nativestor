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
	topolvmv1 "github.com/alauda/topolvm-operator/api/v1"
	"github.com/alauda/topolvm-operator/controllers"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
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
	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	PrepareVgCmd.RunE = prepareVg
}

func prepareVg(cmd *cobra.Command, args []string) error {

	cluster.SetLogLevel()

	nodeName := os.Getenv(cluster.NodeNameEnv)
	if nodeName == "" {
		logger.Errorf("get env:%s failed", cluster.NodeNameEnv)
		return fmt.Errorf("get env %s failed", cluster.NodeNameEnv)
	}

	namespace := os.Getenv(cluster.PodNameSpaceEnv)
	if namespace == "" {
		logger.Errorf("get env %s failed", cluster.PodNameSpaceEnv)
		return fmt.Errorf("get env %s failed", cluster.PodNameSpaceEnv)
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme,
		LeaderElection: false,
	})
	if err != nil {
		return err
	}

	context := cluster.NewContext()
	context.Client = mgr.GetClient()

	// used to stop controller when job is completed
	ctx := ctrl.SetupSignalHandler()

	c := controllers.NewPrepareVgController(nodeName, namespace, context)

	if err := c.SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "Node")
		return err
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		logger.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil

}
