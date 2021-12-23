package metric

import (
	"context"
	"fmt"
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricsNamespace = "topolvm"
)

type metricsExporter struct {
	clusterStatus *prometheus.GaugeVec
	nodeStatus    *prometheus.GaugeVec
	updater       chan *topolvm.Metrics
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

func (m metricsExporter) Start(ctx context.Context) error {

	go func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case met := <-m.updater:
				fmt.Println("metric coming")
				m.clusterStatus.WithLabelValues(met.Cluster).Set(float64(met.ClusterStatus))
				for _, ele := range met.NodeStatus {
					m.nodeStatus.WithLabelValues(ele.Node).Set(float64(ele.Status))
				}
			}
		}
	}()

	return nil
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (m *metricsExporter) NeedLeaderElection() bool {
	return false
}

func NewMetricsExporter(updater chan *topolvm.Metrics) manager.Runnable {
	clusterStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: "health",
		Name:      "cluster_status",
		Help:      "Topolvm cluster status",
	}, []string{"cluster_name"})
	metrics.Registry.MustRegister(clusterStatus)

	nodeStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: "health",
		Name:      "node_status",
		Help:      "Topolvm node status",
	}, []string{"node_name"})
	metrics.Registry.MustRegister(nodeStatus)

	return &metricsExporter{
		clusterStatus: clusterStatus,
		nodeStatus:    nodeStatus,
		updater:       updater,
	}
}
