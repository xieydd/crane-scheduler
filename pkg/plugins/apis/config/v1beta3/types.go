package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DynamicArgs is the args struction of Dynamic scheduler plugin.
type DynamicArgs struct {
	metav1.TypeMeta `json:",inline"`
	// PolicyConfigPath specified the path of policy config.
	PolicyConfigPath *string `json:"policyConfigPath,omitempty"`
}
