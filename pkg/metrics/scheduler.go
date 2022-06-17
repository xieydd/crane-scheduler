package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
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

	ExtenderHandlerErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_handler_error",
			Help:      "count of errors for extender handler",
		},
		[]string{"handler_name", "pod", "namespace"},
	)

	ExtenderHandlerErrorDetailGuage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "extender_handler_error_detail",
			Help:      "detail of errors for extender handler",
		},
		[]string{"handler_name", "pod", "namespace", "error"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	prometheus.MustRegister(ExtenderPredicateLatency, ExtenderPriorityLatency, ExtenderPredicateNodeLatency, ExtenderPredicateHandlerLatency, ExtenderPriorityHandlerLatency)

}

func CustomCollectorRegister(collector ...prometheus.Collector) {
	prometheus.MustRegister(collector...)
}

func RecordExtenderHandlerError(handler string, pod *corev1.Pod, err error) {
	podname := ""
	namespace := ""
	if pod != nil {
		podname = pod.Name
		namespace = pod.Namespace
	}
	if err != nil {
		ExtenderHandlerErrorCounter.With(prometheus.Labels{
			"handler_name": handler,
			"pod":          podname,
			"namespace":    namespace,
		}).Inc()
		ExtenderHandlerErrorDetailGuage.With(prometheus.Labels{
			"handler_name": handler,
			"pod":          podname,
			"namespace":    namespace,
			"error":        err.Error(),
		}).Set(1)
	} else {
		ExtenderHandlerErrorDetailGuage.With(prometheus.Labels{
			"handler_name": handler,
			"pod":          podname,
			"namespace":    namespace,
			"error":        "",
		}).Set(0)
	}
}
