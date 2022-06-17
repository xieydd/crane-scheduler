package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
)

// inActivePeriod judges if node annotation with this timestamp is effective.
func InActivePeriod(updatetimeStr string, activeDuration time.Duration) bool {
	if len(updatetimeStr) < known.MinTimestampStrLength {
		klog.Errorf("[crane] illegel timestamp: %s", updatetimeStr)
		return false
	}

	originUpdateTime, err := time.ParseInLocation(TimeFormat, updatetimeStr, GetLocation())
	if err != nil {
		klog.Errorf("[crane] failed to parse timestamp: %v", err)
		return false
	}

	now, updatetime := time.Now(), originUpdateTime.Add(activeDuration)

	if now.Before(updatetime) {
		return true
	}

	return false
}

func GetResourceUsage(anno map[string]string, key string, activeDuration time.Duration) (float64, error) {

	if anno == nil {
		return 0, fmt.Errorf("annotation is null")
	}
	annkey := BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingBalanceLoad, key)

	loadStr, ok := anno[annkey]
	if !ok {
		return 0, fmt.Errorf("annotation key %v not found", annkey)
	}

	usedSlice := strings.Split(loadStr, ",")
	if len(usedSlice) != 2 {
		return 0, fmt.Errorf("illegel value: %s", loadStr)
	}

	if !InActivePeriod(usedSlice[1], activeDuration) {
		return 0, fmt.Errorf("timestamp[%s] is expired", loadStr)
	}

	UsedValue, err := strconv.ParseFloat(usedSlice[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse float[%s]", usedSlice[0])
	}

	if UsedValue < 0 {
		return 0, fmt.Errorf("illegel value: %s", loadStr)
	}

	return UsedValue / 100., nil
}

func GetNodeHotValue(anno map[string]string, activeDuration time.Duration) (float64, error) {

	if anno == nil {
		return 0, fmt.Errorf("annotation is null")
	}
	annkey := BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingBalanceLoad, known.NodeHotValueKey)

	loadStr, ok := anno[annkey]
	if !ok {
		return 0, fmt.Errorf("annotation key %v not found", annkey)
	}

	usedSlice := strings.Split(loadStr, ",")
	if len(usedSlice) != 2 {
		return 0, fmt.Errorf("illegel value: %s", loadStr)
	}

	if !InActivePeriod(usedSlice[1], activeDuration) {
		return 0, fmt.Errorf("timestamp[%s] is expired", loadStr)
	}

	UsedValue, err := strconv.ParseFloat(usedSlice[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse float[%s]", usedSlice[0])
	}

	if UsedValue < 0 {
		return 0, fmt.Errorf("illegel value: %s", loadStr)
	}

	return UsedValue, nil
}

func GetResourceTargetThreshold(anno map[string]string, key string) (float64, error) {
	//todo: because of the threshold key is cpu or memory in crd now,
	// but the usage key is the policy name such as cpu_usage_avg_5m or cpu_usage_max_avg_1h

	annkey := BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingBalanceTarget, key)
	target, ok := anno[annkey]
	if !ok {
		cpuKey := v1.ResourceCPU.String()
		memKey := v1.ResourceMemory.String()
		if strings.Contains(key, cpuKey) {
			key = cpuKey
		} else if strings.Contains(key, "mem") {
			// load mem_usage_avg_xxx
			// but target is memory
			key = memKey
		}
		annkey := BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingBalanceTarget, key)
		target, ok = anno[annkey]
		if !ok {
			return 0, fmt.Errorf("annotation %v not found", annkey)
		}
	}
	targetPercent, err := strconv.ParseInt(target, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse int[%v]", target)
	}

	return float64(targetPercent) / 100, nil
}

func GetActiveDuration(syncPeriodList []policy.SyncPolicy, name string) (time.Duration, error) {
	for _, period := range syncPeriodList {
		if period.Name == name {
			if period.Period.Duration != 0 {
				return period.Period.Duration + known.ExtraActivePeriod, nil
			}
		}
	}

	return 0, fmt.Errorf("failed to get the active duration")
}

// isDaemonsetPod judges if this pod belongs to one daemonset workload.
func IsDaemonsetPod(pod *v1.Pod) bool {
	for _, ownerRef := range pod.GetOwnerReferences() {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}
