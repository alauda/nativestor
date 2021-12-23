package cmd

import (
	"flag"
	"fmt"
	raw_device2 "github.com/alauda/nativestor/pkg/raw_device"
	"github.com/spf13/viper"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var config struct {
	csiSocket                   string
	metricsAddr                 string
	probeAddr                   string
	webhookAddr                 string
	enableLeaderElection        bool
	certDir                     string
	leaderElectionID            string
	httpEndpoint                string
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	zapOpts                     zap.Options
}

var rootCmd = &cobra.Command{
	Use:     "raw-device-plugin",
	Version: raw_device2.Version,
	Short:   "Raw device plugin",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.Flags()

	fs.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&config.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&config.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&config.webhookAddr, "webhook-addr", ":9443", "Listen address for the webhook endpoint")
	fs.StringVar(&config.certDir, "cert-dir", "", "certificate directory")
	fs.StringVar(&config.leaderElectionNamespace, "leader-election-namespace", "", "Namespace where the leader election resource lives. Defaults to the pod namespace if not set.")
	fs.StringVar(&config.csiSocket, "csi-socket", raw_device2.DefaultCSISocket, "UNIX domain socket filename for CSI")
	fs.StringVar(&config.leaderElectionID, "leader-election-id", "raw-device", "ID for leader election by controller-runtime")
	fs.DurationVar(&config.leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
	fs.DurationVar(&config.leaderElectionRenewDeadline, "leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up. Defaults to 10 seconds.")
	fs.DurationVar(&config.leaderElectionRetryPeriod, "leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions. Defaults to 5 seconds.")
	viper.BindEnv("nodename", "NODE_NAME")
	viper.BindPFlag("nodename", fs.Lookup("nodename"))
	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	config.zapOpts.BindFlags(goflags)

	fs.AddGoFlagSet(goflags)
}
