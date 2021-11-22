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
	"context"
	"flag"
	"fmt"
	topolvmv1 "github.com/alauda/topolvm-operator/api/v2"
	"github.com/alauda/topolvm-operator/cmd/topolvm"
	"github.com/alauda/topolvm-operator/controllers"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/metric"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

var AddToManagerFuncs = []func(manager.Manager, *cluster.Context, context.Context, cluster.OperatorConfig) error{
	controllers.Add,
}

func startOperator(cmd *cobra.Command, args []string) error {

	cluster.SetLogLevel()
	var metricsAddr string
	var enableLeaderElection bool
	var leaderElectionID string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "topolvm-operator", "ID for leader election by topolvm operator")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         metricsAddr,
		Port:                       9443,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaderElection:             true,
		LeaderElectionID:           leaderElectionID,
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

	metricsCh := make(chan *cluster.Metrics)
	if err := mgr.Add(metric.NewMetricsExporter(metricsCh)); err != nil {
		return err
	}

	err = controllers.RemoveNodeCapacityAnnotations(ctx.Clientset)
	if err != nil {
		logger.Errorf("RemoveNodeCapacityAnnotations failed err %v", err)
		return fmt.Errorf("RemoveNodeCapacityAnnotations failed err %v", err)
	}

	operatorImage := topolvm.GetOperatorImage(ctx.Clientset, "")
	c := controllers.NewTopolvmClusterReconciler(mgr.GetScheme(), ctx, operatorImage, metricsCh)
	if err := c.SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller", "controller", "TopolvmCluster")
		os.Exit(1)
	}

	opctx := context.TODO()
	config := cluster.OperatorConfig{
		Image: operatorImage,
	}
	for _, f := range AddToManagerFuncs {
		if err := f(mgr, ctx, opctx, config); err != nil {
			return err
		}
	}
	if err != nil {
		logger.Error(err, "problem running manager")
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
