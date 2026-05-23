package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func makeDeployment(name, ns, revision string, uid types.UID) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			UID:       uid,
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": revision,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
		},
	}
}

func makeRS(name, ns, revision, hash string, owner *appsv1.Deployment) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": revision,
			},
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment", Name: owner.Name, UID: owner.UID,
			}},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				"app":               owner.Name,
				"pod-template-hash": hash,
			}},
		},
	}
}

func makePod(name, ns string, labels map[string]string, containers []string) *corev1.Pod {
	specCtrs := make([]corev1.Container, 0, len(containers))
	for _, c := range containers {
		specCtrs = append(specCtrs, corev1.Container{Name: c})
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec:       corev1.PodSpec{Containers: specCtrs},
	}
}

func TestPodsForDeployment_CurrentOnly_FiltersOldRSPods(t *testing.T) {
	dep := makeDeployment("nginx", "default", "3", "dep-uid-1")
	rsCur := makeRS("nginx-currev", "default", "3", "abc123", dep)
	rsOld := makeRS("nginx-oldrev", "default", "2", "old456", dep)

	podCur := makePod("nginx-abc123-pod1", "default",
		map[string]string{"app": "nginx", "pod-template-hash": "abc123"},
		[]string{"web"})
	podOld := makePod("nginx-old456-pod2", "default",
		map[string]string{"app": "nginx", "pod-template-hash": "old456"},
		[]string{"web"})

	cs := fake.NewSimpleClientset(dep, rsCur, rsOld, podCur, podOld)
	targets, err := PodsForDeployment(context.Background(), cs, dep, true)
	if err != nil {
		t.Fatalf("PodsForDeployment: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 current-rev pod, got %d (%v)", len(targets), targets)
	}
	if targets[0].Name != "nginx-abc123-pod1" {
		t.Errorf("expected current-rev pod, got %q", targets[0].Name)
	}
	if len(targets[0].Containers) != 1 || targets[0].Containers[0] != "web" {
		t.Errorf("expected containers=[web], got %v", targets[0].Containers)
	}
}

func TestPodsForDeployment_AllGenerations_IncludesOldPods(t *testing.T) {
	dep := makeDeployment("nginx", "default", "3", "dep-uid-2")
	rsCur := makeRS("nginx-currev", "default", "3", "abc123", dep)
	podCur := makePod("nginx-abc123-pod1", "default",
		map[string]string{"app": "nginx", "pod-template-hash": "abc123"},
		[]string{"web"})
	podOld := makePod("nginx-old456-pod2", "default",
		map[string]string{"app": "nginx", "pod-template-hash": "old456"},
		[]string{"web"})

	cs := fake.NewSimpleClientset(dep, rsCur, podCur, podOld)
	targets, err := PodsForDeployment(context.Background(), cs, dep, false)
	if err != nil {
		t.Fatalf("PodsForDeployment: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected both pods when currentOnly=false, got %d", len(targets))
	}
}

func TestPodsForDeployment_NoCurrentRS_FallsBackToSelector(t *testing.T) {
	dep := makeDeployment("nginx", "default", "5", "dep-uid-3")
	// RS for an earlier revision; nothing matches "5"
	rsStale := makeRS("nginx-stale", "default", "2", "stale", dep)
	pod := makePod("nginx-stale-x", "default",
		map[string]string{"app": "nginx", "pod-template-hash": "stale"},
		[]string{"web"})

	cs := fake.NewSimpleClientset(dep, rsStale, pod)
	targets, err := PodsForDeployment(context.Background(), cs, dep, true)
	if err != nil {
		t.Fatalf("PodsForDeployment: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("fallback selector should still return matching pods, got %d", len(targets))
	}
}

func TestPodsForDeployment_NoPods_EmptyResult(t *testing.T) {
	dep := makeDeployment("nginx", "default", "1", "dep-uid-4")
	cs := fake.NewSimpleClientset(dep)
	targets, err := PodsForDeployment(context.Background(), cs, dep, true)
	if err != nil {
		t.Fatalf("PodsForDeployment: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected empty when no pods, got %v", targets)
	}
}

func TestPodsForWorkload_RejectsUnsupportedKind(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"}}
	cs := fake.NewSimpleClientset(pod)
	item := ResourceItem{Name: "p", Namespace: "default", Raw: pod}
	_, err := PodsForWorkload(context.Background(), cs, item, true)
	if err == nil {
		t.Error("expected error for unsupported workload kind (Pod), got nil")
	}
}
