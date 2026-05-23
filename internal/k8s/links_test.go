package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func ptrStr(s string) *string { return &s }

func TestBuildIngressLinks_BackendsAndTLS(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "web"},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptrStr("nginx"),
			Rules: []networkingv1.IngressRule{{
				Host: "example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{Path: "/api", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api-svc"}}},
							{Path: "/web", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api-svc"}}}, // dup
						},
					},
				},
			}},
			TLS: []networkingv1.IngressTLS{{SecretName: "tls-cert", Hosts: []string{"example.com"}}},
		},
	}
	got := buildIngressLinks(ing)
	if len(got) != 3 {
		t.Fatalf("expected 3 sections (class, backends, tls), got %d", len(got))
	}
	if got[0].Entries[0].Ref.Type != ResourceIngressClasses {
		t.Errorf("class entry ref type = %s, want IngressClasses", got[0].Entries[0].Ref.Type)
	}
	if n := len(got[1].Entries); n != 1 {
		t.Errorf("backend services dedupe failed: got %d entries, want 1", n)
	}
	if got[1].Entries[0].Ref.Type != ResourceServices {
		t.Errorf("backend ref type = %s, want Services", got[1].Entries[0].Ref.Type)
	}
	if got[2].Entries[0].Ref.Type != ResourceSecrets {
		t.Errorf("TLS ref type = %s, want Secrets", got[2].Entries[0].Ref.Type)
	}
}

func TestBuildHPALinks_ScaleTarget(t *testing.T) {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "h", Namespace: "n"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "api"},
		},
	}
	got := buildHPALinks(hpa)
	if len(got) != 1 || len(got[0].Entries) != 1 {
		t.Fatalf("want 1 section/1 entry, got %+v", got)
	}
	e := got[0].Entries[0]
	if e.Ref == nil || e.Ref.Type != ResourceDeployments || e.Ref.Name != "api" {
		t.Errorf("scale target ref wrong: %+v", e.Ref)
	}
}

func TestBuildHPALinks_UnsupportedKind(t *testing.T) {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "ReplicaSet", Name: "x"},
		},
	}
	// ReplicaSet maps to Deployments via kindToResourceType, so this still produces a section.
	got := buildHPALinks(hpa)
	if len(got) != 1 {
		t.Fatalf("ReplicaSet should map to Deployment link, got %d sections", len(got))
	}

	hpa.Spec.ScaleTargetRef.Kind = "WeirdKind"
	if buildHPALinks(hpa) != nil {
		t.Errorf("unknown kind should return nil")
	}
}

func TestBuildPVCLinks_BoundAndStorageClass(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName:       "pv-001",
			StorageClassName: ptrStr("ssd"),
		},
	}
	got := buildPVCLinks(pvc)
	if len(got) != 1 || len(got[0].Entries) != 2 {
		t.Fatalf("want 1 section/2 entries, got %+v", got)
	}
	if got[0].Entries[0].Ref.Type != ResourcePersistentVolumes {
		t.Errorf("PV ref type wrong: %s", got[0].Entries[0].Ref.Type)
	}
	if got[0].Entries[1].Ref.Type != ResourceStorageClasses {
		t.Errorf("SC ref type wrong: %s", got[0].Entries[1].Ref.Type)
	}
}

func TestBuildPVCLinks_Unbound(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{}
	if got := buildPVCLinks(pvc); got != nil {
		t.Errorf("empty PVC should yield nil links, got %+v", got)
	}
}

func TestBuildCronJobLinks_ActiveJobs(t *testing.T) {
	cj := &batchv1.CronJob{
		Status: batchv1.CronJobStatus{
			Active: []corev1.ObjectReference{
				{Name: "job-1", Namespace: "batch"},
				{Name: "job-2", Namespace: "batch"},
			},
		},
	}
	got := buildCronJobLinks(cj)
	if len(got) != 1 || len(got[0].Entries) != 2 {
		t.Fatalf("want 2 active jobs, got %+v", got)
	}
	for i, e := range got[0].Entries {
		if e.Ref == nil || e.Ref.Type != ResourceJobs {
			t.Errorf("entry[%d] ref wrong: %+v", i, e.Ref)
		}
	}
}

func TestBuildWorkloadStaticLinks_SkipsDefaultSA(t *testing.T) {
	got := buildWorkloadStaticLinks(nil, "default", "n")
	if got != nil {
		t.Errorf("default SA should be skipped: %+v", got)
	}
	got = buildWorkloadStaticLinks(nil, "my-sa", "n")
	if len(got) != 1 || len(got[0].Entries) != 1 || got[0].Entries[0].Ref.Type != ResourceServiceAccounts {
		t.Errorf("non-default SA expected: %+v", got)
	}
}

func TestPodUsesConfigMap_VolumeAndEnv(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "volume",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "shared"}}},
				}},
			}},
			want: true,
		},
		{
			name: "envFrom",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{EnvFrom: []corev1.EnvFromSource{{
					ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "shared"}},
				}}}},
			}},
			want: true,
		},
		{
			name: "valueFrom",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{Env: []corev1.EnvVar{{ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "shared"}}}}}}},
			}},
			want: true,
		},
		{
			name: "projected",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{Name: "p", VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "shared"}}}}},
				}}},
			}},
			want: true,
		},
		{
			name: "miss",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Image: "x"}},
			}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := podUsesConfigMap(tc.pod, "shared"); got != tc.want {
				t.Errorf("podUsesConfigMap = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPodUsesSecret_AllChannels(t *testing.T) {
	cases := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "imagePullSecret",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
			}},
			want: true,
		},
		{
			name: "secret volume",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{{VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{SecretName: "regcred"}}}},
			}},
			want: true,
		},
		{
			name: "envFrom",
			pod: &corev1.Pod{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "regcred"}}}}}},
			}},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := podUsesSecret(tc.pod, "regcred"); got != tc.want {
				t.Errorf("podUsesSecret = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnrichLinks_ConfigMapConsumers(t *testing.T) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "ns"}}
	user := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "user-pod", Namespace: "ns"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{Name: "c", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "shared"}}}}},
		},
	}
	other := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns"}}
	cs := fake.NewSimpleClientset(cm, user, other)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceConfigMaps, ResourceItem{Raw: cm}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 consumer, got %+v", detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "user-pod" {
		t.Errorf("wrong consumer: %s", detail.Links[0].Entries[0].Ref.Name)
	}
}

func TestBuildEventLinks(t *testing.T) {
	e := &corev1.Event{
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "p1", Namespace: "ns"},
	}
	got := buildEventLinks(e)
	if len(got) != 1 || got[0].Entries[0].Ref.Type != ResourcePods {
		t.Fatalf("expected Pod drill, got %+v", got)
	}

	unknown := &corev1.Event{InvolvedObject: corev1.ObjectReference{Kind: "CustomThing", Name: "x"}}
	if buildEventLinks(unknown) != nil {
		t.Errorf("unknown kind should yield nil")
	}
}

func TestBuildPVLinks_ClaimAndStorageClass(t *testing.T) {
	pv := &corev1.PersistentVolume{
		Spec: corev1.PersistentVolumeSpec{
			ClaimRef:         &corev1.ObjectReference{Name: "pvc-a", Namespace: "app"},
			StorageClassName: "ssd",
		},
	}
	got := buildPVLinks(pv)
	if len(got) != 1 || len(got[0].Entries) != 2 {
		t.Fatalf("want 1 section/2 entries, got %+v", got)
	}
	if got[0].Entries[0].Ref.Type != ResourcePersistentVolumeClaims || got[0].Entries[0].Ref.Namespace != "app" {
		t.Errorf("claim ref wrong: %+v", got[0].Entries[0].Ref)
	}
	if got[0].Entries[1].Ref.Type != ResourceStorageClasses {
		t.Errorf("SC ref wrong: %+v", got[0].Entries[1].Ref)
	}
}

func TestBuildEndpointSliceLinks_ServiceAndPods(t *testing.T) {
	es := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "web",
			Labels:    map[string]string{"kubernetes.io/service-name": "api"},
		},
		Endpoints: []discoveryv1.Endpoint{
			{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-1", Namespace: "web"}},
			{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-1", Namespace: "web"}}, // dup
			{TargetRef: &corev1.ObjectReference{Kind: "Node", Name: "n1"}},                     // not a Pod
		},
	}
	got := buildEndpointSliceLinks(es)
	if len(got) != 2 {
		t.Fatalf("want 2 sections (service, endpoints), got %d", len(got))
	}
	if got[0].Entries[0].Ref.Type != ResourceServices || got[0].Entries[0].Ref.Name != "api" {
		t.Errorf("service ref wrong: %+v", got[0].Entries[0].Ref)
	}
	if n := len(got[1].Entries); n != 1 {
		t.Errorf("endpoint dedupe + Pod filter failed: %d entries, want 1", n)
	}
}

func TestBuildClusterRoleBindingLinks_RoleRefAndSubjects(t *testing.T) {
	crb := &rbacv1.ClusterRoleBinding{
		RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "sa1", Namespace: "ns"},
			{Kind: "User", Name: "alice"},
		},
	}
	got := buildClusterRoleBindingLinks(crb)
	if len(got) != 2 {
		t.Fatalf("want 2 sections, got %d", len(got))
	}
	if got[0].Entries[0].Ref.Type != ResourceClusterRoles {
		t.Errorf("roleRef wrong: %+v", got[0].Entries[0].Ref)
	}
	if got[1].Entries[0].Ref == nil || got[1].Entries[0].Ref.Type != ResourceServiceAccounts {
		t.Errorf("SA subject should be drillable: %+v", got[1].Entries[0])
	}
	if got[1].Entries[1].Ref != nil {
		t.Errorf("User subject should not be drillable: %+v", got[1].Entries[1])
	}
}

func TestBuildRoleBindingLinks_RoleScopedToBindingNS(t *testing.T) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "viewer"},
	}
	got := buildRoleBindingLinks(rb)
	if len(got) == 0 || got[0].Entries[0].Ref.Namespace != "team-a" {
		t.Errorf("role ref should inherit binding namespace, got %+v", got[0].Entries[0].Ref)
	}

	rb2 := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
	}
	got2 := buildRoleBindingLinks(rb2)
	if got2[0].Entries[0].Ref.Namespace != "" {
		t.Errorf("ClusterRole ref should be cluster-scoped, got %+v", got2[0].Entries[0].Ref)
	}
}

func TestBuildServiceAccountStaticLinks(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta:       metav1.ObjectMeta{Name: "sa1", Namespace: "ns"},
		Secrets:          []corev1.ObjectReference{{Name: "token-1"}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
	}
	got := buildServiceAccountStaticLinks(sa)
	if len(got) != 2 {
		t.Fatalf("want 2 sections, got %d", len(got))
	}
	if got[0].Entries[0].Ref.Type != ResourceSecrets || got[0].Entries[0].Ref.Namespace != "ns" {
		t.Errorf("Secret ref wrong: %+v", got[0].Entries[0].Ref)
	}
	if got[1].Entries[0].Ref.Name != "regcred" {
		t.Errorf("ImagePullSecret ref wrong: %+v", got[1].Entries[0])
	}
}

func TestEnrichLinks_NodePods(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	other := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "o", Namespace: "ns"},
		Spec:       corev1.PodSpec{NodeName: "node-b"},
	}
	cs := fake.NewSimpleClientset(node, pod, other)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceNodes, ResourceItem{Raw: node}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 pod, got %+v", detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "p" {
		t.Errorf("wrong pod: %s", detail.Links[0].Entries[0].Ref.Name)
	}
}

func TestEnrichLinks_ServiceAccountConsumers_DefaultSADoesNotMatchEmpty(t *testing.T) {
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "my-sa", Namespace: "ns"}}
	user := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "user", Namespace: "ns"},
		Spec:       corev1.PodSpec{ServiceAccountName: "my-sa"},
	}
	other := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns"},
		Spec:       corev1.PodSpec{}, // empty SA → "default"
	}
	cs := fake.NewSimpleClientset(sa, user, other)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceServiceAccounts, ResourceItem{Raw: sa}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("want only 'user' pod, got %+v", detail.Links)
	}
}

func TestEnrichLinks_PDBPods(t *testing.T) {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "pdb", Namespace: "ns"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "web"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "ns", Labels: map[string]string{"tier": "web"}},
	}
	cs := fake.NewSimpleClientset(pdb, pod)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourcePodDisruptionBudgets, ResourceItem{Raw: pdb}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 pod section, got %+v", detail.Links)
	}
}

func TestEnrichLinks_NetworkPolicyPods(t *testing.T) {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny", Namespace: "ns"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "db"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db-1", Namespace: "ns", Labels: map[string]string{"role": "db"}},
	}
	cs := fake.NewSimpleClientset(np, pod)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceNetworkPolicies, ResourceItem{Raw: np}, detail)
	if len(detail.Links) != 1 || detail.Links[0].Entries[0].Ref.Name != "db-1" {
		t.Fatalf("expected db-1 pod, got %+v", detail.Links)
	}
}

func TestEnrichLinks_RoleBindings(t *testing.T) {
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "viewer", Namespace: "ns"}}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "view-binding", Namespace: "ns"},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "viewer"},
	}
	other := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "other-binding", Namespace: "ns"},
		RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "different"},
	}
	cs := fake.NewSimpleClientset(role, rb, other)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceRoles, ResourceItem{Raw: role}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 binding, got %+v", detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "view-binding" {
		t.Errorf("wrong binding: %s", detail.Links[0].Entries[0].Ref.Name)
	}
}

func TestEnrichLinks_PodOwner_ResolvesReplicaSetToDeployment(t *testing.T) {
	// Pod is owned by a ReplicaSet (Kubernetes auto-created); the RS in
	// turn is owned by the Deployment "harbor-core". buildPodLinks
	// initially sets Owner to Deployments/<RS-name> (wrong name);
	// EnrichLinks must rewrite Name to the actual Deployment name.
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "harbor-core-847f66dfbc",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: "harbor-core"},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "harbor-core-847f66dfbc-xyz",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "harbor-core-847f66dfbc"},
			},
		},
	}
	cs := fake.NewSimpleClientset(rs, pod)

	detail := &ResourceDetail{
		PodLinks: &PodLinksData{
			Owner: &RefTarget{
				Type:      ResourceDeployments,
				Name:      "harbor-core-847f66dfbc", // wrong name set by buildPodLinks
				Namespace: "default",
			},
		},
	}
	EnrichLinks(context.Background(), cs, ResourcePods, ResourceItem{Raw: pod}, detail)

	if detail.PodLinks.Owner.Name != "harbor-core" {
		t.Errorf("Owner.Name should be resolved to 'harbor-core', got %q", detail.PodLinks.Owner.Name)
	}
	if detail.PodLinks.Owner.Type != ResourceDeployments {
		t.Errorf("Owner.Type should stay Deployments, got %s", detail.PodLinks.Owner.Type)
	}
}

func TestEnrichLinks_PodOwner_NonReplicaSetUnchanged(t *testing.T) {
	// DaemonSet-owned pods carry the DS name directly — EnrichLinks must
	// leave the Owner alone (already-correct).
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-exporter-abc",
			Namespace: "monitoring",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "node-exporter"},
			},
		},
	}
	cs := fake.NewSimpleClientset(pod)

	detail := &ResourceDetail{
		PodLinks: &PodLinksData{
			Owner: &RefTarget{Type: ResourceDaemonSets, Name: "node-exporter", Namespace: "monitoring"},
		},
	}
	EnrichLinks(context.Background(), cs, ResourcePods, ResourceItem{Raw: pod}, detail)

	if detail.PodLinks.Owner.Name != "node-exporter" {
		t.Errorf("DaemonSet owner should be untouched, got %q", detail.PodLinks.Owner.Name)
	}
}

func TestEnrichLinks_PodOwner_RSLookupFailureLeavesOwnerUnchanged(t *testing.T) {
	// If the RS lookup fails (e.g., RBAC, deleted mid-rollout), leave
	// the initial (wrong) Owner alone — the user will at least see a
	// "not found" toast when they try to drill, rather than us erasing
	// the link entirely.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "missing-rs"},
			},
		},
	}
	cs := fake.NewSimpleClientset(pod) // no RS object

	detail := &ResourceDetail{
		PodLinks: &PodLinksData{
			Owner: &RefTarget{Type: ResourceDeployments, Name: "missing-rs", Namespace: "default"},
		},
	}
	EnrichLinks(context.Background(), cs, ResourcePods, ResourceItem{Raw: pod}, detail)

	if detail.PodLinks.Owner.Name != "missing-rs" {
		t.Errorf("on lookup failure, Owner should be unchanged; got %q", detail.PodLinks.Owner.Name)
	}
}

func TestEnrichLinks_ClusterRoleBindings(t *testing.T) {
	cr := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "view"}}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "viewers"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
	}
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "team-viewers", Namespace: "team-a"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
	}
	unrelated := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "editors"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
	}
	cs := fake.NewSimpleClientset(cr, crb, rb, unrelated)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceClusterRoles, ResourceItem{Raw: cr}, detail)
	if len(detail.Links) != 2 {
		t.Fatalf("expected 2 sections (CRBs + RBs), got %d: %+v", len(detail.Links), detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "viewers" {
		t.Errorf("CRB ref wrong: %+v", detail.Links[0].Entries[0])
	}
	if detail.Links[1].Entries[0].Ref.Name != "team-viewers" || detail.Links[1].Entries[0].Ref.Namespace != "team-a" {
		t.Errorf("RB ref wrong: %+v", detail.Links[1].Entries[0])
	}
}

func TestEnrichLinks_StorageClassPVCs(t *testing.T) {
	sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "ssd"}}
	user := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "app"},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: ptrStr("ssd")},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	other := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "scratch", Namespace: "app"},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: ptrStr("hdd")},
	}
	cs := fake.NewSimpleClientset(sc, user, other)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceStorageClasses, ResourceItem{Raw: sc}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 PVC, got %+v", detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "data" {
		t.Errorf("wrong PVC: %s", detail.Links[0].Entries[0].Ref.Name)
	}
}

func TestEnrichLinks_IngressClassIngresses(t *testing.T) {
	ic := &networkingv1.IngressClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}}
	user := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "web"},
		Spec:       networkingv1.IngressSpec{IngressClassName: ptrStr("nginx")},
	}
	other := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "web"},
		Spec:       networkingv1.IngressSpec{IngressClassName: ptrStr("traefik")},
	}
	cs := fake.NewSimpleClientset(ic, user, other)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceIngressClasses, ResourceItem{Raw: ic}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 ingress, got %+v", detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "site" {
		t.Errorf("wrong Ingress: %s", detail.Links[0].Entries[0].Ref.Name)
	}
}

func TestEnrichLinks_WorkloadPods(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "ns"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "api-1", Namespace: "ns",
			Labels: map[string]string{"app": "api"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := fake.NewSimpleClientset(dep, pod)

	detail := &ResourceDetail{}
	EnrichLinks(context.Background(), cs, ResourceDeployments, ResourceItem{Raw: dep}, detail)
	if len(detail.Links) != 1 || len(detail.Links[0].Entries) != 1 {
		t.Fatalf("expected 1 pod section/1 entry, got %+v", detail.Links)
	}
	if detail.Links[0].Entries[0].Ref.Name != "api-1" {
		t.Errorf("wrong pod: %s", detail.Links[0].Entries[0].Ref.Name)
	}
}
