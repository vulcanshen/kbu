package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMarshalContainerYAML_ExtractsSpecAndStatus(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:1.21"},
				{Name: "sidecar", Image: "sidecar:v1"},
			},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 2},
			},
		},
	}
	item := ResourceItem{Name: "p", Namespace: "ns", Raw: pod}

	out := MarshalContainerYAML(item, "nginx")
	if out == "" {
		t.Fatal("expected non-empty YAML for nginx container")
	}
	if !strings.Contains(out, "nginx:1.21") {
		t.Errorf("YAML must contain container image, got:\n%s", out)
	}
	if !strings.Contains(out, "restartCount: 2") {
		t.Errorf("YAML must contain status, got:\n%s", out)
	}
	// Sidecar must not appear since we asked for nginx only.
	if strings.Contains(out, "sidecar") {
		t.Errorf("YAML must not include unrelated container, got:\n%s", out)
	}
}

func TestMarshalContainerYAML_InitContainer(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init-db", Image: "busybox"}},
		},
	}
	item := ResourceItem{Raw: pod}
	out := MarshalContainerYAML(item, "init-db")
	if !strings.Contains(out, "busybox") {
		t.Errorf("expected init container YAML, got %q", out)
	}
}

func TestMarshalContainerYAML_NotFound(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "a"}}},
	}
	item := ResourceItem{Raw: pod}
	if out := MarshalContainerYAML(item, "missing"); out != "" {
		t.Errorf("expected empty for missing container, got %q", out)
	}
}

func TestMarshalItemYAML_PopulatesGVK(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
	}
	item := ResourceItem{Name: "nginx", Namespace: "default", Raw: pod}

	out := MarshalItemYAML(item)
	if !strings.Contains(out, "apiVersion: v1") {
		t.Errorf("YAML must contain apiVersion at top, got:\n%s", out)
	}
	if !strings.Contains(out, "kind: Pod") {
		t.Errorf("YAML must contain kind, got:\n%s", out)
	}
}

func TestMarshalContainerYAML_NotAPod(t *testing.T) {
	item := ResourceItem{Raw: "not a pod"}
	if out := MarshalContainerYAML(item, "x"); out != "" {
		t.Errorf("expected empty when Raw is not *corev1.Pod, got %q", out)
	}
}
