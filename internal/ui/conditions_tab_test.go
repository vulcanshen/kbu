package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/kbu/internal/k8s"
)

func TestSetResourceType_AppendsConditionsForKindsThatHaveThem(t *testing.T) {
	cases := []struct {
		rt       k8s.ResourceType
		wantTab  bool
		comment  string
		expectIn string // a tab that should always be present, for sanity
	}{
		{k8s.ResourcePods, true, "Pod always has conditions", "Logs"},
		{k8s.ResourceDeployments, true, "Deployment has Available/Progressing", "Logs"},
		{k8s.ResourceStatefulSets, true, "StatefulSet has conditions", "Events"},
		{k8s.ResourceDaemonSets, true, "DaemonSet has conditions", "Events"},
		{k8s.ResourceJobs, true, "Job has conditions when failed/completed", "Events"},
		{k8s.ResourceHorizontalPodAutoscalers, true, "HPA reports scaling conditions", "Events"},
		{k8s.ResourcePersistentVolumeClaims, true, "PVC has Bound condition", "Events"},
		{k8s.ResourceNodes, true, "Node has many conditions", "Events"},
		{k8s.ResourceIngresses, true, "Ingress may have conditions", "Events"},

		{k8s.ResourceConfigMaps, false, "ConfigMap has no conditions", "Events"},
		{k8s.ResourceSecrets, false, "Secret has no conditions", "Events"},
		{k8s.ResourceServices, false, "Service has no conditions", "Events"},
		{k8s.ResourceServiceAccounts, false, "ServiceAccount has no conditions", "Events"},
		{k8s.ResourceReleases, false, "Helm Release uses History instead", "History"},
	}

	for _, tc := range cases {
		t.Run(string(tc.rt), func(t *testing.T) {
			m := newTestDetail()
			m.SetResourceType(tc.rt)

			has := false
			for _, tab := range m.tabs {
				if tab == "Conditions" {
					has = true
					break
				}
			}
			if has != tc.wantTab {
				t.Errorf("%s: Conditions tab present=%v, want %v (%s)", tc.rt, has, tc.wantTab, tc.comment)
			}

			sanity := false
			for _, tab := range m.tabs {
				if tab == tc.expectIn {
					sanity = true
					break
				}
			}
			if !sanity {
				t.Errorf("%s: sanity tab %q missing — tabs=%v", tc.rt, tc.expectIn, m.tabs)
			}
		})
	}
}

func TestBuildConditionsLines_RendersRowsWithFailureHighlight(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)

	detail := sampleDetail()
	detail.Conditions = []k8s.ConditionItem{
		{Type: "PodScheduled", Status: "False", Reason: "Unschedulable",
			Message: "0/3 nodes are available: 3 Insufficient cpu", Age: "5m"},
		{Type: "Initialized", Status: "True", Age: "5m"},
		{Type: "Ready", Status: "False", Age: "5m"},
	}
	m.SetDetail(detail, nil)

	// Switch to Conditions tab.
	var condIdx int = -1
	for i, tab := range m.tabs {
		if tab == "Conditions" {
			condIdx = i
			break
		}
	}
	if condIdx < 0 {
		t.Fatalf("Conditions tab missing for Pod")
	}
	m = m.switchToTab(DetailTab(condIdx))

	joined := strings.Join(m.contentLines, "\n")
	for _, want := range []string{"TYPE", "STATUS", "REASON", "MESSAGE", "AGE", "PodScheduled", "Unschedulable", "Initialized", "Ready", "Insufficient cpu"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Conditions tab missing %q in render:\n%s", want, joined)
		}
	}
}

func TestBuildConditionsLines_EmptyShowsNoConditions(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)

	detail := sampleDetail()
	detail.Conditions = nil
	m.SetDetail(detail, nil)

	var condIdx int = -1
	for i, tab := range m.tabs {
		if tab == "Conditions" {
			condIdx = i
			break
		}
	}
	if condIdx < 0 {
		t.Fatalf("Conditions tab missing for Pod")
	}
	m = m.switchToTab(DetailTab(condIdx))

	joined := strings.Join(m.contentLines, "\n")
	if !strings.Contains(joined, "No conditions") {
		t.Errorf("expected 'No conditions' empty state, got:\n%s", joined)
	}
}
