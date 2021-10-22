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
	topolvmv1 "github.com/alauda/topolvm-operator/api/v2"
	"github.com/alauda/topolvm-operator/cmd/topolvm"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	opediscover "github.com/alauda/topolvm-operator/pkg/operator/discover"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"time"
)

var (
	scheme                  = runtime.NewScheme()
	discoverDevicesInterval time.Duration
)

var DiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "discover available devices",
}

func init() {
	DiscoverCmd.Flags().DurationVar(&discoverDevicesInterval, "discover-interval", 60*time.Second, "interval between discovering devices (default 60m)")
	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	DiscoverCmd.RunE = discover
}

func discover(cmd *cobra.Command, args []string) error {
	context := cluster.NewContext()
	err := opediscover.Run(context, discoverDevicesInterval)
	if err != nil {
		topolvm.TerminateFatal(err)
	}

	return nil

}
