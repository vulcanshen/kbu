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

// ── Pod Overview parsing ──────────────────────────────────────────────────

func TestBuildPodOverview_ExtractsOwnerNodeServiceAccount(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-7f9c4d-abc12",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet", Name: "nginx-7f9c4d", UID: "rs-uid",
			}},
		},
		Spec: corev1.PodSpec{
			NodeName:           "worker-3",
			ServiceAccountName: "nginx-sa",
			Containers:         []corev1.Container{{Name: "web", Image: "nginx:1.27.1"}},
			InitContainers:     []corev1.Container{{Name: "wait-db", Image: "busybox:latest"}},
		},
	}
	po := buildPodOverview(pod)
	if po.Owner == nil || po.Owner.Name != "nginx-7f9c4d" {
		t.Errorf("owner: expected nginx-7f9c4d, got %v", po.Owner)
	}
	if po.Owner.Type != ResourceDeployments {
		t.Errorf("ReplicaSet owner should map to Deployment (closest user-meaningful kind), got %v", po.Owner.Type)
	}
	if po.Node == nil || po.Node.Name != "worker-3" {
		t.Errorf("node: expected worker-3, got %v", po.Node)
	}
	if po.Node.Type != ResourceNodes {
		t.Errorf("node Type should be ResourceNodes, got %v", po.Node.Type)
	}
	if po.ServiceAccount == nil || po.ServiceAccount.Name != "nginx-sa" {
		t.Errorf("SA: expected nginx-sa, got %v", po.ServiceAccount)
	}
	if len(po.Images) != 1 || po.Images[0] != "nginx:1.27.1" {
		t.Errorf("images: expected [nginx:1.27.1], got %v", po.Images)
	}
	if len(po.InitImages) != 1 || po.InitImages[0] != "busybox:latest" {
		t.Errorf("init images: expected [busybox:latest], got %v", po.InitImages)
	}
}

func TestBuildPodOverview_SkipsDefaultServiceAccount(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec:       corev1.PodSpec{ServiceAccountName: "default"},
	}
	po := buildPodOverview(pod)
	if po.ServiceAccount != nil {
		t.Errorf("default SA should be elided, got %v", po.ServiceAccount)
	}
}

func TestBuildPodOverview_VolumesMapToDrillRefs(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: "config", VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "nginx-config"},
					},
				}},
				{Name: "tls", VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "nginx-tls"},
				}},
				{Name: "data", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"},
				}},
				{Name: "tmp", VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
	}
	po := buildPodOverview(pod)
	if len(po.Volumes) != 4 {
		t.Fatalf("expected 4 volume refs, got %d", len(po.Volumes))
	}
	got := map[string]VolumeRef{}
	for _, v := range po.Volumes {
		got[v.Name] = v
	}
	if got["config"].Kind != "configMap" || got["config"].Ref == nil || got["config"].Ref.Type != ResourceConfigMaps || got["config"].Ref.Name != "nginx-config" {
		t.Errorf("config volume mapped wrong: %+v", got["config"])
	}
	if got["tls"].Kind != "secret" || got["tls"].Ref == nil || got["tls"].Ref.Type != ResourceSecrets || got["tls"].Ref.Name != "nginx-tls" {
		t.Errorf("tls volume mapped wrong: %+v", got["tls"])
	}
	if got["data"].Kind != "pvc" || got["data"].Ref == nil || got["data"].Ref.Type != ResourcePersistentVolumeClaims || got["data"].Ref.Name != "data-pvc" {
		t.Errorf("pvc volume mapped wrong: %+v", got["data"])
	}
	if got["tmp"].Kind != "emptyDir" || got["tmp"].Ref != nil {
		t.Errorf("emptyDir volume must be informational (no Ref), got %+v", got["tmp"])
	}
}

func TestBuildPodOverview_UnknownOwnerKindOmits(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "WeirdControllerKind", Name: "thing",
			}},
		},
	}
	po := buildPodOverview(pod)
	if po.Owner != nil {
		t.Errorf("unknown owner kind should be omitted (not drillable), got %v", po.Owner)
	}
}

// ── FetchResourceByRef ────────────────────────────────────────────────────

func TestFetchResourceByRef_Pod(t *testing.T) {
	pod := makePod("nginx-abc", "default", nil, []string{"web"})
	cs := fake.NewSimpleClientset(pod)
	item, err := FetchResourceByRef(context.Background(), cs, RefTarget{
		Type: ResourcePods, Name: "nginx-abc", Namespace: "default",
	})
	if err != nil {
		t.Fatalf("FetchResourceByRef: %v", err)
	}
	if item.Name != "nginx-abc" {
		t.Errorf("expected Name=nginx-abc, got %q", item.Name)
	}
	if _, ok := item.Raw.(*corev1.Pod); !ok {
		t.Errorf("expected Raw to be *corev1.Pod, got %T", item.Raw)
	}
}

func TestFetchResourceByRef_UnsupportedType(t *testing.T) {
	cs := fake.NewSimpleClientset()
	_, err := FetchResourceByRef(context.Background(), cs, RefTarget{
		Type: "made-up", Name: "x",
	})
	if err == nil {
		t.Error("expected error for unsupported type")
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
