package safeoverload

import (
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	schedulerextapi "k8s.io/kube-scheduler/extender/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/gocrane/crane-scheduler/pkg/algorithms"
	"github.com/gocrane/crane-scheduler/pkg/frameworkerrors"
	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

func PredictateFunc(pod corev1.Pod, nodes []corev1.Node, policySpec policy.PolicySpec) (*schedulerextapi.ExtenderFilterResult, error) {
	labels := map[string]string{
		"predicate_name": known.PredicateSafeOverloadName,
		"pod":            pod.Name,
		"namespace":      pod.Namespace,
	}
	start := time.Now()
	defer func() {
		metrics.ExtenderPredicateLatency.With(labels).Observe(time.Since(start).Seconds())
	}()

	canSchedule := make([]corev1.Node, 0, len(nodes))
	canNotSchedule := make(map[string]string)

	// todo: parallel
	for _, node := range nodes {
		result, status := predicate(pod, node, policySpec)
		if !status.IsSuccess() {
			canNotSchedule[node.Name] = status.AsError().Error()
		} else {
			if result {
				canSchedule = append(canSchedule, node)
			}
		}
	}

	result := &schedulerextapi.ExtenderFilterResult{
		Nodes: &corev1.NodeList{
			Items: canSchedule,
		},
		FailedNodes: canNotSchedule,
		Error:       "",
	}

	if klog.V(6).Enabled() {
		verbose, err := json.Marshal(result)
		klog.V(6).Infof("==> PredicateFunc: applying safe-overload, calculating priority for pod %s, filterResult: %s, err: %v",
			klog.KObj(&pod), string(verbose), err)
	}

	return result, nil

}

// PredicateFunc : safe predicate function
func predicate(pod corev1.Pod, node corev1.Node, policySpec policy.PolicySpec) (bool, *framework.Status) {
	labels := map[string]string{
		"predicate_name": known.PredicateSafeOverloadName,
		"pod":            pod.Name,
		"namespace":      pod.Namespace,
		"node":           node.Name,
	}
	start := time.Now()
	defer func() {
		metrics.ExtenderPredicateNodeLatency.With(labels).Observe(time.Since(start).Seconds())
	}()

	// for non-housekeeper-scoped pods, normal nodes pass directly but for housekeeper node we do load balance so housekeeper has some more capability
	// this is an product policy for housekeeper migration and sell
	if !utils.IsHouseKeeperScopePod(&pod) && !utils.IsHouseKeeperNode(&node) {
		return true, framework.NewStatus(framework.Success)
	}

	if utils.IsDaemonsetPod(&pod) {
		return true, framework.NewStatus(framework.Success)
	}

	anno := node.ObjectMeta.Annotations
	if anno == nil {
		anno = map[string]string{}
	}

	if algorithms.IsOverLoad(&pod, &node, anno, policySpec) {
		return false, framework.NewStatus(framework.Unschedulable, frameworkerrors.ErrReasonOverloadThresholdExceeded)
	}

	return true, framework.NewStatus(framework.Success)
}
