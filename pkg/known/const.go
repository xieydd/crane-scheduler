package known

import "time"

const (
	CraneNamespace = "crane-system"

	// DefaultLeaseDuration is the default LeaseDuration for leader election.
	DefaultLeaseDuration = 15 * time.Second
	// DefaultRenewDeadline is the default RenewDeadline for leader election.
	DefaultRenewDeadline = 10 * time.Second
	// DefaultRetryPeriod is the default RetryPeriod for leader election.
	DefaultRetryPeriod = 5 * time.Second
)

const (
	GB = 1024 * 1024 * 1024
)

//Tags/Dimensions/Labels
// this is an abstract inter-mediate labels name, different data source has different label naming which point to the same meaning
const (
	LabelAppId         = "appid"
	LabelContainerId   = "container_id"
	LabelContainerName = "container_name"
	LabelNamespace     = "namespace"
	LabelNode          = "node"
	LabelNodeRole      = "node_role"
	LabelNodeIp        = "node_ip"
	LabelPodName       = "pod_name"
	LabelRegion        = "region"
	LabelClusterId     = "cluster_id"
	LabelUnInstanceId  = "un_instance_id"
	LabelWorkloadKind  = "workload_kind"
	LabelWorkloadName  = "workload_name"
	LabelInstanceType  = "instance_type"
	LabelInstance      = "instance"
	LabelProviderId    = "provider_id"
)

const (
	MetricCpuRequest = "cpu_request"
	MetricCpuLimit   = "cpu_limit"
	MetricMemRequest = "mem_request"
	MetricMemLimit   = "mem_limit"
	// todo: more node metric
	MetricCpuUsagePercent = "cpu_usage_percent"
	MetricMemUsagePercent = "mem_usage_percent"

	MetricCpuUsageAvg5mPercent    = "cpu_usage_avg_5m"
	MetricCpuUsageMaxAvg1hPercent = "cpu_usage_max_avg_1h"
	MetricCpuUsageMaxAvg1dPercent = "cpu_usage_max_avg_1d"
	MetricMemUsageAvg5mPercent    = "mem_usage_avg_5m"
	MetricMemUsageMaxAvg1hPercent = "mem_usage_max_avg_1h"
	MetricMemUsageMaxAvg1dPercent = "mem_usage_max_avg_1d"

	MetricWorkloadReplicas = "replicas"
)
