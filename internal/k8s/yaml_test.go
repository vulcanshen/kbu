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

func TestMarshalItemYAMLForCompare_StripsStatusAndNoise(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "nginx",
			Namespace:         "default",
			UID:               "deadbeef-1234",
			ResourceVersion:   "98765",
			Generation:        7,
			CreationTimestamp: metav1.Date(2026, 1, 1, 12, 0, 0, 0, metav1.Now().Location()),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:1.27.1"}},
		},
		Status: corev1.PodStatus{
			Phase:   corev1.PodRunning,
			PodIP:   "10.244.0.5",
			HostIP:  "192.168.1.10",
			Message: "should not appear in compare",
		},
	}
	item := ResourceItem{Name: "nginx", Namespace: "default", Raw: pod}
	out := MarshalItemYAMLForCompare(item)
	if out == "" {
		t.Fatal("expected non-empty YAML")
	}
	for _, banned := range []string{
		"uid: deadbeef-1234",
		"resourceVersion: \"98765\"",
		"generation: 7",
		"creationTimestamp:",
		"status:",
		"10.244.0.5", // PodIP — inside status block
		"should not appear in compare",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("compare YAML must NOT contain %q, got:\n%s", banned, out)
		}
	}
	// Identity + spec must remain — the diff is meaningless without them.
	for _, want := range []string{
		"name: nginx",
		"namespace: default",
		"image: nginx:1.27.1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("compare YAML must contain %q, got:\n%s", want, out)
		}
	}
}

func TestStripStatusBlock_Idempotent(t *testing.T) {
	// stripStatusBlock should be a no-op on a YAML doc without `status:`.
	in := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: x\nspec:\n  containers:\n  - name: y\n"
	if got := stripStatusBlock(in); got != in {
		t.Errorf("stripStatusBlock changed a status-less doc:\nwant %q\ngot  %q", in, got)
	}
}

func TestStripStatusBlock_RemovesTopLevelStatus(t *testing.T) {
	in := strings.Join([]string{
		"apiVersion: v1",
		"kind: Pod",
		"metadata:",
		"  name: x",
		"spec:",
		"  containers:",
		"  - name: y",
		"status:",
		"  phase: Running",
		"  podIP: 10.0.0.1",
		"  conditions:",
		"  - type: Ready",
		"    status: \"True\"",
		"",
	}, "\n")
	out := stripStatusBlock(in)
	if strings.Contains(out, "status:") {
		t.Errorf("status: line still present, got:\n%s", out)
	}
	if strings.Contains(out, "podIP") {
		t.Errorf("podIP (status child) still present, got:\n%s", out)
	}
	if !strings.Contains(out, "kind: Pod") {
		t.Errorf("trimmed too aggressively — kind missing, got:\n%s", out)
	}
	if !strings.Contains(out, "name: y") {
		t.Errorf("container spec lost (mis-detected as status child), got:\n%s", out)
	}
}
