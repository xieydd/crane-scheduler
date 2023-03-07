package known

import "time"

const (
	// PredicateSafeExpansionFitName: name of safe expansion resource fit predicate function
	PredicateSafeExpansionFitName = "safe-expansion-fit"
	// PredicateSafeOverloadName : name of safe overload predicate function
	PredicateSafeOverloadName = "safe-overload"
	// PrioritySafeBalanceName : name of safe balance priority function
	PrioritySafeBalanceName = "safe-balance"
)

const (
	// MinTimestampStrLength defines the min length of timestamp string.
	MinTimestampStrLength = 5
	// NodeHotValueKey is the key of hot value annotation.
	NodeHotValueKey = "node_hot_value"
	// DefautlHotVauleActivePeriod defines the validity period of nodes' hotvalue.
	DefautlHotVauleActivePeriod = 5 * time.Minute
	// ExtraActivePeriod gives extra active time to the annotation.
	ExtraActivePeriod = 5 * time.Minute
)

const (
	ConfigMapSchedulerApplyScope                  = "crane-scheduler-apply-scope"
	ConfigMapSchedulerApplyScopeKeyClusterScope   = "clusterScope"
	ConfigMapSchedulerApplyScopeKeyNamespaceScope = "namespaceScope"
	WildCard                                      = "*"

	AnnotationPodSchedulingScope               = "scope.scheduling.crane.io"
	AnnotationPodSchedulingScopeHousekeeperVal = "housekeeper"

	// nodes with following label key/value is housekeeper managed node pool nodes.
	// checked with @jerryachen
	LabelHousekeeperNodeKey = "cloud.tencent.com/provider"
	LabelHousekeeperNodeVal = "tencentcloud"

	LabelDynamicSchedulerNodeKey = "enabled.dynamicscheduler.crane.io"
	LabelDynamicSchedulerNodeVal = "true"

	LabelAnnotateNodeKey = "enabled.annotatenode.crane.io"
	LabelAnnotateNodeVal = "true"
)

const (
	DefaultBackOff = 10 * time.Second
	MaxBackOff     = 360 * time.Second
)
