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
	topolvmcommon "github.com/alauda/topolvm-operator/pkg/cluster/topolvm"
	"github.com/alauda/topolvm-operator/pkg/operator"
	"github.com/alauda/topolvm-operator/pkg/operator/discover"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	rawdev_csi "github.com/alauda/topolvm-operator/pkg/operator/raw_device/csi"
	topolvmctr "github.com/alauda/topolvm-operator/pkg/operator/topolvm/controller"
	topolvm_csi "github.com/alauda/topolvm-operator/pkg/operator/topolvm/csi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"

	rawv1 "github.com/alauda/topolvm-operator/apis/rawdevice/v1"
	topolvmv2 "github.com/alauda/topolvm-operator/apis/topolvm/v2"
	"github.com/alauda/topolvm-operator/cmd/topolvm"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/coreos/pkg/capnslog"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
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
	_ = topolvmv2.AddToScheme(scheme)
	_ = rawv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

var AddToManagerFuncs = []func(manager.Manager, *cluster.Context, context.Context, operator.OperatorConfig) error{
	topolvmctr.Add,
	rawdev_csi.Add,
	topolvm_csi.Add,
}

func startOperator(cmd *cobra.Command, args []string) error {

	topolvmcommon.SetLogLevel()
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	topolvmcommon.NameSpace = os.Getenv(topolvmcommon.PodNameSpaceEnv)
	if topolvmcommon.NameSpace == "" {
		logger.Errorf("unable get env %s ", topolvmcommon.PodNameSpaceEnv)
		return fmt.Errorf("get env:%s failed ", topolvmcommon.PodNameSpaceEnv)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		Namespace:              topolvmcommon.NameSpace,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "c6b32c27.cybozu.com",
	})
	if err != nil {
		logger.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := cluster.NewContext()
	ctx.Client = mgr.GetClient()
	err = topolvmctr.RemoveNodeCapacityAnnotations(ctx.Clientset)
	if err != nil {
		logger.Errorf("RemoveNodeCapacityAnnotations failed err %v", err)
		return fmt.Errorf("RemoveNodeCapacityAnnotations failed err %v", err)
	}

	operatorImage := topolvm.GetOperatorImage(ctx.Clientset, "")

	opctx := context.TODO()
	setting, err := ctx.Clientset.CoreV1().ConfigMaps(topolvmcommon.NameSpace).Get(opctx, operator.OperatorSettingConfigMapName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "unable get configmap operator setting", "configmap", operator.OperatorSettingConfigMapName)
	}

	config := operator.OperatorConfig{
		Image:             operatorImage,
		NamespaceToWatch:  topolvmcommon.NameSpace,
		Parameters:        setting.Data,
		OperatorNamespace: topolvmcommon.NameSpace,
	}

	enableRawDev := k8sutil.GetValue(config.Parameters, operator.EnableRawDeviceEnv, "false")
	if enableRawDev == "true" {
		discover.MakeDiscoverDevicesDaemonset(ctx.Clientset, operator.DiscoverAppName, operatorImage, true, true)
	} else {
		discover.MakeDiscoverDevicesDaemonset(ctx.Clientset, operator.DiscoverAppName, operatorImage, true, true)
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

	//+kubebuilder:scaffold:builder
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "problem running manager")
		os.Exit(1)
	}
	return nil
}
