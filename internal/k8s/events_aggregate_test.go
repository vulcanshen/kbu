package k8s

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestFetchResourceEventsAggregated_DeploymentMergesPodEvents verifies that
// selecting a Deployment surfaces events from the Deployment itself AND its
// child Pods (the killer case — Deployment events alone are sparse, the
// debug-useful events live on Pods).
func TestFetchResourceEventsAggregated_DeploymentMergesPodEvents(t *testing.T) {
	now := time.Now()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web",
			Namespace:   "ns",
			Annotations: map[string]string{"deployment.kubernetes.io/revision": "1"},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web-rs1",
			Namespace:   "ns",
			Annotations: map[string]string{"deployment.kubernetes.io/revision": "1"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: "web", UID: dep.UID},
			},
			Labels: map[string]string{"app": "web"},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-pod-abc",
			Namespace: "ns",
			Labels:    map[string]string{"app": "web"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1"}},
		},
	}

	depEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "dep-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Deployment", Name: "web", Namespace: "ns"},
		Type:           "Normal",
		Reason:         "ScalingReplicaSet",
		Message:        "Scaled up replica set web-rs1 to 1",
		LastTimestamp:  metav1.NewTime(now.Add(-1 * time.Hour)),
	}
	podEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pod-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "web-pod-abc", Namespace: "ns"},
		Type:           "Warning",
		Reason:         "BackOff",
		Message:        "Back-off restarting failed container",
		LastTimestamp:  metav1.NewTime(now.Add(-5 * time.Minute)),
	}

	cs := fake.NewSimpleClientset(dep, rs, pod, depEvent, podEvent)

	item := ResourceItem{Name: "web", Namespace: "ns", Raw: dep}
	events, err := FetchResourceEventsAggregated(context.Background(), cs, item)
	if err != nil {
		t.Fatalf("FetchResourceEventsAggregated: %v", err)
	}

	// fake.NewSimpleClientset ignores FieldSelector (returns ALL events for
	// every list call), so we can't assert exact count — real K8s would
	// return parent+child events once each. We can still verify that BOTH
	// sources contributed (pod-level event + deployment-level event are
	// present) and that sorting put the newest first.
	if len(events) == 0 {
		t.Fatal("expected at least one merged event, got 0")
	}
	if events[0].Reason != "BackOff" {
		t.Errorf("expected newest event first (BackOff at 5m ago), got %q", events[0].Reason)
	}
	hasReason := func(reason string) bool {
		for _, e := range events {
			if e.Reason == reason {
				return true
			}
		}
		return false
	}
	if !hasReason("ScalingReplicaSet") {
		t.Errorf("missing Deployment's own event (ScalingReplicaSet) in merged result: %+v", events)
	}
	if !hasReason("BackOff") {
		t.Errorf("missing child Pod's event (BackOff) in merged result: %+v", events)
	}
	// Object column should distinguish source kinds.
	hasObject := func(obj string) bool {
		for _, e := range events {
			if e.Object == obj {
				return true
			}
		}
		return false
	}
	if !hasObject("Pod/web-pod-abc") {
		t.Errorf("expected Pod object in result, got %+v", events)
	}
	if !hasObject("Deployment/web") {
		t.Errorf("expected Deployment object in result, got %+v", events)
	}
}

// TestFetchResourceEventsAggregated_NonWorkloadFallsBack verifies that for
// kinds PodsForWorkload doesn't support, aggregation degrades to single-object
// behavior (parent events only, no error).
func TestFetchResourceEventsAggregated_NonWorkloadFallsBack(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "ns"}}
	cmEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "cm-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "ConfigMap", Name: "shared", Namespace: "ns"},
		Type:           "Normal",
		Reason:         "Updated",
		Message:        "shared updated",
		LastTimestamp:  metav1.NewTime(time.Now()),
	}
	cs := fake.NewSimpleClientset(cm, cmEvent)

	item := ResourceItem{Name: "shared", Namespace: "ns", Raw: cm}
	events, err := FetchResourceEventsAggregated(context.Background(), cs, item)
	if err != nil {
		t.Fatalf("expected no error for non-workload kind, got %v", err)
	}
	if len(events) != 1 || events[0].Reason != "Updated" {
		t.Errorf("expected 1 ConfigMap event, got %+v", events)
	}
}

// TestFetchResourceEventsAggregated_DeploymentWithNoPods returns parent events
// only and does not error when the deployment has no live pods.
func TestFetchResourceEventsAggregated_DeploymentWithNoPods(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ghost", Namespace: "ns"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "ghost"}},
		},
	}
	depEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ghost-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Deployment", Name: "ghost", Namespace: "ns"},
		Reason:         "FailedCreate",
		LastTimestamp:  metav1.NewTime(time.Now()),
	}
	cs := fake.NewSimpleClientset(dep, depEvent)

	item := ResourceItem{Name: "ghost", Namespace: "ns", Raw: dep}
	events, err := FetchResourceEventsAggregated(context.Background(), cs, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 || events[0].Reason != "FailedCreate" {
		t.Errorf("expected single deployment event, got %+v", events)
	}
}
