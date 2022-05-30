package qcloudmonitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"

	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud"
	qconsts "github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/consts"
	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/credential"
	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/qmonitor"
	"github.com/gocrane/crane-scheduler/pkg/common"
	"github.com/gocrane/crane-scheduler/pkg/datasource"
	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/metricnaming"
	"github.com/gocrane/crane-scheduler/pkg/metricquery"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
	"github.com/gocrane/crane-scheduler/pkg/querybuilder"
)

const (
	DefaultStep = 1 * time.Minute
)

var _ datasource.Interface = &qcloudmonitor{}

type qcloudmonitor struct {
	cmClient *qmonitor.QCloudMonitorClient
	step     time.Duration
}

func FillDefaultValue(config *datasource.QCloudMonitorConfig) {
	if config == nil {
		return
	}
	if config.ClientProfile.DefaultTimeoutSeconds <= 0 {
		config.ClientProfile.DefaultTimeoutSeconds = 15
	}
	if config.ClientProfile.DefaultLanguage == "" {
		config.ClientProfile.DefaultLanguage = "zh-CN"
	}
	if config.ClientProfile.DefaultLimit <= 0 {
		config.ClientProfile.DefaultLimit = 100
	}
	if config.ClientProfile.DomainSuffix == "" {
		config.ClientProfile.DomainSuffix = "tencentcloudapi.com"
	}
	if config.ClientProfile.Scheme == "" {
		config.ClientProfile.Scheme = "https"
	}

}

// NewProvider return a QCloud Monitor data provider
func NewProvider(config *datasource.QCloudMonitorConfig) (datasource.Interface, error) {
	cm := &qcloudmonitor{}
	FillDefaultValue(config)
	cred := credential.NewQCloudNormCredential(config.ClusterId, config.AppId, config.SecretId, config.SecretKey, 1*time.Hour)
	qcp := qcloud.QCloudClientProfile{
		Region:          config.Region,
		DomainSuffix:    config.DomainSuffix,
		Scheme:          config.Scheme,
		DefaultLimit:    config.DefaultLimit,
		DefaultLanguage: config.DefaultLanguage,
		DefaultTimeout:  time.Duration(config.DefaultTimeoutSeconds) * time.Second,
		Debug:           config.Debug,
	}
	klog.Infof("QCloudMonitorConfig: %+v, QCloudClientProfile: %+v", *config, qcp)
	qclouClientConf := &qcloud.QCloudClientConfig{
		DefaultRetryCnt:     qconsts.MAXRETRY,
		Credential:          cred,
		QCloudClientProfile: qcp,
		RateLimiter:         flowcontrol.NewTokenBucketRateLimiter(10, 20),
	}
	cm.cmClient = qmonitor.NewQCloudMonitorClient(qclouClientConf)
	cm.step = DefaultStep
	return cm, nil
}

// NewProvider return a QCloud Monitor data provider, test only
func NewProviderByCmClient(cmClient *qmonitor.QCloudMonitorClient) (datasource.Interface, error) {
	cm := &qcloudmonitor{}
	cm.cmClient = cmClient
	cm.step = DefaultStep
	return cm, nil
}

func (qm *qcloudmonitor) Name() string {
	return "qcloudmonitor"
}

func (qm *qcloudmonitor) EnableDebug() {
	qm.cmClient.EnableDebug()
}

func (qm *qcloudmonitor) QueryLatestTimeSeries(ctx context.Context, metricNamer metricnaming.MetricNamer) ([]*common.TimeSeries, error) {
	ts := time.Now()
	defer func() {
		metrics.RecordDataSourceMetricQueryLatency(metricNamer.GetMetric(), ts, string(metricquery.QCloudMonitorMetricSource), datasource.RealTimeQuery)
	}()
	builder := metricNamer.QueryBuilder().Builder(metricquery.QCloudMonitorMetricSource)
	if builder == nil {
		return nil, fmt.Errorf("nil builder for %v", metricNamer.BuildUniqueKey())
	}
	query, err := builder.BuildQuery(querybuilder.BuildQueryBehavior{FederatedClusterScope: true})
	if err != nil {
		return nil, err
	}
	endTime := time.Now()
	startTime := endTime.Add(-qm.step).Truncate(qm.step)
	return qm.query(ctx, query.QCloudMonitor.Metric, startTime, endTime, qm.step)
}

func (qm *qcloudmonitor) QueryTimeSeries(ctx context.Context, metricNamer metricnaming.MetricNamer, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	ts := time.Now()
	defer func() {
		metrics.RecordDataSourceMetricQueryLatency(metricNamer.GetMetric(), ts, string(metricquery.QCloudMonitorMetricSource), datasource.HistoryQuery)
	}()

	builder := metricNamer.QueryBuilder().Builder(metricquery.QCloudMonitorMetricSource)
	if builder == nil {
		return nil, fmt.Errorf("nil builder for %v", metricNamer.BuildUniqueKey())
	}
	query, err := builder.BuildQuery(querybuilder.BuildQueryBehavior{FederatedClusterScope: true})
	if err != nil {
		return nil, err
	}
	return qm.query(ctx, query.QCloudMonitor.Metric, startTime, endTime, step)
}

func (qm *qcloudmonitor) query(ctx context.Context, metric *metricquery.Metric, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	if metric == nil {
		return nil, fmt.Errorf("metric is null")
	}
	switch metric.Type {
	case metricquery.PodMetricType:
		return qm.podMetric(ctx, metric, startTime, endTime, step)
	case metricquery.WorkloadMetricType:
		return qm.workloadMetric(ctx, metric, startTime, endTime, step)
	case metricquery.ContainerMetricType:
		return qm.containerMetric(ctx, metric, startTime, endTime, step)
	case metricquery.NodeMetricType:
		return qm.nodeMetric(ctx, metric, startTime, endTime, step)
	case metricquery.PromQLMetricType:
		return nil, fmt.Errorf("qcloudmonitor do not support metric type %v", metric.Type)
	default:
		return nil, fmt.Errorf("unknown metric type %v", metric.Type)
	}
}

func (qm *qcloudmonitor) workloadMetric(ctx context.Context, metric *metricquery.Metric, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	selector := metric.Workload.Selector
	if selector == nil {
		return nil, fmt.Errorf("selector is null, require label %v", known.LabelClusterId)
	}
	id, exists := selector.RequiresExactMatch(known.LabelClusterId)
	if !exists {
		return nil, fmt.Errorf("require label %v", known.LabelClusterId)
	}
	conds := MakeWorkloadMetricConditions(id, metric.Workload.Namespace, metric.Workload.Name, metric.Workload.Kind)

	if err := checkQueryWorkloadCondition(conds); err != nil {
		return []*common.TimeSeries{}, err
	}

	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadCpuCoreUsedMetric, conds, startTime, endTime, step)
	case v1.ResourceMemory.String():
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadMemUsageBytesMetric, conds, startTime, endTime, step)
	case known.MetricCpuRequest:
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadCpuRequestsMetric, conds, startTime, endTime, step)
	case known.MetricCpuLimit:
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadCpuLimitsMetric, conds, startTime, endTime, step)
	case known.MetricMemRequest:
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadMemRequestsMetric, conds, startTime, endTime, step)
	case known.MetricMemLimit:
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadMemLimitsMetric, conds, startTime, endTime, step)
	case known.MetricWorkloadReplicas:
		return qm.getMonitorData(ctx, qmonitor.K8sWorkloadReplicasMetric, conds, startTime, endTime, step)
	default:
		return nil, fmt.Errorf("not supported metric name %v for qcloud monitor", metric.MetricName)
	}
}

func (qm *qcloudmonitor) containerMetric(ctx context.Context, metric *metricquery.Metric, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	selector := metric.Container.Selector
	if selector == nil {
		return nil, fmt.Errorf("selector is null, require label %v", known.LabelClusterId)
	}
	id, exists := selector.RequiresExactMatch(known.LabelClusterId)
	if !exists {
		return nil, fmt.Errorf("require label %v", known.LabelClusterId)
	}
	conds := MakeContainerMetricConditions(id, metric.Container.Namespace, metric.Container.WorkloadName, metric.Container.ContainerName, "")

	if err := checkQueryContainerCondition(conds); err != nil {
		return []*common.TimeSeries{}, err
	}

	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return qm.getMonitorData(ctx, qmonitor.K8sContainerCpuCoreUsedMetric, conds, startTime, endTime, step)
	case v1.ResourceMemory.String():
		return qm.getMonitorData(ctx, qmonitor.K8sContainerMemUsageBytesMetric, conds, startTime, endTime, step)
	case known.MetricCpuRequest:
		return qm.getMonitorData(ctx, qmonitor.K8sContainerCpuCoreRequestMetric, conds, startTime, endTime, step)
	case known.MetricCpuLimit:
		return qm.getMonitorData(ctx, qmonitor.K8sContainerCpuCoreLimitMetric, conds, startTime, endTime, step)
	case known.MetricMemRequest:
		return qm.getMonitorData(ctx, qmonitor.K8sContainerMemRequestMetric, conds, startTime, endTime, step)
	case known.MetricMemLimit:
		return qm.getMonitorData(ctx, qmonitor.K8sContainerMemLimitMetric, conds, startTime, endTime, step)
	default:
		return nil, fmt.Errorf("not supported metric name %v for qcloud monitor", metric.MetricName)
	}
}

func (qm *qcloudmonitor) podMetric(ctx context.Context, metric *metricquery.Metric, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	selector := metric.Pod.Selector
	if selector == nil {
		return nil, fmt.Errorf("selector is null, require label %v", known.LabelClusterId)
	}
	id, exists := selector.RequiresExactMatch(known.LabelClusterId)
	if !exists {
		return nil, fmt.Errorf("require label %v", known.LabelClusterId)
	}
	conds := MakePodMetricConditions(id, metric.Pod.Namespace, metric.Pod.Name, "", "")

	if err := checkQueryPodCondition(conds); err != nil {
		return []*common.TimeSeries{}, err
	}

	switch strings.ToLower(metric.MetricName) {
	case v1.ResourceCPU.String():
		return qm.getMonitorData(ctx, qmonitor.K8sPodCpuCoreUsedMetric, conds, startTime, endTime, step)
	case v1.ResourceMemory.String():
		return qm.getMonitorData(ctx, qmonitor.K8sPodMemUsageBytesMetric, conds, startTime, endTime, step)
	default:
		return nil, fmt.Errorf("not supported metric name %v for qcloud monitor", metric.MetricName)
	}
}

func (qm *qcloudmonitor) nodeMetric(ctx context.Context, metric *metricquery.Metric, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	selector := metric.Node.Selector
	if selector == nil {
		return nil, fmt.Errorf("selector is null, require label %v", known.LabelClusterId)
	}
	id, exists := selector.RequiresExactMatch(known.LabelClusterId)
	if !exists {
		return nil, fmt.Errorf("require label %v", known.LabelClusterId)
	}
	nodeRole := ""
	role, exists := selector.RequiresExactMatch(known.LabelNodeRole)
	if exists {
		nodeRole = role
	}

	conds := MakeNodeMetricConditions(id, metric.Node.Name, nodeRole)

	if err := checkQueryNodeCondition(conds); err != nil {
		return []*common.TimeSeries{}, err
	}

	// for barad node metric
	startTime, endTime, step = NodePeriodConvert(metric.MetricName, startTime, endTime, step)

	switch strings.ToLower(metric.MetricName) {
	case known.MetricCpuUsagePercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeCpuUsagePercentMetric, conds, startTime, endTime, step)
	case known.MetricMemUsagePercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeMemUsagePercentMetric, conds, startTime, endTime, step)
	case known.MetricCpuUsageAvg5mPercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeCpuUsageAvg5mPercentMetric, conds, startTime, endTime, step)
	case known.MetricMemUsageAvg5mPercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeMemUsageAvg5mPercentMetric, conds, startTime, endTime, step)
	case known.MetricCpuUsageMaxAvg1hPercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeCpuUsageMaxAvg1hPercentMetric, conds, startTime, endTime, step)
	case known.MetricMemUsageMaxAvg1hPercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeMemUsageMaxAvg1hPercentMetric, conds, startTime, endTime, step)
	case known.MetricCpuUsageMaxAvg1dPercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeCpuUsageMaxAvg1dPercentMetric, conds, startTime, endTime, step)
	case known.MetricMemUsageMaxAvg1dPercent:
		return qm.getMonitorData(ctx, qmonitor.K8sNodeMemUsageMaxAvg1dPercentMetric, conds, startTime, endTime, step)
	default:
		return nil, fmt.Errorf("not supported metric name %v for qcloud monitor", metric.MetricName)
	}
}

// This is a sb way for doggy barad
func NodePeriodConvert(metricName string, startTime, endTime time.Time, step time.Duration) (time.Time, time.Time, time.Duration) {
	switch metricName {
	case known.MetricCpuUsageMaxAvg1dPercent:
		fallthrough
	case known.MetricMemUsageMaxAvg1dPercent:
		step = 86400 * time.Second
		start := endTime.Add(-2 * step)
		return start, endTime, step
	case known.MetricCpuUsageMaxAvg1hPercent:
		fallthrough
	case known.MetricMemUsageMaxAvg1hPercent:
		step = 3600 * time.Second
		start := endTime.Add(-2 * step)
		return start, endTime, step
	case known.MetricCpuUsageAvg5mPercent:
		fallthrough
	case known.MetricMemUsageAvg5mPercent:
		step = 300 * time.Second
		start := endTime.Add(-2 * step)
		return start, endTime, step
	default:
		return startTime, endTime, step
	}
}

func MakeWorkloadMetricConditions(clusterid, namespace, workloadname, workloadkind string) []common.QueryCondition {
	conditions := []common.QueryCondition{
		{
			Key:      qmonitor.LabelClusterId,
			Operator: common.OperatorEqual,
			Value:    []string{clusterid},
		},
		{
			Key:      qmonitor.LabelNamespace,
			Operator: common.OperatorEqual,
			Value:    []string{namespace},
		},
		{
			Key:      qmonitor.LabelWorkloadName,
			Operator: common.OperatorEqual,
			Value:    []string{workloadname},
		},
	}
	if workloadkind != "" {
		conditions = append(conditions, common.QueryCondition{
			Key:      qmonitor.LabelWorkloadKind,
			Operator: common.OperatorEqual,
			Value:    []string{workloadkind},
		})
	}
	return conditions
}

func MakePodMetricConditions(clusterid, namespace, podname, workloadname, nodename string) []common.QueryCondition {
	conditions := []common.QueryCondition{
		{
			Key:      qmonitor.LabelNamespace,
			Operator: common.OperatorEqual,
			Value:    []string{namespace},
		},
		{
			Key:      qmonitor.LabelPodName,
			Operator: common.OperatorEqual,
			Value:    []string{podname},
		},
		{
			Key:      qmonitor.LabelClusterId,
			Operator: common.OperatorEqual,
			Value:    []string{clusterid},
		},
	}
	if workloadname != "" {
		conditions = append(conditions, common.QueryCondition{
			Key:      qmonitor.LabelWorkloadName,
			Operator: common.OperatorEqual,
			Value:    []string{workloadname},
		})
	}
	if nodename != "" {
		conditions = append(conditions, common.QueryCondition{
			Key:      qmonitor.LabelNode,
			Operator: common.OperatorEqual,
			Value:    []string{nodename},
		})
	}
	return conditions
}

// must specify clusterid, namespace, workloadname, containername
func MakeContainerMetricConditions(clusterid, namespace, workloadname, containername, containerid string) []common.QueryCondition {
	conditions := []common.QueryCondition{
		{
			Key:      qmonitor.LabelNamespace,
			Operator: common.OperatorEqual,
			Value:    []string{namespace},
		},
		{
			Key:      qmonitor.LabelWorkloadName,
			Operator: common.OperatorEqual,
			Value:    []string{workloadname},
		},
		{
			Key:      qmonitor.LabelClusterId,
			Operator: common.OperatorEqual,
			Value:    []string{clusterid},
		},
		{
			Key:      qmonitor.LabelContainerName,
			Operator: common.OperatorEqual,
			Value:    []string{containername},
		},
	}
	if containerid != "" {
		conditions = append(conditions, common.QueryCondition{
			Key:      qmonitor.LabelContainerId,
			Operator: common.OperatorEqual,
			Value:    []string{containerid},
		})

	}
	return conditions
}

// must specify clusterid, nodename, noderole
func MakeNodeMetricConditions(clusterid, nodename, noderole string) []common.QueryCondition {
	conditions := []common.QueryCondition{
		{
			Key:      qmonitor.LabelClusterId,
			Operator: common.OperatorEqual,
			Value:    []string{clusterid},
		},
	}

	if noderole != "" {
		conditions = append(conditions, common.QueryCondition{
			Key:      qmonitor.LabelNodeRole,
			Operator: common.OperatorEqual,
			Value:    []string{noderole},
		})
	}

	if nodename != "" {
		conditions = append(conditions, common.QueryCondition{
			Key:      qmonitor.LabelNode,
			Operator: common.OperatorEqual,
			Value:    []string{nodename},
		})
	}
	return conditions
}

func (qm *qcloudmonitor) getMonitorData(ctx context.Context, metricName string, conditions []common.QueryCondition, startTime time.Time, endTime time.Time, step time.Duration) ([]*common.TimeSeries, error) {
	startStr := startTime.Format(time.RFC3339)
	endStr := endTime.Format(time.RFC3339)
	period := uint64(step.Seconds())
	req := &qmonitor.GetDataParam{
		Module:      qmonitor.MetricsModule,
		Namespace:   qmonitor.MetricsNamespace,
		MetricNames: []string{metricName},
		StartTime:   startStr,
		EndTime:     endStr,
		Period:      period,
	}
	for _, cond := range conditions {
		req.AppendCondition(cond.Key, string(cond.Operator), cond.Value)
	}

	result, err := qm.cmClient.DescribeStatisticData(ctx, req)
	if err != nil {
		return []*common.TimeSeries{}, err
	}
	return MonitorData2CommonTimeSeries(metricName, result), nil
}

func MonitorData2CommonTimeSeries(metric string, result *qmonitor.GetDataResult) []*common.TimeSeries {
	datas := []*common.TimeSeries{}
	if len(result.Data) > 0 {
		for _, point := range result.Data[0].Points {
			data := common.NewTimeSeries()
			data.SetLabels(ConvertLabels(point.Dimensions))
			data.SetSamples(Points2Samples(metric, point.Values))
			if len(data.Samples) == 0 {
				// NOTE !!! some time the value is null although has timestamp, if all values is null, we ignore the series
				// "Values":[{"Timestamp":1654531200},{"Timestamp":1654617600,"Value":20},{"Timestamp":1654704000}]
				continue
			}
			datas = append(datas, data)
		}
	}
	return datas
}

func ConvertLabels(dimensions []qmonitor.Dimension) []common.Label {
	results := make([]common.Label, 0)
	for _, dim := range dimensions {
		results = append(results, common.Label{Name: dim.Name, Value: dim.Value})
	}
	return results
}

func Points2Samples(metric string, points []qmonitor.Point) []common.Sample {
	results := make([]common.Sample, 0)
	memoryMetricsSet := sets.NewString(qmonitor.K8sPodMemNoCacheBytesMetric, qmonitor.K8sPodMemUsageBytesMetric,
		qmonitor.K8sContainerMemNoCacheBytesMetric, qmonitor.K8sContainerMemUsageBytesMetric, qmonitor.K8sWorkloadCpuCoreUsedMetric)
	for _, point := range points {
		if point.Timestamp != nil && point.Value != nil {
			// NOTE: barad ram unit is MByte, convert it to Bytes
			val := *point.Value
			if memoryMetricsSet.Has(metric) {
				val = val * 1024.
			}
			results = append(results, common.Sample{Timestamp: int64(*point.Timestamp), Value: val})
		}
	}
	return results
}

// There are some required dimensions for query, please see https://cloud.tencent.com/document/product/248/53821
// and it has some query performance issue for too many series, so make sure conditions can query one series as much as possible, this is not following the doc completely.
func checkQueryPodCondition(conditions []common.QueryCondition) error {
	requiredLabels := sets.NewString(qmonitor.LabelPodName, qmonitor.LabelClusterId)

	condkeys := []string{}
	for _, v := range conditions {
		condkeys = append(condkeys, v.Key)
	}
	condSets := sets.NewString(condkeys...)
	if !condSets.Has(qmonitor.LabelNode) && !condSets.Has(qmonitor.LabelWorkloadName) {
		return fmt.Errorf("must include labels %+v, %+v", qmonitor.LabelNode, qmonitor.LabelWorkloadName)
	}
	if !condSets.HasAll(qmonitor.LabelPodName, qmonitor.LabelClusterId) {
		return fmt.Errorf("must include labels %+v", requiredLabels.List())
	}

	return nil
}

// There are some required dimensions for query, please see https://cloud.tencent.com/document/product/248/53821
// and it has some query performance issue for too many series, so make sure conditions can query one series as much as possible,  this is not following the doc completely.
func checkQueryContainerCondition(conditions []common.QueryCondition) error {
	requiredLabels := sets.NewString(qmonitor.LabelWorkloadName, qmonitor.LabelContainerName, qmonitor.LabelClusterId)

	condkeys := []string{}
	for _, v := range conditions {
		condkeys = append(condkeys, v.Key)
	}
	condSets := sets.NewString(condkeys...)
	if !condSets.HasAll(requiredLabels.List()...) {
		return fmt.Errorf("must include labels %+v", requiredLabels.List())
	}

	return nil
}

// There are some required dimensions for query, please see https://cloud.tencent.com/document/product/248/53821
// and it has some query performance issue for too many series, so make sure conditions can query one series as much as possible,  this is not following the doc completely.
func checkQueryWorkloadCondition(conditions []common.QueryCondition) error {
	requiredLabels := sets.NewString(qmonitor.LabelWorkloadName, qmonitor.LabelNamespace, qmonitor.LabelClusterId)

	condkeys := []string{}
	for _, v := range conditions {
		condkeys = append(condkeys, v.Key)
	}
	condSets := sets.NewString(condkeys...)
	if !condSets.HasAll(requiredLabels.List()...) {
		return fmt.Errorf("must include labels %+v", requiredLabels.List())
	}

	return nil
}

func checkQueryNodeCondition(conditions []common.QueryCondition) error {
	requiredLabels := sets.NewString(qmonitor.LabelClusterId)

	condkeys := []string{}
	for _, v := range conditions {
		condkeys = append(condkeys, v.Key)
	}
	condSets := sets.NewString(condkeys...)
	if !condSets.HasAll(requiredLabels.List()...) {
		return fmt.Errorf("must include labels %+v", requiredLabels.List())
	}

	return nil
}
