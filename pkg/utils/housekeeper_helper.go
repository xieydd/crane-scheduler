package utils

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/gocrane/crane-scheduler/pkg/known"
)

func IsHouseKeeperScopePod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	ann := pod.GetAnnotations()
	if ann == nil {
		return false
	}
	hkVal, ok := ann[known.AnnotationPodSchedulingScope]
	if !ok {
		return false
	}
	if hkVal == known.AnnotationPodSchedulingScopeHousekeeperVal {
		return true
	}
	return false
}

func IsHouseKeeperNode(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	labels := node.GetLabels()
	if labels == nil {
		return false
	}
	housekeeperVal, ok := labels[known.LabelHousekeeperNodeKey]
	if !ok {
		return false
	}
	if housekeeperVal == known.LabelHousekeeperNodeVal {
		return true
	}
	return false
}

func NodeHaveSpecificLabel(node *corev1.Node, key, value string) bool {
	if node == nil {
		return false
	}
	labels := node.GetLabels()
	if labels == nil {
		return false
	}
	val, ok := labels[key]
	if !ok {
		return false
	}
	if val == value {
		return true
	}
	return false
}
