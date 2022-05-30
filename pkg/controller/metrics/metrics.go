package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gocrane/crane-scheduler/pkg/datasource"
	"github.com/gocrane/crane-scheduler/pkg/metricnaming"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
)

// MetricClient provides client to interact with data source to fetch metric value.
type MetricClient interface {
	// QueryNodeLatestMetric queries data by kubernetes node.
	QueryNodeMetricLatest(metricName string, node *corev1.Node) (string, int64, error)
}

type metricsClient struct {
	clusterid string
	history   datasource.History
	realtime  datasource.RealTime
}

func NewMetricClient(clusterid string, history datasource.History, realTime datasource.RealTime) MetricClient {
	return &metricsClient{
		clusterid: clusterid,
		history:   history,
		realtime:  realTime,
	}
}

func (m *metricsClient) QueryNodeMetricLatest(metricName string, node *corev1.Node) (string, int64, error) {
	nodeName := node.Name
	nodeIp := ""
	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			nodeIp = addr.Address
			break
		}
	}
	st := time.Now()
	defer func() {
		labels := map[string]string{
			"metric": metricName,
			"node":   nodeName,
			"ip":     nodeIp,
		}
		metrics.NodeMetricQueryLatency.With(labels).Observe(time.Since(st).Seconds())
	}()

	namer := metricnaming.NodeMetricNamer(m.clusterid, nodeName, nodeIp, "", "Node", metricName, labels.Everything())
	end := time.Now().Truncate(1 * time.Minute)
	step := 5 * time.Minute
	start := end.Add(-2 * step)
	results, err := m.history.QueryTimeSeries(context.TODO(), namer, start, end, step)
	if err != nil {
		return "", 0, err
	}
	if len(results) == 0 {
		return "", 0, fmt.Errorf("history query time series is null, clusterid: %v, nodeName: %v, metricName: %v", m.clusterid, nodeName, metricName)
	}
	maxLen := 0
	maxLenSeries := results[0]
	for i := range results {
		if len(results[i].Samples) > maxLen {
			maxLenSeries = results[i]
		}
	}
	n := len(maxLenSeries.Samples)
	return strconv.FormatFloat(maxLenSeries.Samples[n-1].Value, 'f', 5, 64), maxLenSeries.Samples[n-1].Timestamp, nil
}
