package k8s

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func podWithLabels(name, ns string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
	}
}

func TestPodsForStatefulSet_ResolvesBySelector(t *testing.T) {
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns"},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
		},
	}
	p1 := podWithLabels("db-0", "ns", map[string]string{"app": "db"})
	p2 := podWithLabels("db-1", "ns", map[string]string{"app": "db"})
	other := podWithLabels("web-0", "ns", map[string]string{"app": "web"})

	cs := fake.NewSimpleClientset(ss, p1, p2, other)
	targets, err := PodsForStatefulSet(context.Background(), cs, ss)
	if err != nil {
		t.Fatalf("PodsForStatefulSet: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 matching pods (db-0, db-1), got %d: %+v", len(targets), targets)
	}
}

func TestPodsForDaemonSet_ResolvesBySelector(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "node-agent", Namespace: "ns"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "node-agent"}},
		},
	}
	p1 := podWithLabels("node-agent-abc", "ns", map[string]string{"app": "node-agent"})
	p2 := podWithLabels("node-agent-xyz", "ns", map[string]string{"app": "node-agent"})
	cs := fake.NewSimpleClientset(ds, p1, p2)

	targets, err := PodsForDaemonSet(context.Background(), cs, ds)
	if err != nil {
		t.Fatalf("PodsForDaemonSet: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 daemonset pods, got %d", len(targets))
	}
}

func TestPodsForReplicaSet_ResolvesBySelector(t *testing.T) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web-abc", Namespace: "ns"},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web", "pod-template-hash": "abc"}},
		},
	}
	pod := podWithLabels("web-abc-xyz", "ns", map[string]string{"app": "web", "pod-template-hash": "abc"})
	cs := fake.NewSimpleClientset(rs, pod)

	targets, err := PodsForReplicaSet(context.Background(), cs, rs)
	if err != nil {
		t.Fatalf("PodsForReplicaSet: %v", err)
	}
	if len(targets) != 1 || targets[0].Name != "web-abc-xyz" {
		t.Fatalf("expected web-abc-xyz, got %+v", targets)
	}
}

func TestPodsForJob_UsesJobNameLabel(t *testing.T) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "migrate", Namespace: "ns"},
	}
	pod := podWithLabels("migrate-xyz", "ns", map[string]string{"job-name": "migrate"})
	other := podWithLabels("other-pod", "ns", map[string]string{"app": "other"})
	cs := fake.NewSimpleClientset(job, pod, other)

	targets, err := PodsForJob(context.Background(), cs, job)
	if err != nil {
		t.Fatalf("PodsForJob: %v", err)
	}
	if len(targets) != 1 || targets[0].Name != "migrate-xyz" {
		t.Fatalf("expected migrate-xyz, got %+v", targets)
	}
}

func TestPodsForWorkload_DispatchesAllSupportedKinds(t *testing.T) {
	// Smoke test: every supported kind must be reachable via PodsForWorkload
	// (not return "not supported" error).
	cases := []struct {
		name string
		raw  interface{}
	}{
		{"Deployment", &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
			Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}},
		}},
		{"StatefulSet", &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
			Spec:       appsv1.StatefulSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}},
		}},
		{"DaemonSet", &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
			Spec:       appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}},
		}},
		{"ReplicaSet", &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
			Spec:       appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}},
		}},
		{"Job", &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"}}},
	}

	cs := fake.NewSimpleClientset()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := ResourceItem{Name: "x", Namespace: "ns", Raw: tc.raw}
			_, err := PodsForWorkload(context.Background(), cs, item, true)
			if err != nil {
				t.Errorf("%s: unexpected error %v", tc.name, err)
			}
		})
	}
}

func TestPodsForWorkload_UnsupportedKindStillErrors(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "ns"}}
	cs := fake.NewSimpleClientset(cm)
	_, err := PodsForWorkload(context.Background(), cs, ResourceItem{Name: "shared", Namespace: "ns", Raw: cm}, true)
	if err == nil {
		t.Error("expected error for unsupported kind (ConfigMap)")
	}
}

// TestFetchResourceEventsAggregated_StatefulSetMergesPodEvents covers the
// end-to-end path: now that PodsForWorkload knows StatefulSet, aggregate
// events should pick up child Pod events for this kind too.
func TestFetchResourceEventsAggregated_StatefulSetMergesPodEvents(t *testing.T) {
	now := time.Now()
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "ns"},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
		},
	}
	pod := podWithLabels("db-0", "ns", map[string]string{"app": "db"})

	ssEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ss-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "StatefulSet", Name: "db", Namespace: "ns"},
		Reason:         "SuccessfulCreate",
		LastTimestamp:  metav1.NewTime(now.Add(-2 * time.Hour)),
	}
	podEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pod-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "db-0", Namespace: "ns"},
		Reason:         "Pulled",
		LastTimestamp:  metav1.NewTime(now.Add(-3 * time.Minute)),
	}

	cs := fake.NewSimpleClientset(ss, pod, ssEvent, podEvent)
	events, err := FetchResourceEventsAggregated(context.Background(), cs, ResourceItem{Name: "db", Namespace: "ns", Raw: ss})
	if err != nil {
		t.Fatalf("FetchResourceEventsAggregated: %v", err)
	}
	hasReason := func(r string) bool {
		for _, e := range events {
			if e.Reason == r {
				return true
			}
		}
		return false
	}
	if !hasReason("SuccessfulCreate") {
		t.Errorf("missing StatefulSet's own event in merged result: %+v", events)
	}
	if !hasReason("Pulled") {
		t.Errorf("missing child Pod's event in merged result: %+v", events)
	}
}

// TestPodsForCronJob_WalksOwnedJobsToPods covers the 2-hop chain — CronJob
// owns multiple Jobs (history), each Job has Pods. PodsForCronJob should
// return all Pods across all owned Jobs.
func TestPodsForCronJob_WalksOwnedJobsToPods(t *testing.T) {
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "ns", UID: "cj-uid"},
	}
	jobA := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cleanup-1", Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "cleanup", UID: "cj-uid"}},
		},
	}
	jobB := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cleanup-2", Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "cleanup", UID: "cj-uid"}},
		},
	}
	jobUnrelated := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "other-job", Namespace: "ns"},
	}
	podA := podWithLabels("cleanup-1-xyz", "ns", map[string]string{"job-name": "cleanup-1"})
	podB := podWithLabels("cleanup-2-abc", "ns", map[string]string{"job-name": "cleanup-2"})
	podUnrelated := podWithLabels("other-job-pod", "ns", map[string]string{"job-name": "other-job"})

	cs := fake.NewSimpleClientset(cj, jobA, jobB, jobUnrelated, podA, podB, podUnrelated)

	targets, err := PodsForCronJob(context.Background(), cs, cj)
	if err != nil {
		t.Fatalf("PodsForCronJob: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 pods across 2 owned jobs, got %d: %+v", len(targets), targets)
	}
	names := map[string]bool{targets[0].Name: true, targets[1].Name: true}
	if !names["cleanup-1-xyz"] || !names["cleanup-2-abc"] {
		t.Errorf("expected cleanup-1-xyz + cleanup-2-abc, got %+v", targets)
	}
}

// TestFetchResourceEventsAggregated_CronJobMergesJobAndPodEvents covers the
// full 3-tier merge: CronJob's own events + each owned Job's events + each
// child Pod's events. CronJob's value is high here because Job-level events
// (BackoffLimitExceeded, Completed) carry distinct debug info from Pod-level
// events.
func TestFetchResourceEventsAggregated_CronJobMergesJobAndPodEvents(t *testing.T) {
	now := time.Now()
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "ns", UID: "cj-uid"},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cleanup-1", Namespace: "ns",
			OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "cleanup", UID: "cj-uid"}},
		},
	}
	pod := podWithLabels("cleanup-1-xyz", "ns", map[string]string{"job-name": "cleanup-1"})

	cjEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "cj-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "CronJob", Name: "cleanup", Namespace: "ns"},
		Reason:         "SuccessfulCreate",
		LastTimestamp:  metav1.NewTime(now.Add(-15 * time.Minute)),
	}
	jobEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "job-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Job", Name: "cleanup-1", Namespace: "ns"},
		Reason:         "Completed",
		LastTimestamp:  metav1.NewTime(now.Add(-10 * time.Minute)),
	}
	podEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pod-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "cleanup-1-xyz", Namespace: "ns"},
		Reason:         "Started",
		LastTimestamp:  metav1.NewTime(now.Add(-12 * time.Minute)),
	}

	cs := fake.NewSimpleClientset(cj, job, pod, cjEvent, jobEvent, podEvent)
	events, err := FetchResourceEventsAggregated(context.Background(), cs, ResourceItem{Name: "cleanup", Namespace: "ns", Raw: cj})
	if err != nil {
		t.Fatalf("FetchResourceEventsAggregated: %v", err)
	}

	hasReason := func(r string) bool {
		for _, e := range events {
			if e.Reason == r {
				return true
			}
		}
		return false
	}
	if !hasReason("SuccessfulCreate") {
		t.Errorf("missing CronJob's own event: %+v", events)
	}
	if !hasReason("Completed") {
		t.Errorf("missing intermediate Job event (the killer feature): %+v", events)
	}
	if !hasReason("Started") {
		t.Errorf("missing child Pod event: %+v", events)
	}
}

// TestFetchResourceEventsAggregated_JobMergesPodEvents covers the Job →
// job-name label → Pods path.
func TestFetchResourceEventsAggregated_JobMergesPodEvents(t *testing.T) {
	now := time.Now()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "migrate", Namespace: "ns"},
	}
	pod := podWithLabels("migrate-xyz", "ns", map[string]string{"job-name": "migrate"})

	jobEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "job-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Job", Name: "migrate", Namespace: "ns"},
		Reason:         "SuccessfulCreate",
		LastTimestamp:  metav1.NewTime(now.Add(-10 * time.Minute)),
	}
	podEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "pod-evt", Namespace: "ns"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "migrate-xyz", Namespace: "ns"},
		Reason:         "Started",
		LastTimestamp:  metav1.NewTime(now.Add(-5 * time.Minute)),
	}

	cs := fake.NewSimpleClientset(job, pod, jobEvent, podEvent)
	events, err := FetchResourceEventsAggregated(context.Background(), cs, ResourceItem{Name: "migrate", Namespace: "ns", Raw: job})
	if err != nil {
		t.Fatalf("FetchResourceEventsAggregated: %v", err)
	}
	hasReason := func(r string) bool {
		for _, e := range events {
			if e.Reason == r {
				return true
			}
		}
		return false
	}
	if !hasReason("SuccessfulCreate") {
		t.Errorf("missing Job's own event: %+v", events)
	}
	if !hasReason("Started") {
		t.Errorf("missing child Pod's event: %+v", events)
	}
}
