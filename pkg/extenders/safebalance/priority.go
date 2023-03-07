package safebalance

import (
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	schedulerextapi "k8s.io/kube-scheduler/extender/v1"

	"github.com/gocrane/crane-scheduler/pkg/algorithms"
	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

// PriorityFunc : compute safe priority function
func PriorityFunc(pod corev1.Pod, nodes []corev1.Node, policySpec policy.PolicySpec) (*schedulerextapi.HostPriorityList, error) {
	labels := map[string]string{
		"priority_name": known.PrioritySafeBalanceName,
		"pod":           pod.Name,
		"namespace":     pod.Namespace,
	}
	start := time.Now()
	defer func() {
		metrics.ExtenderPriorityLatency.With(labels).Observe(time.Since(start).Seconds())
	}()

	var priorityList schedulerextapi.HostPriorityList
	priorityList = make([]schedulerextapi.HostPriority, len(nodes))

	for i, node := range nodes {
		anno := node.ObjectMeta.Annotations
		if anno == nil {
			anno = map[string]string{}
		}

		score := algorithms.GetNodeScoreWithHotSpotPenalty(&pod, &node, anno, policySpec)

		priorityList[i] = schedulerextapi.HostPriority{
			Host:  node.Name,
			Score: score,
		}

		// for non-housekeeper-scoped pods, normal nodes score zero directly but for housekeeper or metacluster node we do load balance score so housekeeper has some more capability
		// this is an product policy for housekeeper migration and sell
		if !utils.IsHouseKeeperScopePod(&pod) && (!utils.IsHouseKeeperNode(&node) ||
			!utils.NodeHaveSpecificLabel(&node, known.LabelDynamicSchedulerNodeKey, known.LabelDynamicSchedulerNodeVal)) {
			priorityList[i].Score = 0
		}

	}
	if klog.V(6).Enabled() {
		verbose, err := json.Marshal(priorityList)
		klog.V(6).Infof("==> PriorityFunc: applying safe-balance, calculating priority for pod %s, priorityList: %s, err: %v",
			klog.KObj(&pod), string(verbose), err)
	}

	return &priorityList, nil
}
