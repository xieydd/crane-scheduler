package algorithms

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

func GetMetricScore(node *corev1.Node, anno map[string]string, priorityPolicy policy.PriorityPolicy, syncPeriod []policy.SyncPolicy) (float64, error) {
	activeDuration, err := utils.GetActiveDuration(syncPeriod, priorityPolicy.Name)
	if err != nil || activeDuration == 0 {
		return 0, fmt.Errorf("failed to get the active duration of resource[%s] node[%s]: %v, while the actual value is %v", priorityPolicy.Name, node.Name, err, activeDuration)
	}

	usage, err := utils.GetResourceUsage(anno, priorityPolicy.Name, activeDuration)
	if err != nil {
		return 0, err
	}

	score := (1. - usage) * priorityPolicy.Weight * float64(framework.MaxNodeScore)

	return score, nil
}

// IsOverLoad return the pod is overload when the pod is assumed to be scheduled to this node. later split the thresholds to node annotation
// todo: now pod info is not valid, later maybe used.
func IsOverLoad(pod *corev1.Pod, node *corev1.Node, anno map[string]string, policySpec policy.PolicySpec) bool {
	// all usage must less than overload threshold.
	// if can not get the usage load or not exists in node annotation or there are no thresholds, we think it is not overload for safety
	thresholdExceedCounts := 0
	for _, predicatePolicy := range policySpec.Predicate {
		activeDuration, err := utils.GetActiveDuration(policySpec.SyncPeriod, predicatePolicy.Name)
		if err != nil || activeDuration == 0 {
			klog.Warningf("Predicate pod: %s, node: %s, getactiveDuration error %s", klog.KObj(pod), node.Name,
				predicatePolicy.Name)
			continue
		}

		usage, err := utils.GetResourceUsage(anno, predicatePolicy.Name, activeDuration)
		if err != nil {
			klog.Errorf("Failed to get the usage of pod[%s] metric[%s] from node[%s]'s annotation: %v", klog.KObj(pod), predicatePolicy.Name, node.Name, err)
			continue
		}

		targetThreshold, err := utils.GetResourceTargetThreshold(anno, predicatePolicy.Name)
		if err != nil {
			klog.Errorf("Failed to get the target threshold of pod[%s] metric[%s] from node[%s]'s annotation: %v", klog.KObj(pod), predicatePolicy.Name, node.Name, err)
			continue
		}

		// threshold lt 0 means that the filter according to this metric is useless.
		if targetThreshold < 0 {
			klog.V(6).Infof("Ignore the filter of pod[%s] metric[%s] from node[%s] for targetThreshold was set lt 0", klog.KObj(pod), predicatePolicy.Name, node.Name)
			continue
		}

		// and operation for all predicates thresholds, so as long as there has one usage overload, it is overload
		if usage > targetThreshold {
			klog.V(6).Infof("Predicate pod: %s, node: %s, out of %s, usage=%v,targetThreshold=%v", klog.KObj(pod), node.Name,
				predicatePolicy.Name, usage, targetThreshold)
			thresholdExceedCounts++
		}
	}

	n := len(policySpec.Predicate)
	if n > 0 && thresholdExceedCounts >= 0 {
		klog.V(6).Infof("Predicate pod: %s, node: %s is overload, thresholdExceedCounts: %v, len(policySpec.Predicate): %v", klog.KObj(pod), node.Name, thresholdExceedCounts, n)
		return true
	}

	return false
}

func GetNodeScore(pod *corev1.Pod, node *corev1.Node, anno map[string]string, policySpec policy.PolicySpec) int64 {

	lenPriorityPolicyList := len(policySpec.Priority)
	if lenPriorityPolicyList == 0 {
		klog.Warningf("No priority policy exists, all nodes scores 0.")
		return 0
	}

	var score, weight float64

	for _, priorityPolicy := range policySpec.Priority {

		priorityScore, err := GetMetricScore(node, anno, priorityPolicy, policySpec.SyncPeriod)
		if err != nil {
			klog.Errorf("Failed to get metric score, pod: %v, node: %v, metric: %v, score: %v", klog.KObj(pod), node.Name, priorityPolicy.Name, score)
		}

		weight += priorityPolicy.Weight
		score += priorityScore
	}

	finalScore := int64(score / weight)

	return finalScore
}

// GetNodeScoreWithHotSpotPenalty return score when the pod is assumed to be scheduled to this node.
// todo: now pod info is not valid, later maybe used.
func GetNodeScoreWithHotSpotPenalty(pod *corev1.Pod, node *corev1.Node, anno map[string]string, policySpec policy.PolicySpec) int64 {

	lenPriorityPolicyList := len(policySpec.Priority)
	if lenPriorityPolicyList == 0 {
		klog.Warningf("No priority policy exists, all nodes scores 0.")
		return 0
	}

	var score, weight float64

	// if the usage metric is expired or has error, the node do not compute score
	for _, priorityPolicy := range policySpec.Priority {
		priorityScore, err := GetMetricScore(node, anno, priorityPolicy, policySpec.SyncPeriod)
		if err != nil {
			klog.Errorf("Failed to get metric score, pod: %v, node: %v, metric: %v, score: %v, err: %v", klog.KObj(pod), node.Name, priorityPolicy.Name, score, err)
			return 0
		}
		weight += priorityPolicy.Weight
		score += priorityScore
	}

	weightedScore := int64(score / weight)
	hotValuePenalty := GetNodeHotValue(node)
	finalScore := weightedScore - int64(hotValuePenalty)
	if finalScore < 0 {
		finalScore = 0
	}
	klog.V(6).Infof("GetNodeScoreWithHotSpotPenalty, pod: %v, node: %v, metric weighted score: %v, hotValuePenalty: %v, finalScore: %v", klog.KObj(pod), node.Name, weightedScore, hotValuePenalty, finalScore)
	return finalScore
}

func GetNodeHotValue(node *corev1.Node) float64 {
	anno := node.ObjectMeta.Annotations
	if anno == nil {
		return 0
	}

	hotvalue, err := utils.GetNodeHotValue(anno, known.DefautlHotVauleActivePeriod)
	if err != nil {
		return 0
	}

	return hotvalue
}
