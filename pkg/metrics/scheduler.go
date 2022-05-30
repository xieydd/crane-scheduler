package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

var (
	ExtenderPredicateHandlerLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_predicate_handler_duration_seconds",
			Help:      "Latency for extender predicate handler",
			Buckets:   metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"predicate_name"},
	)

	ExtenderPriorityHandlerLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_priority_handler_duration_seconds",
			Help:      "Latency for extender priority handler",
			// Start with 10ms with the last bucket being [~88m, Inf).
			Buckets: metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"priority_name"},
	)

	ExtenderPredicateLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_predicate_duration_seconds",
			Help:      "Latency for extender predicate func",
			// Start with 10ms with the last bucket being [~88m, Inf).
			Buckets: metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"predicate_name", "pod", "namespace"},
	)

	ExtenderPredicateNodeLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_predicate_node_duration_seconds",
			Help:      "Latency for extender predicate one node",
			Buckets:   metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"predicate_name", "pod", "namespace", "node"},
	)

	ExtenderPriorityLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_priority_duration_seconds",
			Help:      "Latency for extender priority func",
			Buckets:   metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"priority_name", "pod", "namespace"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	prometheus.MustRegister(ExtenderPredicateLatency, ExtenderPriorityLatency, ExtenderPredicateNodeLatency, ExtenderPredicateHandlerLatency, ExtenderPriorityHandlerLatency)

}

func CustomCollectorRegister(collector ...prometheus.Collector) {
	prometheus.MustRegister(collector...)
}
