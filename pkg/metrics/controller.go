package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

var (
	NodeMetricQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "node_metric_query_duration_seconds",
			Help:      "Latency for node metric query",
			Buckets:   metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"metric", "node", "ip"},
	)
)

var registerControllerMetricsOnce sync.Once

func RegisterController() {
	registerControllerMetricsOnce.Do(func() {
		prometheus.MustRegister(NodeMetricQueryLatency)
	})
}
