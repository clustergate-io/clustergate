package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// CheckReady is a gauge that reports whether each individual check is passing.
	// Labels: check (check name), cluster_readiness (CR name), severity, category.
	CheckReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "clustergate",
			Name:      "check_ready",
			Help:      "Whether a readiness check is passing (1) or failing (0).",
		},
		[]string{"check", "cluster_readiness", "severity", "category"},
	)

	// CheckDuration is a histogram that records how long each check takes to run.
	// Labels: check (check name), severity, category.
	CheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "clustergate",
			Name:      "check_duration_seconds",
			Help:      "Duration of readiness check execution in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"check", "severity", "category"},
	)

	// ClusterReady is a gauge that reports overall cluster readiness.
	// Labels: cluster_readiness (CR name).
	ClusterReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "clustergate",
			Name:      "cluster_ready",
			Help:      "Whether the cluster is fully ready (all critical checks passing).",
		},
		[]string{"cluster_readiness"},
	)

	// ClusterHealthState is a gauge that reports the cluster health state.
	// Labels: cluster_readiness (CR name), state (Healthy, Degraded, Unhealthy).
	// The active state has value 1, others have value 0.
	ClusterHealthState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "clustergate",
			Name:      "cluster_health_state",
			Help:      "Cluster health state: Healthy (all passing), Degraded (warnings failing), Unhealthy (critical failing). Active state=1.",
		},
		[]string{"cluster_readiness", "state"},
	)

	// CategoryReady is a gauge that reports per-category readiness.
	// Labels: category, cluster_readiness (CR name).
	CategoryReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "clustergate",
			Name:      "category_ready",
			Help:      "Whether all critical checks in a category are passing.",
		},
		[]string{"category", "cluster_readiness"},
	)
)

func init() {
	metrics.Registry.MustRegister(CheckReady, CheckDuration, ClusterReady, ClusterHealthState, CategoryReady)
}
