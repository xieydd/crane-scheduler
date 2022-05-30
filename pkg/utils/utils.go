package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"

	"github.com/gocrane/crane-scheduler/pkg/known"
)

const (
	TimeFormat      = "2006-01-02T15:04:05Z"
	DefaultTimeZone = "Asia/Shanghai"
)

func GetLocalTime() string {
	loc := GetLocation()
	if loc == nil {
		time.Now().Format(TimeFormat)
	}

	return time.Now().In(loc).Format(TimeFormat)
}

func GetLocation() *time.Location {
	zone := os.Getenv("TZ")

	if zone == "" {
		zone = DefaultTimeZone
	}

	loc, _ := time.LoadLocation(zone)

	return loc
}

func IsCraneExpansionPrefix(key string) bool {
	return strings.HasPrefix(key, schedulingapi.AnnotationPrefixSchedulingExpansion)
}

func IsCraneLoadBalanceTargetPrefix(key string) bool {
	return strings.HasPrefix(key, schedulingapi.AnnotationPrefixSchedulingBalanceTarget)
}

func IsCraneLoadBalanceLoadPrefix(key string) bool {
	return strings.HasPrefix(key, schedulingapi.AnnotationPrefixSchedulingBalanceLoad)
}

func BuildCraneAnnotation(prefix, name string) string {
	return strings.Join([]string{prefix, name}, "/")
}

func BuildCranePatchAnnotationKey(prefix, name string) string {
	return strings.Join([]string{prefix, name}, "~1")
}

func HasCraneLoadBalanceTargetAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	for k := range annotations {
		if IsCraneLoadBalanceTargetPrefix(k) {
			return true
		}
	}
	return false
}

func GetCraneAnnotations(annotations map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range annotations {
		if IsCraneExpansionPrefix(k) {
			result[k] = v
		} else if IsCraneLoadBalanceTargetPrefix(k) {
			result[k] = v
		} else if IsCraneLoadBalanceLoadPrefix(k) {
			result[k] = v
		}
	}
	return result
}

func GetDswCraneAnnotations(annotations map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range annotations {
		if IsCraneExpansionPrefix(k) {
			result[k] = v
		} else if IsCraneLoadBalanceTargetPrefix(k) {
			result[k] = v
		}
	}
	return result
}

type PatchItem struct {
	Op    string `json:"op,omitempty"`
	Path  string `json:"path,omitempty"`
	Value string `json:"value,omitempty"`
}

func BuildPatchBytes(aswAnns, dswAnns map[string]string) ([]byte, error) {
	var patchItems []PatchItem
	for dswKey, dswVal := range dswAnns {
		splits := strings.Split(dswKey, "/")
		// see https://github.com/kubernetes-sigs/kustomize/issues/2368
		// https://jsonpatch.com/#json-pointer
		patchPathKey := strings.Join(splits, "~1")
		aswVal, ok := aswAnns[dswKey]
		if !ok {
			patchItems = append(patchItems, PatchItem{
				Op:    "add",
				Path:  fmt.Sprintf("/metadata/annotations/%s", patchPathKey),
				Value: dswVal,
			})
			continue
		}
		if aswVal != dswVal {
			patchItems = append(patchItems, PatchItem{
				Op:    "replace",
				Path:  fmt.Sprintf("/metadata/annotations/%s", patchPathKey),
				Value: dswVal,
			})
			continue
		}
	}
	for aswKey := range aswAnns {
		dswVal, ok := dswAnns[aswKey]
		if !ok {
			splits := strings.Split(aswKey, "/")
			// see https://github.com/kubernetes-sigs/kustomize/issues/2368
			// https://jsonpatch.com/#json-pointer
			patchPathKey := strings.Join(splits, "~1")

			patchItems = append(patchItems, PatchItem{
				Op:    "remove",
				Path:  fmt.Sprintf("/metadata/annotations/%s", patchPathKey),
				Value: dswVal,
			})
		}
	}
	return json.Marshal(patchItems)
}

func GetSchedulerNamespaceApplyScope(config v1.ConfigMap) map[string]bool {
	namespaces := make(map[string]bool)
	namespaceScope := make(map[string]interface{})
	clusterScope := false
	for k, v := range config.Data {
		switch k {
		case known.ConfigMapSchedulerApplyScopeKeyClusterScope:
			Bv, err := strconv.ParseBool(v)
			if err != nil {
				clusterScope = false
				klog.Errorf("Failed to parse configmap kv[%s: %s], use default false: %v", k, v, err)
			} else {
				clusterScope = Bv
			}
		case known.ConfigMapSchedulerApplyScopeKeyNamespaceScope:
			err := json.Unmarshal([]byte(v), &namespaceScope)
			if err != nil {
				klog.Errorf("Failed to parse configmap kv[%s: %s], use default false: %v", k, v, err)
				return namespaces
			}
		}
	}

	if clusterScope {
		namespaces[known.WildCard] = true
	}
	for k, v := range namespaceScope {
		val := false
		if bv, ok := v.(bool); ok {
			val = bv
		} else if strV, ok := v.(string); ok {
			parsedVal, err := strconv.ParseBool(strV)
			if err != nil {
				klog.Errorf("Failed to parse value bool for kv[%v:%v] in configmap:", k, v, val)
				continue
			}
			val = parsedVal
		} else {
			klog.Errorf("Failed to parse value bool for kv[%v:%v] in configmap:", k, v, val)
			continue
		}
		namespaces[k] = val
	}
	return namespaces

}
