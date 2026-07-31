package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeClientsetWithPods seeds pods across three namespaces so the
// multi-namespace fetch path can be exercised without a live cluster.
func fakeClientsetWithPods() *fake.Clientset {
	mk := func(ns, name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}
	return fake.NewSimpleClientset(
		mk("beta", "b1"),
		mk("alpha", "a1"),
		mk("alpha", "a2"),
		mk("gamma", "g1"),
	)
}

func namespacesOf(items []ResourceItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Namespace
	}
	return out
}

func TestFetchResourcesMulti_AllReturnsEveryPod(t *testing.T) {
	cs := fakeClientsetWithPods()
	items, err := FetchResourcesMulti(context.Background(), cs, ResourcePods, AllNamespaces())
	if err != nil {
		t.Fatalf("FetchResourcesMulti(All): %v", err)
	}
	if len(items) != 4 {
		t.Errorf("All must return every pod, got %d want 4 (ns: %v)", len(items), namespacesOf(items))
	}
}

func TestFetchResourcesMulti_SpecificOrderedByNamespace(t *testing.T) {
	cs := fakeClientsetWithPods()
	// Input order is beta, alpha — but the result must be alpha's pods
	// then beta's (requirement [5]: fetch per namespace, sorted by name).
	items, err := FetchResourcesMulti(context.Background(), cs, ResourcePods, SelectedNamespaces("beta", "alpha"))
	if err != nil {
		t.Fatalf("FetchResourcesMulti(specific): %v", err)
	}
	got := namespacesOf(items)
	want := []string{"alpha", "alpha", "beta"} // alpha (2) before beta (1); gamma excluded
	if len(got) != len(want) {
		t.Fatalf("expected 3 items (alpha×2 + beta×1), got %d (%v)", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("namespace order[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestFetchResourcesMulti_SingleNamespaceExcludesOthers(t *testing.T) {
	cs := fakeClientsetWithPods()
	items, err := FetchResourcesMulti(context.Background(), cs, ResourcePods, SelectedNamespaces("alpha"))
	if err != nil {
		t.Fatalf("FetchResourcesMulti(single): %v", err)
	}
	if len(items) != 2 {
		t.Errorf("alpha has 2 pods, got %d (%v)", len(items), namespacesOf(items))
	}
	for _, ns := range namespacesOf(items) {
		if ns != "alpha" {
			t.Errorf("selection {alpha} leaked namespace %q", ns)
		}
	}
}
