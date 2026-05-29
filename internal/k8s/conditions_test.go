package k8s

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractConditions_Pod(t *testing.T) {
	now := time.Now().Add(-30 * time.Minute)
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable",
					Message:            "0/3 nodes are available: 3 Insufficient cpu",
					LastTransitionTime: metav1.NewTime(now)},
				{Type: corev1.PodReady, Status: corev1.ConditionFalse,
					LastTransitionTime: metav1.NewTime(now)},
			},
		},
	}

	got := ExtractConditions(ResourceItem{Raw: pod})
	if len(got) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(got))
	}
	if got[0].Type != "PodScheduled" || got[0].Status != "False" || got[0].Reason != "Unschedulable" {
		t.Errorf("PodScheduled mapping wrong: %+v", got[0])
	}
	if got[0].Message != "0/3 nodes are available: 3 Insufficient cpu" {
		t.Errorf("message lost: %q", got[0].Message)
	}
	if got[0].Age == "" {
		t.Errorf("expected non-empty Age for non-zero LastTransitionTime")
	}
}

func TestExtractConditions_Deployment(t *testing.T) {
	dep := &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse,
					Reason: "MinimumReplicasUnavailable", Message: "Deployment does not have minimum availability."},
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue,
					Reason: "ReplicaSetUpdated"},
			},
		},
	}

	got := ExtractConditions(ResourceItem{Raw: dep})
	if len(got) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(got))
	}
	if got[0].Type != "Available" || got[0].Status != "False" {
		t.Errorf("Available mapping wrong: %+v", got[0])
	}
	if got[1].Type != "Progressing" || got[1].Status != "True" {
		t.Errorf("Progressing mapping wrong: %+v", got[1])
	}
}

func TestExtractConditions_UnsupportedKindReturnsNil(t *testing.T) {
	cm := &corev1.ConfigMap{}
	got := ExtractConditions(ResourceItem{Raw: cm})
	if got != nil {
		t.Errorf("expected nil for ConfigMap (no conditions field), got %+v", got)
	}
}

func TestExtractConditions_ZeroTimeAgeEmpty(t *testing.T) {
	pod := &corev1.Pod{
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	got := ExtractConditions(ResourceItem{Raw: pod})
	if len(got) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(got))
	}
	if got[0].Age != "" {
		t.Errorf("expected empty Age for zero LastTransitionTime, got %q", got[0].Age)
	}
}
