package cmd

import (
	"context"
	"errors"
	v1 "github.com/alauda/nativestor/apis/rawdevice/v1"
	"github.com/alauda/nativestor/csi"
	"github.com/alauda/nativestor/driver/raw_device"
	"github.com/alauda/nativestor/generated/nativestore/rawdevice/clientset/versioned"
	"github.com/alauda/nativestor/generated/nativestore/rawdevice/informers/externalversions"
	"github.com/alauda/nativestor/pkg/cluster"
	"github.com/alauda/nativestor/pkg/raw_device/runner"
	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	ResyncPeriodOfCsiInformer = 1 * time.Hour
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

// Run builds and starts the manager with leader election.
func subMain() error {

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))
	nodename := viper.GetString("nodename")
	if len(nodename) == 0 {
		return errors.New("node name is not given")
	}
	setupLog.Info("create kubernetes client set ")
	ctx := cluster.NewContext()
	clientset, err := versioned.NewForConfig(ctx.KubeConfig)
	if err != nil {
		setupLog.Error(err, "create raw device client set failed")
		return err
	}
	ctx.RawDeviceClientset = clientset

	setupLog.Info("create raw-device informer and lister ")
	factory := externalversions.NewSharedInformerFactory(ctx.RawDeviceClientset, ResyncPeriodOfCsiInformer)
	rawDeviceLister := factory.Rawdevice().V1().RawDevices().Lister()

	setupLog.Info("register csi node server")
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(ErrorLoggingInterceptor))
	csi.RegisterIdentityServer(grpcServer, raw_device.NewIdentityService())
	csi.RegisterNodeServer(grpcServer, raw_device.NewNodeService(ctx, rawDeviceLister, nodename))
	controllerServer := runner.NewGRPCRunner(grpcServer, config.csiSocket, config.enableLeaderElection)

	run := func(ctx context.Context) {
		factory.Start(ctx.Done())
		setupLog.Info("controller server start")
		err = controllerServer.Start(ctx)
		if err != nil {
			setupLog.Error(err, "start controller server  failed")
		}
	}

	if !config.enableLeaderElection {
		run(context.TODO())
	} else {
		// this lock name pattern is also copied from sigs.k8s.io/sig-storage-lib-external-provisioner/controller
		// to preserve backwards compatibility
		lockName := config.leaderElectionID

		// create a new clientset for leader election

		le := leaderelection.NewLeaderElection(ctx.Clientset, lockName, run)

		if config.leaderElectionNamespace != "" {
			le.WithNamespace(config.leaderElectionNamespace)
		}

		le.WithLeaseDuration(config.leaderElectionLeaseDuration)
		le.WithRenewDeadline(config.leaderElectionRenewDeadline)
		le.WithRetryPeriod(config.leaderElectionRetryPeriod)

		if err := le.Run(); err != nil {
			setupLog.Error(err, "failed to initialize leader election")
			return err
		}
	}

	return nil
}

func ErrorLoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		ctrl.Log.Error(err, "error on grpc call", "method", info.FullMethod)
	}
	return resp, err
}
