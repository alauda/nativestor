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

package discovercmd

import (
	"context"
	rawdevicev1 "github.com/alauda/topolvm-operator/apis/rawdevice/v1"
	rawclient "github.com/alauda/topolvm-operator/generated/nativestore/rawdevice/clientset/versioned"
	"github.com/alauda/topolvm-operator/generated/nativestore/rawdevice/informers/externalversions"
	"github.com/pkg/errors"
	"os"
	"time"

	topolvmv2 "github.com/alauda/topolvm-operator/apis/topolvm/v2"
	"github.com/alauda/topolvm-operator/cmd/topolvm"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	topolvmcluster "github.com/alauda/topolvm-operator/pkg/cluster/topolvm"
	opediscover "github.com/alauda/topolvm-operator/pkg/operator/discover"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	scheme                  = runtime.NewScheme()
	discoverDevicesInterval time.Duration
)

var DiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "discover available devices",
}

const (
	ResyncPeriodOfRawDeviceInformer = 1 * time.Hour
)

func init() {
	DiscoverCmd.Flags().DurationVar(&discoverDevicesInterval, "discover-interval", 60*time.Second, "interval between discovering devices (default 60m)")
	utilruntime.Must(topolvmv2.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(rawdevicev1.AddToScheme(scheme))
	DiscoverCmd.RunE = discover
}

func discover(cmd *cobra.Command, args []string) error {
	ctx := cluster.NewContext()
	clientset, err := rawclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		topolvm.TerminateOnError(err, "create raw device client set failed")
		return err
	}
	nodeName := os.Getenv(topolvmcluster.NodeNameEnv)
	namespace := os.Getenv(topolvmcluster.PodNameSpaceEnv)
	if nodeName == "" || namespace == "" {
		topolvm.TerminateOnError(errors.New("can not get node name and namespace"), "")
	}

	useLoop := false

	if os.Getenv(topolvmcluster.UseLoopEnv) == topolvmcluster.UseLoop {
		useLoop = true
	} else {
		useLoop = false
	}
	ctx.RawDeviceClientset = clientset
	factory := externalversions.NewSharedInformerFactory(ctx.RawDeviceClientset, ResyncPeriodOfRawDeviceInformer)
	rawDeviceLister := factory.Rawdevice().V1().RawDevices().Lister()
	udevEventPeriod := time.Duration(5) * time.Second

	deviceManager := opediscover.NewDeviceManager(ctx, udevEventPeriod, discoverDevicesInterval, rawDeviceLister, nodeName, namespace, useLoop)

	factory.Start(context.TODO().Done())
	err = deviceManager.Run()
	if err != nil {
		topolvm.TerminateFatal(err)
	}
	return nil
}
