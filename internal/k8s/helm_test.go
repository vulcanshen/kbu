package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestParseReleases_Empty(t *testing.T) {
	got, err := parseReleases(nil)
	if err != nil {
		t.Fatalf("parseReleases(nil) returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestParseReleases_Sample(t *testing.T) {
	// Sample shape from `helm list -A -o json` (helm v3).
	sample := []byte(`[
		{
			"name": "nginx",
			"namespace": "default",
			"revision": "3",
			"updated": "2024-05-19 14:31:22.123456 +0800 CST",
			"status": "deployed",
			"chart": "nginx-15.4.4",
			"app_version": "1.25.3"
		},
		{
			"name": "redis",
			"namespace": "data",
			"revision": "1",
			"updated": "2024-05-20 09:00:00.000000 +0000 UTC",
			"status": "failed",
			"chart": "redis-18.0.0",
			"app_version": "7.2.4"
		}
	]`)

	got, err := parseReleases(sample)
	if err != nil {
		t.Fatalf("parseReleases returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(got))
	}
	if got[0].Name != "nginx" || got[0].Namespace != "default" {
		t.Errorf("first release: got name=%q ns=%q", got[0].Name, got[0].Namespace)
	}
	if got[0].Chart != "nginx-15.4.4" || got[0].AppVersion != "1.25.3" {
		t.Errorf("first release chart/appver mismatch: %+v", got[0])
	}
	if got[1].Status != "failed" {
		t.Errorf("second release status: got %q want failed", got[1].Status)
	}
}

func TestParseReleases_BadJSON(t *testing.T) {
	_, err := parseReleases([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse helm list") {
		t.Errorf("error message missing context prefix: %v", err)
	}
}

func TestFormatHelmUpdated_Unknown(t *testing.T) {
	if got := formatHelmUpdated(""); got != "<unknown>" {
		t.Errorf("empty string: got %q want <unknown>", got)
	}
}

func TestFormatHelmUpdated_FallbackOnParseFail(t *testing.T) {
	// Garbage that does not match the helm time layout should pass through
	// unchanged so the user still sees something rather than a stale tombstone.
	raw := "garbage-timestamp"
	if got := formatHelmUpdated(raw); got != raw {
		t.Errorf("got %q want %q", got, raw)
	}
}

func TestDetectHelm_Smoke(t *testing.T) {
	// detectHelm is a pure exec.LookPath check — assertion-light because the
	// CI runner may or may not have helm installed. Just ensure it returns a
	// bool without panicking.
	_ = detectHelm()
}

func TestDetailRelease_Stub(t *testing.T) {
	rel := &Release{
		Name:       "nginx",
		Namespace:  "default",
		Revision:   "3",
		Status:     "deployed",
		Chart:      "nginx-15.4.4",
		AppVersion: "1.25.3",
	}
	item := ResourceItem{Name: rel.Name, Namespace: rel.Namespace, UID: "helm/default/nginx", Raw: rel}
	d := detailRelease(item)
	if d.Name != "nginx" || d.Namespace != "default" || d.Kind != "Release" {
		t.Errorf("detail meta wrong: %+v", d)
	}
	if len(d.Fields) != 5 {
		t.Errorf("expected 5 detail fields, got %d", len(d.Fields))
	}
}

func TestPollWatch_FiresModifiedEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pw := newPollWatch(ctx, 20*time.Millisecond)
	defer pw.Stop()

	select {
	case ev, ok := <-pw.ResultChan():
		if !ok {
			t.Fatal("channel closed before any event")
		}
		if ev.Type != watch.Modified {
			t.Errorf("expected watch.Modified, got %v", ev.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no event within 500ms — poll watch not firing")
	}
}

func TestPollWatch_StopClosesChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pw := newPollWatch(ctx, 10*time.Millisecond)
	pw.Stop()

	// After Stop, the channel must eventually close. Drain any in-flight event
	// and wait for the close signal (zero-value receive with !ok).
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case _, ok := <-pw.ResultChan():
			if !ok {
				return // closed, as expected
			}
		case <-deadline:
			t.Fatal("ResultChan not closed within 500ms after Stop")
		}
	}
}

// ── Phase 2d: Rule A + helm storage secret filter ─────────────────────────

func TestIsHelmManaged_LabelMatch(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "x",
			Labels: map[string]string{"app.kubernetes.io/managed-by": "Helm"},
		},
	}
	item := ResourceItem{Name: "x", Raw: pod}
	if !IsHelmManaged(item) {
		t.Error("label app.kubernetes.io/managed-by=Helm should match")
	}
}

func TestIsHelmManaged_AnnotationMatch(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "x",
			Annotations: map[string]string{"meta.helm.sh/release-name": "foo"},
		},
	}
	item := ResourceItem{Name: "x", Raw: pod}
	if !IsHelmManaged(item) {
		t.Error("annotation meta.helm.sh/release-name should match")
	}
}

func TestIsHelmManaged_Negative(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "x",
			Labels: map[string]string{"app.kubernetes.io/managed-by": "kustomize"},
		},
	}
	if IsHelmManaged(ResourceItem{Name: "x", Raw: pod}) {
		t.Error("non-helm managed-by should not match")
	}
	// Nil Raw / non-Object Raw shouldn't panic and should return false.
	if IsHelmManaged(ResourceItem{Name: "x"}) {
		t.Error("nil Raw should not be helm-managed")
	}
}

func TestIsHelmStorageSecret(t *testing.T) {
	tests := []struct {
		secType string
		want    bool
	}{
		{"helm.sh/release.v1", true},
		{"Opaque", false},
		{"kubernetes.io/service-account-token", false},
		{"", false},
	}
	for _, tc := range tests {
		sec := &corev1.Secret{Type: corev1.SecretType(tc.secType)}
		got := IsHelmStorageSecret(ResourceItem{Raw: sec})
		if got != tc.want {
			t.Errorf("type=%q got %v want %v", tc.secType, got, tc.want)
		}
	}
	// Non-Secret Raw: false (defensive)
	if IsHelmStorageSecret(ResourceItem{Raw: &corev1.Pod{}}) {
		t.Error("non-Secret Raw should return false")
	}
}

func TestToggleHelmHideManaged(t *testing.T) {
	// Save / restore so this test can run in any order without leaking
	// state into the rest of the suite.
	orig := HelmHideManaged()
	defer SetHelmHideManaged(orig)

	SetHelmHideManaged(true)
	if v := ToggleHelmHideManaged(); v {
		t.Errorf("toggle from true should yield false, got %v", v)
	}
	if v := ToggleHelmHideManaged(); !v {
		t.Errorf("toggle from false should yield true, got %v", v)
	}
}

func TestMarkHelm(t *testing.T) {
	managed := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "nginx",
			Labels: map[string]string{"app.kubernetes.io/managed-by": "Helm"},
		},
	}
	plain := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "raw"}}
	if got := MarkHelm(ResourceItem{Raw: managed}); got == "" {
		t.Error("helm-managed pod should return non-empty MarkHelm icon")
	}
	if got := MarkHelm(ResourceItem{Raw: plain}); got != "" {
		t.Errorf("plain pod should return empty MarkHelm, got %q", got)
	}
}

// ── Phase 2c: helm history + rollback ─────────────────────────────────────

func TestParseReleaseHistory_Sample(t *testing.T) {
	sample := []byte(`[
		{"revision":1,"updated":"2026-05-19T10:00:00.000000+08:00","status":"superseded","chart":"harbor-1.16.0","app_version":"2.12.0","description":"Install complete"},
		{"revision":2,"updated":"2026-05-20T16:18:54.682817+08:00","status":"deployed","chart":"harbor-1.17.0","app_version":"2.13.0","description":"Upgrade complete"}
	]`)
	got, err := parseReleaseHistory(sample)
	if err != nil {
		t.Fatalf("parseReleaseHistory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(got))
	}
	if got[0].Revision != 1 || got[1].Revision != 2 {
		t.Errorf("revision numbers wrong: %+v", got)
	}
	if got[1].Status != "deployed" {
		t.Errorf("rev 2 status: got %q want deployed", got[1].Status)
	}
	if got[1].Description != "Upgrade complete" {
		t.Errorf("description not parsed: %+v", got[1])
	}
}

func TestParseReleaseHistory_Empty(t *testing.T) {
	got, err := parseReleaseHistory(nil)
	if err != nil {
		t.Fatalf("nil input should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestParseReleaseHistory_BadJSON(t *testing.T) {
	if _, err := parseReleaseHistory([]byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRollbackCommandString(t *testing.T) {
	got := RollbackCommandString("harbor", "harbor", 3)
	want := "helm rollback harbor 3 -n harbor"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFormatHelmHistoryDate_Empty(t *testing.T) {
	if got := FormatHelmHistoryDate(""); got != "" {
		t.Errorf("empty input: got %q want empty", got)
	}
}

func TestFormatHelmHistoryDate_FallbackOnParseFail(t *testing.T) {
	raw := "not-a-date"
	if got := FormatHelmHistoryDate(raw); got != raw {
		t.Errorf("got %q want passthrough %q", got, raw)
	}
}

// ── Phase 2b: parseManifestResources / kindToResourceType ─────────────────

func TestParseManifestResources_BasicMultiDoc(t *testing.T) {
	// Shape modelled after real `helm get manifest` output: leading "---\n"
	// from helm's renderer (yields an empty first chunk after split on
	// "\n---\n", but also produces a stray "---" trailing on the first
	// real doc). We expect both real resources to come through.
	manifest := `---
# Source: chart/templates/deploy.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: default
spec:
  replicas: 2
---
# Source: chart/templates/svc.yaml
apiVersion: v1
kind: Service
metadata:
  name: app
  namespace: default
spec:
  ports:
  - port: 80
`
	got := parseManifestResources(manifest)
	if len(got) != 2 {
		t.Fatalf("expected 2 resources, got %d: %+v", len(got), got)
	}
	if got[0].Kind != "Deployment" || got[0].Name != "app" || got[0].Namespace != "default" {
		t.Errorf("got[0] wrong: %+v", got[0])
	}
	if got[1].Kind != "Service" {
		t.Errorf("got[1].Kind = %q, want Service", got[1].Kind)
	}
}

func TestParseManifestResources_SkipsCommentOnlyAndMissingFields(t *testing.T) {
	manifest := `---
# Source: chart/templates/comment-only.yaml
# (no resource)
---
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-ok
  namespace: default
`
	got := parseManifestResources(manifest)
	if len(got) != 1 {
		t.Fatalf("expected 1 valid resource (comment-only + missing-name skipped), got %d: %+v", len(got), got)
	}
	if got[0].Name != "cm-ok" {
		t.Errorf("expected cm-ok, got %+v", got[0])
	}
}

func TestParseManifestResources_Empty(t *testing.T) {
	if got := parseManifestResources(""); len(got) != 0 {
		t.Errorf("empty input should yield no resources, got %d", len(got))
	}
}

// Note: TestKindToResourceType_* covers the kinds I added when extending
// the existing kindToResourceType helper for Phase 2b (Helm Deployed
// Resources). The pre-existing kinds (Pod / Deployment / Service / ...)
// have implicit coverage through their original callers; here I only test
// the new entries and the negative path.
func TestKindToResourceType_NewKinds(t *testing.T) {
	cases := map[string]ResourceType{
		"Namespace":               ResourceNamespaces,
		"Event":                   ResourceEvents,
		"Role":                    ResourceRoles,
		"RoleBinding":             ResourceRoleBindings,
		"ClusterRole":             ResourceClusterRoles,
		"ClusterRoleBinding":      ResourceClusterRoleBindings,
		"Ingress":                 ResourceIngresses,
		"IngressClass":            ResourceIngressClasses,
		"NetworkPolicy":           ResourceNetworkPolicies,
		"EndpointSlice":           ResourceEndpointSlices,
		"StorageClass":            ResourceStorageClasses,
		"HorizontalPodAutoscaler": ResourceHorizontalPodAutoscalers,
		"PodDisruptionBudget":     ResourcePodDisruptionBudgets,
	}
	for kind, want := range cases {
		got, ok := kindToResourceType(kind)
		if !ok || got != want {
			t.Errorf("kindToResourceType(%q) = (%q, %v), want (%q, true)", kind, got, ok, want)
		}
	}
}

func TestKindToResourceType_UnknownReturnsFalse(t *testing.T) {
	// CRDs and anything km8 doesn't register should drop out.
	for _, kind := range []string{"ServiceMonitor", "Certificate", "VirtualService", "Issuer", ""} {
		if _, ok := kindToResourceType(kind); ok {
			t.Errorf("kindToResourceType(%q) ok=true, want false", kind)
		}
	}
}

func TestDetailRelease_NonReleaseRaw(t *testing.T) {
	// Defensive: if a caller hands in a ResourceItem whose Raw is not a
	// *Release we should still return a valid (if sparse) ResourceDetail.
	item := ResourceItem{Name: "x", Namespace: "y", UID: "z", Raw: "not-a-release"}
	d := detailRelease(item)
	if d.Kind != "Release" {
		t.Errorf("kind: got %q", d.Kind)
	}
	if len(d.Fields) != 0 {
		t.Errorf("expected no fields when Raw is wrong type, got %d", len(d.Fields))
	}
}
