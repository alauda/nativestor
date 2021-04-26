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

package operator

import (
	"flag"
	"fmt"
	topolvmv1 "github.com/alauda/topolvm-operator/api/v1"
	"github.com/alauda/topolvm-operator/cmd/topolvm"
	"github.com/alauda/topolvm-operator/controllers"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var OperatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Check Disk and Create Volume group",
}

var (
	scheme = runtime.NewScheme()
	logger = capnslog.NewPackageLogger("topolvm/operator", "topolvm-cluster")
)

func init() {
	OperatorCmd.RunE = startOperator
	addScheme()
}

func addScheme() {

	_ = clientgoscheme.AddToScheme(scheme)

	_ = topolvmv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func startOperator(cmd *cobra.Command, args []string) error {

	cluster.SetLogLevel()
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "355331c5.cybozu.com",
	})

	if err != nil {
		logger.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := cluster.NewContext()
	ctx.Client = mgr.GetClient()

	cluster.NameSpace = os.Getenv(cluster.PodNameSpaceEnv)
	if cluster.NameSpace == "" {
		logger.Errorf("unable get env %s ", cluster.PodNameSpaceEnv)
		return fmt.Errorf("get env:%s failed ", cluster.PodNameSpaceEnv)
	}

	cluster.IsOperatorHub = os.Getenv(cluster.IsOperatorHubEnv)
	if cluster.NameSpace == "" {
		logger.Errorf("unable get env %s ", cluster.IsOperatorHubEnv)
		return fmt.Errorf("get env:%s failed ", cluster.IsOperatorHubEnv)
	}

	err = controllers.RemoveNodeCapacityAnnotations(ctx.Client)
	if err != nil {
		logger.Errorf("RemoveNodeCapacityAnnotations failed err %v", err)
		return fmt.Errorf("RemoveNodeCapacityAnnotations failed err %v", err)
	}

	operatorImage := topolvm.GetOperatorImage(ctx.Clientset, "")
	c := controllers.NewTopolvmClusterReconciler(mgr.GetScheme(), ctx, operatorImage)
	if err := c.SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "TopolvmCluster")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "problem running manager")
		os.Exit(1)
	}

	return nil
}
