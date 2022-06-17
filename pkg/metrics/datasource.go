package metrics

import (
	"k8s.io/kubernetes/pkg/util/maps"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"

	"github.com/gocrane/crane-scheduler/pkg/metricquery"
)

var (
	DataSourceMetricQueryLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "datasource_metric_query_duration_seconds",
			Help:      "Latency for data source metric query",
			Buckets:   metrics.ExponentialBuckets(0.001, 2, 20),
		},
		[]string{"metric_name", "metric_type", "datasource", "query_type", "apiversion", "kind", "namespace", "workload", "pod", "container", "node", "node_ip", "promql", "selector"},
	)

	DataSourceMetricQueryErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "datasource_metric_query_error",
			Help:      "count of errors for data source metric query",
		},
		[]string{"metric_name", "metric_type", "datasource", "query_type", "apiversion", "kind", "namespace", "workload", "pod", "container", "node", "node_ip", "promql", "selector"},
	)

	DataSourceMetricQueryErrorDetailGuage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "crane",
			Subsystem: "scheduling",
			Name:      "datasource_metric_query_error_detail",
			Help:      "detail of errors for data source metric query",
		},
		[]string{"metric_name", "metric_type", "datasource", "query_type", "apiversion", "kind", "namespace", "workload", "pod", "container", "node", "node_ip", "promql", "selector", "error"},
	)
)

var registerDatasourceMetricsOnce sync.Once

func RegisterDataSource() {
	registerDatasourceMetricsOnce.Do(func() {
		prometheus.MustRegister(DataSourceMetricQueryLatency)
		prometheus.MustRegister(DataSourceMetricQueryErrorCounter)
		prometheus.MustRegister(DataSourceMetricQueryErrorDetailGuage)

	})
}

func RecordDataSourceMetricQueryLatency(metric *metricquery.Metric, start time.Time, datasource, query_type string) {
	labels, buildErr := metric.BuildPromLabels()
	if buildErr != nil {
		return
	}
	labels["datasource"] = datasource
	labels["query_type"] = query_type

	DataSourceMetricQueryLatency.With(labels).Observe(time.Since(start).Seconds())
}

func RecordDataSourceMetricQueryError(metric *metricquery.Metric, err error, datasource, query_type string) {
	counterLabels, buildErr := metric.BuildPromLabels()
	if buildErr != nil {
		return
	}
	counterLabels["datasource"] = datasource
	counterLabels["query_type"] = query_type

	guageLabels := maps.CopySS(counterLabels)

	if err != nil {
		DataSourceMetricQueryErrorCounter.With(counterLabels).Inc()
		guageLabels["error"] = err.Error()
		DataSourceMetricQueryErrorDetailGuage.With(guageLabels).Set(1)
	} else {
		guageLabels["error"] = ""
		DataSourceMetricQueryErrorDetailGuage.With(guageLabels).Set(0)
	}
}
