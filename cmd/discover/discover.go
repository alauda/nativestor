package discover

import (
	topolvmv1 "github.com/alauda/topolvm-operator/api/v1"
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
	DiscoverCmd.Flags().DurationVar(&discoverDevicesInterval, "discover-interval", 60*time.Minute, "interval between discovering devices (default 60m)")
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
