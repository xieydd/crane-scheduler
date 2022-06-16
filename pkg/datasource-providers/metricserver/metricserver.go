package metricserver

import (
	gocontext "context"
	"time"

	cacheddiscovery "k8s.io/client-go/discovery/cached"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	customclient "k8s.io/metrics/pkg/client/custom_metrics"
	externalclient "k8s.io/metrics/pkg/client/external_metrics"

	"github.com/gocrane/crane-scheduler/pkg/common"
	"github.com/gocrane/crane-scheduler/pkg/datasource"
	"github.com/gocrane/crane-scheduler/pkg/metricnaming"
	"github.com/gocrane/crane-scheduler/pkg/metricquery"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
	"github.com/gocrane/crane-scheduler/pkg/querybuilder"
)

var _ datasource.RealTime = &metricsServer{}

//??? do we need to cache all resource metrics to avoid traffic to apiserver. because vpa to apiserver call is triggered by time tick to list all metrics periodically,
// it can be controlled by a unified loop. but crane to apiserver call is triggered by each metric prediction query, the traffic can not be controlled universally.
// maybe we can use clients rate limiter.
type metricsServer struct {
	client MetricsClient
}

func (m *metricsServer) QueryLatestTimeSeries(ctx gocontext.Context, metricNamer metricnaming.MetricNamer) ([]*common.TimeSeries, error) {
	ts := time.Now()
	defer func() {
		metrics.RecordDataSourceMetricQueryLatency(metricNamer.GetMetric(), ts, string(metricquery.MetricServerMetricSource), datasource.RealTimeQuery)
	}()

	msBuilder := metricNamer.QueryBuilder().Builder(metricquery.MetricServerMetricSource)
	msQuery, err := msBuilder.BuildQuery(querybuilder.BuildQueryBehavior{})
	if err != nil {
		klog.Errorf("Failed to QueryLatestTimeSeries metricNamer %v, err: %v", metricNamer.BuildUniqueKey(), err)
		return nil, err
	}
	klog.V(6).Infof("QueryLatestTimeSeries metricNamer %v", metricNamer.BuildUniqueKey())
	return m.client.GetMetricValue(msQuery.MetricServer.Metric)
}

func NewProvider(config *rest.Config) (datasource.RealTime, error) {
	rootClient, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	// Use a discovery client capable of being refreshed.
	discoveryClientSet := rootClient.Discovery()
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClientSet)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	apiVersionsGetter := customclient.NewAvailableAPIsGetter(discoveryClientSet)

	resourceClient, err := resourceclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	customClient := customclient.NewForConfig(config, restMapper, apiVersionsGetter)

	externalClient, err := externalclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &metricsServer{
		client: NewCraneMetricsClient(resourceClient, customClient, externalClient),
	}, nil
}
