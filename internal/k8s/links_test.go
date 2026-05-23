package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
