package utils

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"
)

func TestBuildPatchBytes(t *testing.T) {
	testCases := []struct {
		desc     string
		inputAsw map[string]string
		inputDsw map[string]string
		want     string
		wantErr  error
	}{
		{
			desc: "tc1. equals",
			inputAsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceCPU.String()):    "2",
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			inputDsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceCPU.String()):    "2",
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			want: "null",
		},
		{
			desc: "tc2. dsw > asw",
			inputAsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			inputDsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceCPU.String()):    "2",
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			want: `[{"op":"add","path":"/metadata/annotations/expansion.scheduling.crane.io~1cpu","value":"2"}]`,
		},
		{
			desc: "tc3. dsw < asw",
			inputAsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceCPU.String()):    "2",
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			inputDsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			want: `[{"op":"remove","path":"/metadata/annotations/expansion.scheduling.crane.io~1cpu"}]`,
		},
		{
			desc: "tc4. dsw != asw",
			inputAsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceCPU.String()):    "3",
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.1",
			},
			inputDsw: map[string]string{
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceCPU.String()):    "2",
				BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, v1.ResourceMemory.String()): "1.2",
			},
			want: `[{"op":"replace","path":"/metadata/annotations/expansion.scheduling.crane.io~1cpu","value":"2"},{"op":"replace","path":"/metadata/annotations/expansion.scheduling.crane.io~1memory","value":"1.2"}]`,
		},
	}

	for _, tc := range testCases {
		gotPatch, gotErr := BuildPatchBytes(tc.inputAsw, tc.inputDsw)
		//t.Logf("tc %v, gotPatch: %v, gotErr: %v", tc.desc, string(gotPatch), gotErr)
		if !equality.Semantic.DeepEqual(gotErr, tc.wantErr) {
			t.Fatalf("tc %v failed, gotErr %v, wantErr %v", tc.desc, gotErr, tc.wantErr)
		}
		if !equality.Semantic.DeepEqual(string(gotPatch), tc.want) {
			t.Fatalf("tc %v failed, got %v, want %v", tc.desc, gotPatch, tc.want)
		}
	}
}
