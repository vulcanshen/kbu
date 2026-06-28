package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestDetail() DetailModel {
	t := theme.DefaultTheme()
	m := NewDetailModel(t)
	m.SetSize(80, 20)
	m.SetFocused(true)
	return m
}

func sampleDetail() k8s.ResourceDetail {
	return k8s.ResourceDetail{
		Name:      "nginx-7b4f6c8d4-abc12",
		Namespace: "default",
		Kind:      "Pod",
		UID:       "abc-123-def",
		CreatedAt: "3d ago",
		Labels: map[string]string{
			"app":     "nginx",
			"version": "1.0",
		},
		Annotations: map[string]string{
			"kubectl.kubernetes.io/last-applied-configuration": "...",
		},
		Fields: []k8s.DetailField{
			{Label: "Status", Value: "Running"},
			{Label: "Node", Value: "orbstack"},
			{Label: "IP", Value: "10.0.0.5"},
		},
	}
}

func sampleEvents() []k8s.EventItem {
	return []k8s.EventItem{
		{Type: "Normal", Reason: "Pulled", Object: "Pod/nginx", Message: "Successfully pulled image", Age: "3m"},
		{Type: "Normal", Reason: "Created", Object: "Pod/nginx", Message: "Created container nginx", Age: "3m"},
		{Type: "Normal", Reason: "Started", Object: "Pod/nginx", Message: "Started container nginx", Age: "3m"},
	}
}

func TestDetailModel_InitialState(t *testing.T) {
	m := newTestDetail()

	if m.hasData {
		t.Error("expected hasData=false initially")
	}
	if m.activeTab != DetailTabInfo {
		t.Errorf("expected activeTab=DetailTabInfo, got %d", m.activeTab)
	}
	if len(m.tabs) != 2 {
		t.Errorf("expected 2 tabs (no Logs for non-Pod), got %d", len(m.tabs))
	}
	if m.tabs[0] != "Relatives" || m.tabs[1] != "Events" {
		t.Errorf("expected tabs=[Relatives, Events], got %v", m.tabs)
	}
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0, got %d", m.scrollOffset)
	}
}

func TestDetailModel_SetDetail(t *testing.T) {
	m := newTestDetail()

	m.SetDetail(sampleDetail(), sampleEvents())

	if !m.hasData {
		t.Error("expected hasData=true after SetDetail")
	}
	if m.detail.Name != "nginx-7b4f6c8d4-abc12" {
		t.Errorf("expected detail.Name=nginx-7b4f6c8d4-abc12, got %s", m.detail.Name)
	}
	if len(m.events) != 3 {
		t.Errorf("expected 3 events, got %d", len(m.events))
	}
	if len(m.contentLines) == 0 {
		t.Error("expected contentLines to be populated after SetDetail")
	}
}

// TestDetailModel_SetDetail_PreservesScrollOnSameUID pins the watcher-tick
// scroll-reset bug fix: SetDetail must keep the user's scroll position
// when the same row is being refreshed (UID match). Without the guard,
// the watcher's ~3s polling tick would snap Logs back to top every cycle
// — most visible on an idle pod where no incoming line arrives to push
// scroll back down, but the same regression silently affected Relatives,
// Events, History scrolling too.
func TestDetailModel_SetDetail_PreservesScrollOnSameUID(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	d := sampleDetail()
	d.UID = "uid-A"
	m.SetDetail(d, nil)
	m.scrollOffset = 5 // user scrolled down

	// Polling refresh: same UID, fresher data
	m.SetDetail(d, nil)
	if m.scrollOffset != 5 {
		t.Errorf("same-UID SetDetail must preserve scrollOffset; want 5, got %d", m.scrollOffset)
	}

	// Row change: different UID resets scroll
	d2 := sampleDetail()
	d2.UID = "uid-B"
	m.SetDetail(d2, nil)
	if m.scrollOffset != 0 {
		t.Errorf("different-UID SetDetail must reset scrollOffset to 0; got %d", m.scrollOffset)
	}
}

func TestDetailModel_SwitchTab(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods) // 4 tabs: Logs, Relatives, Events, Conditions
	m.SetDetail(sampleDetail(), sampleEvents())

	if m.activeTab != 0 {
		t.Fatalf("expected activeTab=0 (Logs), got %d", m.activeTab)
	}
	if m.ActiveTabName() != "Logs" {
		t.Fatalf("expected default tab=Logs for Pod, got %s", m.ActiveTabName())
	}

	// ']' cycles Logs → Relatives
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Relatives" {
		t.Errorf("expected Relatives after first ']', got %s", m.ActiveTabName())
	}

	// ']' cycles Relatives → Events
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Events" {
		t.Errorf("expected Events after second ']', got %s", m.ActiveTabName())
	}

	// ']' cycles Events → Conditions
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Conditions" {
		t.Errorf("expected Conditions after third ']', got %s", m.ActiveTabName())
	}

	// ']' wraps Conditions → Logs
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Logs" {
		t.Errorf("expected Logs after wrap ']', got %s", m.ActiveTabName())
	}

	// '[' wraps Logs → Conditions (backward to last tab)
	m, _ = m.Update(keyMsg('['))
	if m.ActiveTabName() != "Conditions" {
		t.Errorf("expected Conditions after '[' from Logs, got %s", m.ActiveTabName())
	}
}

func TestDetailModel_ScrollDown(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods) // tabs: Logs, Relatives, Events
	m.SetDetail(sampleDetail(), sampleEvents())
	// Logs tab scrolls by line — Relatives tab uses j/k for cursor
	// navigation, so use Logs as the scroll-mechanics testbed.
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	// Pause follow-tail so scrollOffset can move freely.
	m.followTail = false
	m.scrollOffset = 0

	if m.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset=0 initially, got %d", m.scrollOffset)
	}

	// Press 'j' to scroll down.
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after j, got %d", m.scrollOffset)
	}

	// Press 'j' again.
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 2 {
		t.Errorf("expected scrollOffset=2 after second j, got %d", m.scrollOffset)
	}
}

func TestDetailModel_ScrollUp(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m.followTail = false
	m.scrollOffset = 0

	// Scroll down a few lines first.
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 3 {
		t.Fatalf("expected scrollOffset=3, got %d", m.scrollOffset)
	}

	// Press 'k' to scroll up.
	m, _ = m.Update(keyMsg('k'))
	if m.scrollOffset != 2 {
		t.Errorf("expected scrollOffset=2 after k, got %d", m.scrollOffset)
	}

	// Scroll up past 0 — should clamp.
	m, _ = m.Update(keyMsg('k'))
	m, _ = m.Update(keyMsg('k'))
	m, _ = m.Update(keyMsg('k'))
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 at top boundary, got %d", m.scrollOffset)
	}
}

func TestDetailModel_GG(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m.followTail = false
	m.scrollOffset = 0

	// Scroll down several lines.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(keyMsg('j'))
	}
	if m.scrollOffset != 5 {
		t.Fatalf("expected scrollOffset=5, got %d", m.scrollOffset)
	}

	// Press g (first).
	m, _ = m.Update(keyMsg('g'))
	if !m.pendingG {
		t.Fatal("expected pendingG=true after first g")
	}

	// Press g (second) — scrollOffset should go to 0.
	m, _ = m.Update(keyMsg('g'))
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after gg, got %d", m.scrollOffset)
	}
	if m.pendingG {
		t.Error("expected pendingG=false after gg")
	}
}

func TestDetailModel_ShiftG(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m.followTail = false
	m.scrollOffset = 0

	// Press G — scrollOffset should go to max.
	m, _ = m.Update(keyMsg('G'))

	expected := m.maxScrollOffset()
	if m.scrollOffset != expected {
		t.Errorf("expected scrollOffset=%d after G, got %d", expected, m.scrollOffset)
	}
	if expected == 0 {
		t.Error("expected maxScrollOffset > 0 for test to be meaningful")
	}
}

func TestDetailModel_LogsTab_NonPodResource(t *testing.T) {
	m := newTestDetail()
	// Default resourceType is 0 (ResourceNamespaces), not Pods.
	// For non-Pod resources, tabs are ["Detail", "Events"] — no Logs tab.
	if len(m.tabs) != 2 {
		t.Fatalf("expected 2 tabs for non-Pod resource, got %d", len(m.tabs))
	}
	if m.tabs[1] != "Events" {
		t.Errorf("expected second tab to be 'Events', got %q", m.tabs[1])
	}
}

func TestDetailModel_WorkloadKinds_AllGetLogsTab(t *testing.T) {
	// supportsLogs + SetResourceType extended in this iteration to cover
	// every workload kind that k8s.PodsForWorkload routes — StatefulSet,
	// DaemonSet, Job, CronJob — not just Deployment. README claim about
	// "aggregate logs for all workload kinds" used to be ahead of code;
	// this test pins the gate so the next refactor can't silently regress.
	cases := []struct {
		name string
		rt   k8s.ResourceType
	}{
		{"StatefulSet", k8s.ResourceStatefulSets},
		{"DaemonSet", k8s.ResourceDaemonSets},
		{"Job", k8s.ResourceJobs},
		{"CronJob", k8s.ResourceCronJobs},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestDetail()
			m.SetResourceType(c.rt)

			if !supportsLogs(c.rt) {
				t.Errorf("supportsLogs(%v) = false; expected true for workload kind", c.rt)
			}
			if !isAggregateLogsKind(c.rt) {
				t.Errorf("isAggregateLogsKind(%v) = false; expected true for non-Pod workload", c.rt)
			}
			foundLogs := false
			for _, tab := range m.tabs {
				if tab == "Logs" {
					foundLogs = true
					break
				}
			}
			if !foundLogs {
				t.Errorf("tabs %v missing Logs tab for kind %v", m.tabs, c.rt)
			}
		})
	}
}

func TestDetailModel_PodIsNotAggregateLogsKind(t *testing.T) {
	// Pods take the single-pod streaming path in app.go's dispatch, not
	// the aggregate route. The helper must say so explicitly so callers
	// don't accidentally fire PodsForWorkload against a Pod.
	if !supportsLogs(k8s.ResourcePods) {
		t.Error("Pods must supportsLogs (single-stream path)")
	}
	if isAggregateLogsKind(k8s.ResourcePods) {
		t.Error("Pods must NOT be classed as an aggregate-logs kind")
	}
}

func TestDetailModel_Deployment_TabOrderLogsFirst(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceDeployments)
	if len(m.tabs) != 4 {
		t.Fatalf("expected 4 tabs for Deployment (Logs/Relatives/Events/Conditions), got %d (%v)", len(m.tabs), m.tabs)
	}
	wantOrder := []string{"Logs", "Relatives", "Events", "Conditions"}
	for i, want := range wantOrder {
		if m.tabs[i] != want {
			t.Errorf("tab %d: expected %q, got %q", i, want, m.tabs[i])
		}
	}
	if m.activeTab != 0 {
		t.Errorf("Deployment default activeTab must be 0 (Logs), got %d", m.activeTab)
	}
}

func TestDetailModel_AppendLogLine_AggregatePrefix(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceDeployments)
	// Aggregate mode: pod name carries through to the prefix.
	m.AppendLogLine("nginx-abc123-xyz45", "web", "hello from pod1")
	m = m.switchToTab(0) // Logs

	if len(m.contentLines) == 0 {
		t.Fatal("expected log lines rendered")
	}
	// Pod hash tag (last segment) should appear, container name should appear.
	if !strings.Contains(m.contentLines[0], "xyz45") {
		t.Errorf("expected pod-hash tag 'xyz45' in line, got %q", m.contentLines[0])
	}
	if !strings.Contains(m.contentLines[0], "web") {
		t.Errorf("expected container name 'web' in line, got %q", m.contentLines[0])
	}
}

// ── Relatives tab + drill ─────────────────────────────────────────────────

func samplePodRelativesDetail() k8s.ResourceDetail {
	return k8s.ResourceDetail{
		Name:      "nginx-7f9c4d-abc12",
		Namespace: "default",
		Kind:      "Pod",
		PodRelatives: &k8s.PodRelativesData{
			Owner: &k8s.RefTarget{
				Type: k8s.ResourceDeployments, Name: "nginx", Namespace: "default",
			},
			Node:           &k8s.RefTarget{Type: k8s.ResourceNodes, Name: "worker-3"},
			ServiceAccount: &k8s.RefTarget{Type: k8s.ResourceServiceAccounts, Name: "nginx-sa", Namespace: "default"},
			Images:         []string{"nginx:1.27.1"},
		},
	}
}

func TestDetailModel_RelativesTab_RendersDrillableRefs(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1) // Relatives

	joined := strings.Join(m.contentLines, "\n")
	for _, want := range []string{"Owner", "Node", "ServiceAccount", "worker-3", "nginx-sa"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Relatives must contain %q, got:\n%s", want, joined)
		}
	}
	// Strict Relatives: container images are NOT included (not a K8s resource).
	if strings.Contains(joined, "nginx:1.27.1") {
		t.Errorf("Relatives must not include image strings (use Y popup for that), got:\n%s", joined)
	}
}

func TestDetailModel_LinksCursor_LandsOnFirstSelectable(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1) // Relatives

	if m.relativeCursor < 0 || m.relativeCursor >= len(m.relativeEntries) {
		t.Fatalf("cursor out of bounds: %d (entries %d)", m.relativeCursor, len(m.relativeEntries))
	}
	got := m.relativeEntries[m.relativeCursor]
	if !got.isSelectable() {
		t.Errorf("cursor must land on selectable entry, got section header %q", got.label)
	}
	if got.label != "Owner" {
		t.Errorf("first selectable should be Owner, got %q", got.label)
	}
}

func TestDetailModel_LinksCursor_JKMovesBetweenSelectable(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1) // Relatives

	// Initial: Owner
	if m.relativeEntries[m.relativeCursor].label != "Owner" {
		t.Fatalf("setup: cursor expected on Owner, got %q", m.relativeEntries[m.relativeCursor].label)
	}
	// j → Node
	m, _ = m.Update(keyMsg('j'))
	if m.relativeEntries[m.relativeCursor].label != "Node" {
		t.Errorf("after j: expected Node, got %q", m.relativeEntries[m.relativeCursor].label)
	}
	// j → ServiceAccount
	m, _ = m.Update(keyMsg('j'))
	if m.relativeEntries[m.relativeCursor].label != "ServiceAccount" {
		t.Errorf("after j×2: expected ServiceAccount, got %q", m.relativeEntries[m.relativeCursor].label)
	}
	// k → Node
	m, _ = m.Update(keyMsg('k'))
	if m.relativeEntries[m.relativeCursor].label != "Node" {
		t.Errorf("after k: expected Node back, got %q", m.relativeEntries[m.relativeCursor].label)
	}
}

func TestDetailModel_LinksEnter_EmitsPushMsg(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1) // Relatives

	// Cursor on Owner; Enter now drills into the link chain (push), not the
	// YAML popup. Y is the new key for cursor-pointed YAML.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on drillable entry must return a Cmd")
	}
	push, ok := cmd().(RelativePushMsg)
	if !ok {
		t.Fatalf("expected RelativePushMsg, got %T", cmd())
	}
	if push.Ref.Type != k8s.ResourceDeployments || push.Ref.Name != "nginx" {
		t.Errorf("expected push to deployment/nginx, got %v", push.Ref)
	}
}

func TestDetailModel_DrillStack_PushPop(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	if m.Depth() != 1 {
		t.Fatalf("initial depth should be 1, got %d", m.Depth())
	}

	// Drill into the deployment owner.
	depDetail := k8s.ResourceDetail{
		Name: "nginx", Namespace: "default", Kind: "Deployment",
	}
	depRef := k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx", Namespace: "default"}
	depItem := k8s.ResourceItem{Name: "nginx", Namespace: "default", UID: "uid-dep"}
	m.PushDrillFrame(depRef, depItem, depDetail)
	if m.Depth() != 2 {
		t.Errorf("after push, depth should be 2, got %d", m.Depth())
	}
	if m.currentLevelKind() != k8s.ResourceDeployments {
		t.Errorf("current kind should be Deployments after push, got %s", m.currentLevelKind())
	}

	// Pop back to root.
	m.PopDrillFrame()
	if m.Depth() != 1 {
		t.Errorf("after pop, depth should be 1, got %d", m.Depth())
	}
	if m.currentLevelKind() != k8s.ResourcePods {
		t.Errorf("current kind should be Pods at root, got %s", m.currentLevelKind())
	}
}

func TestDetailModel_DrillStack_JumpToLevel(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	for _, name := range []string{"dep", "rs", "cfg"} {
		m.PushDrillFrame(
			k8s.RefTarget{Type: k8s.ResourceDeployments, Name: name, Namespace: "default"},
			k8s.ResourceItem{Name: name, Namespace: "default", UID: "uid-" + name},
			k8s.ResourceDetail{Name: name, Namespace: "default"},
		)
	}
	if m.Depth() != 4 {
		t.Fatalf("expected depth 4, got %d", m.Depth())
	}
	// Jump back to level 2.
	m.JumpToDrillLevel(2)
	if m.Depth() != 2 {
		t.Errorf("after jump, depth should be 2, got %d", m.Depth())
	}
	// Jump to root.
	m.JumpToDrillLevel(1)
	if m.Depth() != 1 {
		t.Errorf("after jump to root, depth should be 1, got %d", m.Depth())
	}
}

// TestDetailModel_DrillStack_PreservedAcrossSetDetail guards a regression
// where the watcher's background refresh would dispatch a fresh
// fetchResourceDetail for the still-selected root row while the user was
// mid-drill. When the result arrived, SetDetail would wipe drillStack and
// snap the user back to level 1 — exactly when their fetch finished, the
// view jumped away from the level they just navigated into.
//
// The row-change path (RowSelectedMsg) handles reset explicitly via
// ResetDrillStack; namespace/context switches go through ClearDetail.
// SetDetail itself must NOT touch the chain.
func TestDetailModel_DrillStack_PreservedAcrossSetDetail(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)
	if m.Depth() != 2 {
		t.Fatalf("setup failed: depth %d", m.Depth())
	}
	// Watcher-driven refresh delivers a new ResourceDetailMsg for the SAME
	// root row. drillStack must survive.
	m.SetDetail(samplePodRelativesDetail(), nil)
	if m.Depth() != 2 {
		t.Errorf("SetDetail must preserve drillStack, got depth %d", m.Depth())
	}
}

// TestDetailModel_DrillStack_ClearedByClearDetail covers the
// namespace/context switch path — different cluster scope means the chain
// no longer points at reachable resources, so it must be torn down.
func TestDetailModel_DrillStack_ClearedByClearDetail(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)
	if m.Depth() != 2 {
		t.Fatalf("setup failed: depth %d", m.Depth())
	}
	m.ClearDetail()
	if m.Depth() != 1 {
		t.Errorf("ClearDetail must reset drillStack, got depth %d", m.Depth())
	}
}

// TestDetailModel_CurrentLevelRef returns root at depth 1, drilled ref at
// depth 2+. Used by the Relatives-tab space hotkey to identify the resource
// the user wants to promote to the table selection.
func TestDetailModel_CurrentLevelRef(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	d := samplePodRelativesDetail()
	d.Name = "pod-x"
	d.Namespace = "ns-a"
	m.SetDetail(d, nil)
	root := m.CurrentLevelRef()
	if root.Type != k8s.ResourcePods || root.Name != "pod-x" || root.Namespace != "ns-a" {
		t.Errorf("root CurrentLevelRef = %+v, want pod-x in ns-a", root)
	}

	drilled := k8s.RefTarget{Type: k8s.ResourceConfigMaps, Name: "cfg-1", Namespace: "ns-a"}
	m.PushDrillFrame(drilled, k8s.ResourceItem{Name: "cfg-1", Namespace: "ns-a"}, k8s.ResourceDetail{})
	if got := m.CurrentLevelRef(); got != drilled {
		t.Errorf("drilled CurrentLevelRef = %+v, want %+v", got, drilled)
	}
}

func TestDetailModel_TabTitle_ShowsLevelWhenDrilled(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1) // Relatives
	if got := m.ActiveTabTitle(); got != "Relatives" {
		t.Errorf("at root, ActiveTabTitle should be 'Relatives', got %q", got)
	}
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)
	want := "Relatives " + relativesDrillArrow + "2"
	if got := m.ActiveTabTitle(); got != want {
		t.Errorf("at depth 2, ActiveTabTitle should be %q, got %q", want, got)
	}
}

// TestDetailModel_RelativesTab_LongValueWrapsConsistently verifies a Relatives
// row whose value (resource name) is too long for the row width wraps
// to multiple display lines — and does so the same way for cursor and
// non-cursor rows, fixing a previous inconsistency where the cursor
// row wrapped (via lipgloss.Width) but non-cursor rows got truncated
// by the outer panel render.
func TestDetailModel_RelativesTab_LongValueWrapsConsistently(t *testing.T) {
	longName := "harbor-registry-htpasswd-very-long-name-here"
	detail := k8s.ResourceDetail{
		Name:      "p",
		Namespace: "ns",
		Kind:      "Pod",
		PodRelatives: &k8s.PodRelativesData{
			Volumes: []k8s.VolumeRef{
				{
					Name: "vol1",
					Kind: "secret",
					Ref:  &k8s.RefTarget{Type: k8s.ResourceSecrets, Name: longName, Namespace: "ns"},
				},
			},
		},
	}
	m := newTestDetail()
	m.SetSize(40, 20) // narrow panel forces wrap
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(detail, nil)
	m = m.switchToTab(1) // Relatives

	joined := strings.Join(m.contentLines, "\n")
	// Wrap broke the long name at character boundary, so the substring
	// won't appear contiguous. Instead, assert both ends of the long
	// name are present — truncation (the regression we're guarding
	// against) would lose the tail.
	head := longName[:10]
	tail := longName[len(longName)-10:]
	if !strings.Contains(joined, head) {
		t.Errorf("start of long name (%q) missing, got:\n%s", head, joined)
	}
	if !strings.Contains(joined, tail) {
		t.Errorf("end of long name (%q) missing — value was truncated, not wrapped:\n%s", tail, joined)
	}
	// Drill arrow must still render after wrap.
	if !strings.Contains(joined, relativesDrillArrow) {
		t.Errorf("drill arrow lost after wrap, got:\n%s", joined)
	}
}

// TestDetailModel_BorderTopRightHint — v1.5.x: hint always returns "".
// `[b]readcrumbs` retired alongside the `b` key.
func TestDetailModel_BorderTopRightHint(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1)

	if got := m.BorderTopRightHint(); got != "" {
		t.Errorf("depth 1 should have no hint, got %q", got)
	}
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)
	if got := m.BorderTopRightHint(); got != "" {
		t.Errorf("depth 2 must also have no hint (retired in v1.5.x), got %q", got)
	}
}

// TestDetailModel_RelativesH_Retired — v1.5.x: `h` no longer pops drill
// frame. `Esc` owns pop; `h`/`l` are panel-3 tab switches (handled at
// app.go layer).
func TestDetailModel_RelativesH_Retired(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1)
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)

	initialDepth := m.Depth()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if updated.Depth() != initialDepth {
		t.Errorf("h must not pop frame (retired), depth changed %d→%d", initialDepth, updated.Depth())
	}
}

// TestDetailModel_RelativesB_Retired — v1.5.x: `b` retired. Space opens
// the breadcrumb popup at the app layer; this handler should not emit
// RelativeBreadcrumbMsg from `b` anymore.
func TestDetailModel_RelativesB_Retired(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1)
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd != nil {
		if _, ok := cmd().(RelativeBreadcrumbMsg); ok {
			t.Errorf("b must NOT emit RelativeBreadcrumbMsg anymore (retired in v1.5.x)")
		}
	}
}

func TestDetailModel_DrillChain_RootFirst(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m.PushDrillFrame(
		k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "nginx", Namespace: "default"},
		k8s.ResourceItem{}, k8s.ResourceDetail{},
	)
	chain := m.DrillChain()
	if len(chain) != 2 {
		t.Fatalf("chain should have 2 entries, got %d", len(chain))
	}
	if chain[0].Type != k8s.ResourcePods || chain[0].Name != "nginx-7f9c4d-abc12" {
		t.Errorf("chain[0] should be root Pod, got %+v", chain[0])
	}
	if chain[1].Type != k8s.ResourceDeployments {
		t.Errorf("chain[1] should be Deployment, got %+v", chain[1])
	}
}

// TestDetailModel_RelativesL_Retired — v1.5.x: `l` no longer drills.
// Enter is the sole drill / focus key under the new mental model.
// `l` now means "next tab" but only when panel 3 is the active panel
// (handled at app.go layer, not detail.Update).
func TestDetailModel_RelativesL_Retired(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodRelativesDetail(), nil)
	m = m.switchToTab(1)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd != nil {
		if _, ok := cmd().(RelativePushMsg); ok {
			t.Errorf("l must NOT emit RelativePushMsg anymore (Enter is sole drill key)")
		}
	}
}

// TestDetailModel_RelativesTab_LastCursorScrollsViewport guards the
// 2026-06-23 bug where pressing j past the last selectable entry froze
// the viewport — trailing contentLines (section spacing, blank rows)
// stayed invisible because j was bound exclusively to nextSelectableCursor
// and never fell through to scrollDown. With many relatives + a tight
// viewport this manifested as "37 of 47" stuck and the last ~10 lines
// of the list unreachable.
func TestDetailModel_RelativesTab_LastCursorScrollsViewport(t *testing.T) {
	// Generic Relatives with a tail of non-selectable rows AFTER the last
	// drillable entry — mirrors the real-world layout that surfaced the
	// bug (Node view: many "Pods on this Node" drillables + a trailing
	// section header with no entries / informational rows). With viewport
	// smaller than total content, scrolling past the last cursor row
	// requires the fallback path.
	rows := make([]k8s.RelativeRow, 0, 30)
	for i := 0; i < 30; i++ {
		rows = append(rows, k8s.RelativeRow{
			Label: fmt.Sprintf("pod-%02d", i),
			Value: fmt.Sprintf("default/nginx-pod-%02d", i),
			Ref: &k8s.RefTarget{
				Type: k8s.ResourcePods, Name: fmt.Sprintf("nginx-pod-%02d", i), Namespace: "default",
			},
		})
	}
	detail := k8s.ResourceDetail{
		Name: "node-1", Kind: "Node",
		Relatives: []k8s.RelativeSection{
			{Title: "Pods on this Node", Entries: rows},
			// Trailing section: header + a non-drillable info row.
			// Both are non-selectable, so cursor cannot reach them
			// — they exist past the last drillable to force the bug.
			{
				Title: "Trailing Info",
				Entries: []k8s.RelativeRow{
					{Label: "note", Value: "informational, no ref"},
				},
			},
		},
	}
	m := newTestDetail()
	m.SetSize(80, 8) // viewport smaller than full content
	m.SetResourceType(k8s.ResourceNodes)
	m.SetDetail(detail, nil)
	m = m.switchToTab(0) // Relatives (Nodes: 2-tab layout, no Logs)

	// Drive cursor to the last selectable entry — stop as soon as the
	// cursor stops advancing (because every j past that point also
	// scrolls the viewport, which would void the test setup).
	for i := 0; i < len(m.relativeEntries)+5; i++ {
		prev := m.relativeCursor
		m, _ = m.Update(keyMsg('j'))
		if m.relativeCursor == prev {
			break
		}
	}
	if !m.relativeEntries[m.relativeCursor].isSelectable() {
		t.Fatalf("setup: cursor must end on a selectable entry, got header %q",
			m.relativeEntries[m.relativeCursor].label)
	}

	pinnedCursor := m.relativeCursor
	offsetBefore := m.scrollOffset
	maxOffset := m.maxScrollOffset()
	if offsetBefore >= maxOffset {
		t.Skipf("setup: viewport already at max scroll (offset=%d max=%d) — bug doesn't manifest here", offsetBefore, maxOffset)
	}

	// Press j once more — cursor is pinned (already at last selectable);
	// fix must let the viewport advance so trailing lines come into view.
	m, _ = m.Update(keyMsg('j'))
	if m.relativeCursor != pinnedCursor {
		t.Errorf("cursor should stay at last selectable when j has nowhere to go; before=%d after=%d", pinnedCursor, m.relativeCursor)
	}
	if m.scrollOffset <= offsetBefore {
		t.Errorf("scrollOffset must advance once cursor is pinned (regression — list stuck); before=%d after=%d max=%d",
			offsetBefore, m.scrollOffset, maxOffset)
	}
}

// TestDetailModel_RelativesTab_EmptyShowsPlaceholder verifies the "no relatives to
// show" placeholder renders for a supported kind whose specific instance
// happens to have no link refs (e.g. ConfigMap with no consumer Pods).
func TestDetailModel_RelativesTab_EmptyShowsPlaceholder(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceConfigMaps) // supported, but instance has no consumers
	m.SetDetail(k8s.ResourceDetail{Name: "x", Namespace: "default", Kind: "ConfigMap"}, nil)
	m = m.switchToTab(0) // Relatives (ConfigMaps: 2-tab layout, no Logs)

	joined := strings.Join(m.contentLines, "\n")
	if !strings.Contains(joined, "no relatives to show") {
		t.Errorf("supported-but-empty Relatives must show 'no relatives to show' placeholder, got:\n%s", joined)
	}
	if strings.Contains(joined, "not yet supported") {
		t.Errorf("supported kind must not show 'not yet supported' placeholder")
	}
}

// TestDetailModel_NamespaceHidesLinksTab verifies the Relatives tab is dropped
// entirely for Namespace — there are no meaningful refs to surface, so the
// tab strip skips straight to Events.
func TestDetailModel_NamespaceHidesLinksTab(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceNamespaces)

	for _, tab := range m.tabs {
		if tab == "Relatives" {
			t.Fatalf("Namespace should not show Relatives tab, got: %v", m.tabs)
		}
	}
	if len(m.tabs) == 0 || m.tabs[0] != "Events" {
		t.Errorf("Namespace tabs should start with Events, got: %v", m.tabs)
	}
}

func TestPodHashTag(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"nginx-abc123-xyz45", "xyz45"},
		{"deploy-789abcdef0-q12pl", "q12pl"},
		{"short", "short"},
		{"no-dash-five", "five"}, // last segment "five" length 4 fits in 5
		{"abcdefgh", "defgh"},    // no dash → last 5 chars
	}
	for _, c := range cases {
		got := podHashTag(c.name)
		if got != c.want {
			t.Errorf("podHashTag(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDetailModel_LogsTab_PodWaiting(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())

	// Switch to Logs tab — no log lines yet.
	m = m.switchToTab(0) // Logs

	if len(m.contentLines) != 1 {
		t.Fatalf("expected 1 content line, got %d", len(m.contentLines))
	}
	if !strings.Contains(m.contentLines[0], "Waiting for logs...") {
		t.Errorf("expected 'Waiting for logs...', got %q", m.contentLines[0])
	}
}

func TestDetailModel_AppendLogLine(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())

	// Append a log line.
	m.AppendLogLine("", "nginx", "hello world")

	if len(m.logLines) != 1 {
		t.Fatalf("expected 1 logLine, got %d", len(m.logLines))
	}
	if m.logLines[0].container != "nginx" {
		t.Errorf("expected container='nginx', got %q", m.logLines[0].container)
	}
	if m.logLines[0].text != "hello world" {
		t.Errorf("expected text='hello world', got %q", m.logLines[0].text)
	}
}

func TestDetailModel_AppendLogLine_WrapsLongText(t *testing.T) {
	m := newTestDetail() // width=80
	m.SetResourceType(k8s.ResourcePods)
	longText := strings.Repeat("foo bar baz ", 20) // ~240 chars, far over 80

	m.AppendLogLine("", "nginx", longText)
	// Storage stores raw — exactly one entry, unwrapped.
	if len(m.logLines) != 1 {
		t.Fatalf("expected 1 raw log entry, got %d", len(m.logLines))
	}

	// Render-time wrap: switch to Logs tab and inspect contentLines.
	m = m.switchToTab(0) // Logs
	if len(m.contentLines) < 2 {
		t.Fatalf("expected long log to wrap to multiple content lines, got %d", len(m.contentLines))
	}
	if !strings.HasPrefix(m.contentLines[0], "  nginx │ ") {
		t.Errorf("first content line must carry container prefix, got %q", m.contentLines[0])
	}
	contIndent := "  " + strings.Repeat(" ", len("nginx")) + " │ "
	if !strings.HasPrefix(m.contentLines[1], contIndent) {
		t.Errorf("continuation line must align under content column, got %q", m.contentLines[1])
	}
}

func TestDetailModel_Logs_ReflowOnResize(t *testing.T) {
	m := newTestDetail() // width=80
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs
	longText := strings.Repeat("foo bar baz ", 20)
	m.AppendLogLine("", "nginx", longText)

	narrowLines := len(m.contentLines)
	if narrowLines < 2 {
		t.Fatalf("expected wrap at width=80, got %d content lines", narrowLines)
	}

	// Expand: width 200 should reduce wrap (fewer or equal continuation lines).
	m.SetSize(200, 20)
	wideLines := len(m.contentLines)
	if wideLines >= narrowLines {
		t.Errorf("expected fewer wrap lines after expand: was %d, now %d", narrowLines, wideLines)
	}
}

func TestDetailModel_AppendLogLine_MaxLines(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.maxLogLines = 10

	for i := 0; i < 15; i++ {
		m.AppendLogLine("", "test", fmt.Sprintf("line %d", i))
	}

	if len(m.logLines) != 10 {
		t.Errorf("expected 10 logLines after trimming, got %d", len(m.logLines))
	}
	// The oldest lines (0-4) should be trimmed.
	if m.logLines[0].text != "line 5" {
		t.Errorf("expected first logLine text='line 5', got %q", m.logLines[0].text)
	}
}

func TestDetailModel_LogsTab_WithLogLines(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())

	m.AppendLogLine("", "nginx", "log entry 1")
	m.AppendLogLine("", "sidecar", "log entry 2")

	// Switch to Logs tab.
	m = m.switchToTab(0) // Logs

	if len(m.contentLines) != 2 {
		t.Fatalf("expected 2 content lines on Logs tab, got %d", len(m.contentLines))
	}
	if !strings.Contains(m.contentLines[0], "nginx") {
		t.Errorf("expected first line to contain 'nginx', got %q", m.contentLines[0])
	}
	if !strings.Contains(m.contentLines[1], "sidecar") {
		t.Errorf("expected second line to contain 'sidecar', got %q", m.contentLines[1])
	}
}

func TestDetailModel_ClearDetail_ClearsLogs(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())
	m.AppendLogLine("", "nginx", "some log")

	if len(m.logLines) == 0 {
		t.Fatal("expected logLines to be non-empty before clear")
	}

	m.ClearDetail()

	if m.logLines != nil {
		t.Errorf("expected logLines=nil after ClearDetail, got %v", m.logLines)
	}
}

func TestWrapPlain(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"empty stays single", "", 10, []string{""}},
		{"shorter than width", "hi", 10, []string{"hi"}},
		{"equal to width", "0123456789", 10, []string{"0123456789"}},
		{"word boundary", "hello world foo", 11, []string{"hello world", "foo"}},
		{"no spaces hard cut", "abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"width zero passthrough", "anything", 0, []string{"anything"}},
		{"width negative passthrough", "anything", -1, []string{"anything"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapPlain(tc.text, tc.width)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d lines, want %d: %q", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDetailModel_EventsMessage_Wraps_NotTruncates(t *testing.T) {
	m := newTestDetail() // width=80
	longMsg := "this is a deliberately very long event message that should wrap to multiple lines rather than being silently truncated with an ellipsis at the end"
	events := []k8s.EventItem{
		{Type: "Warning", Reason: "BackOff", Object: "Pod/x", Message: longMsg, Age: "1m"},
	}
	detail := sampleDetail()
	m.SetDetail(detail, events)
	m = m.switchToTab(DetailTabEvents)

	joined := strings.Join(m.contentLines, "\n")
	if strings.Contains(joined, "…") {
		t.Errorf("expected no ellipsis (wrap not truncate), got:\n%s", joined)
	}
	// The full message text (every word) must appear somewhere in the rendered output.
	for _, word := range []string{"deliberately", "ellipsis"} {
		if !strings.Contains(joined, word) {
			t.Errorf("expected wrapped output to contain %q, got:\n%s", word, joined)
		}
	}
}

// Panel-3 search was removed entirely in the v1.5 polish pass — cursor
// tabs (Relatives / History) didn't tolerate filtering, and the line-
// based tabs (Logs / Events) read better as plain scrollable views. The
// previous TestDetailModel_SearchJKAreTypedNotNavigation test guarded a
// behavior that no longer exists; deletion intentional, not a regression.

// YAML-rendering tests were removed in the Relatives migration — YAML now
// lives in the `Y` popup, covered by yamlpopup_test.go. CopyableContent's
// YAML special-case is gone too; users copy raw YAML from inside the popup.

func TestDetailModel_CopyableContent_StripsANSI(t *testing.T) {
	// Use Events tab — generic Relatives tab returns a placeholder for non-Pod,
	// non-Deployment kinds; Events is a reliable source of styled content.
	m := newTestDetail()
	m.SetDetail(sampleDetail(), sampleEvents())
	m = m.switchToTab(1) // Events (default 2-tab layout: Relatives, Events)

	plain := m.CopyableContent()
	if plain == "" {
		t.Fatal("expected non-empty copyable content")
	}
	if strings.Contains(plain, "\x1b[") {
		t.Errorf("expected no ANSI escapes in copyable content, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Pod/nginx") {
		t.Errorf("expected event object in copyable content, got:\n%s", plain)
	}
}

func TestDetailModel_CopyableContent_EmptyWhenNoData(t *testing.T) {
	m := newTestDetail()
	if got := m.CopyableContent(); got != "" {
		t.Errorf("expected empty content when no data, got %q", got)
	}
}

func TestDetailModel_ClearDetail(t *testing.T) {
	m := newTestDetail()
	m.SetDetail(sampleDetail(), sampleEvents())

	if !m.hasData {
		t.Fatal("expected hasData=true before clear")
	}

	m.ClearDetail()

	if m.hasData {
		t.Error("expected hasData=false after ClearDetail")
	}
	if m.contentLines != nil {
		t.Error("expected contentLines=nil after ClearDetail")
	}
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after ClearDetail, got %d", m.scrollOffset)
	}
}

// ── Follow-tail (Logs auto-scroll) ─────────────────────────────────────────

func TestDetailModel_FollowTail_DefaultOn(t *testing.T) {
	m := newTestDetail()
	if !m.FollowTail() {
		t.Error("expected followTail=true by default")
	}
}

func TestDetailModel_FollowTail_AppendSnapsToBottom(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs

	// Spam enough lines that scroll has somewhere to go.
	for i := 0; i < 100; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}

	if !m.followTail {
		t.Fatal("expected followTail=true on Logs tab by default")
	}
	if m.scrollOffset != m.maxScrollOffset() {
		t.Errorf("expected scroll glued to bottom while following: offset=%d, max=%d", m.scrollOffset, m.maxScrollOffset())
	}
}

func TestDetailModel_FollowTail_AppendDoesNotMoveWhenPaused(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs, followTail=true at bottom

	// Fill some lines, then user scrolls up — disables follow.
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k'))
	if m.followTail {
		t.Fatal("expected scrolling up to disable followTail")
	}
	pausedAt := m.scrollOffset

	// New lines arrive — scroll offset must not change.
	for i := 50; i < 60; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	if m.scrollOffset != pausedAt {
		t.Errorf("expected scroll to stay put while paused: was %d, now %d", pausedAt, m.scrollOffset)
	}
}

func TestDetailModel_FollowTail_ScrollUpDisablesOnLogsOnly(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)

	// Relatives tab: scrollUp must NOT touch followTail.
	m = m.switchToTab(1) // Relatives
	if m.ActiveTabName() != "Relatives" {
		t.Fatalf("expected Relatives active, got %s", m.ActiveTabName())
	}
	m, _ = m.Update(keyMsg('k'))
	if !m.followTail {
		t.Error("scrolling up on Relatives tab must not disable followTail")
	}

	// Logs tab: scrollUp disables.
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k'))
	if m.followTail {
		t.Error("scrolling up on Logs must disable followTail")
	}
}

func TestDetailModel_FollowTail_GReEnables(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k')) // disable follow
	if m.followTail {
		t.Fatal("setup: k must disable followTail")
	}

	// G jumps to bottom AND resumes follow — "catch up + tail" is one action.
	m, _ = m.Update(keyMsg('G'))
	if m.scrollOffset != m.maxScrollOffset() {
		t.Errorf("expected G to jump to bottom, got offset=%d max=%d", m.scrollOffset, m.maxScrollOffset())
	}
	if !m.followTail {
		t.Error("expected G on Logs tab to re-enable followTail")
	}
}

func TestDetailModel_FollowTail_GOutsideLogsDoesNotEnable(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	// Switch off the Logs tab — G on a non-Logs tab must not flip a state
	// that's irrelevant there.
	m = m.switchToTab(2) // Events
	m.followTail = false
	m, _ = m.Update(keyMsg('G'))
	if m.followTail {
		t.Error("G on a non-Logs tab must not flip followTail")
	}
}

func TestDetailModel_FollowTail_TabSwitchResetsToFollow(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k')) // pause
	if m.followTail {
		t.Fatal("setup: k must disable followTail")
	}

	// Leave Logs and return → state resets to follow.
	m = m.switchToTab(1) // Relatives
	m = m.switchToTab(0) // Logs
	if !m.followTail {
		t.Error("re-entering Logs tab must reset followTail to true")
	}
}

// TestDetailModel_TabTitle_LogsFollowGlyph pins the v1.7.x+ live/paused
// glyph contract: active Logs tab carries U+F0753 (mdi-play, live) when
// followTail is true and U+F0754 (mdi-pause, paused) when scrolled up.
// Color was used as the indicator in v1.5–v1.7.2 but conflicted with the
// "color = signal" mindset (color reserved for abnormal status / cursor
// / lock), so the live/paused state is now a Nerd Font glyph instead.
func TestDetailModel_TabTitle_LogsFollowGlyph(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs, followTail starts true

	if !m.FollowTail() {
		t.Fatal("setup: expected followTail=true initially after switching to Logs")
	}
	if !strings.Contains(m.TabTitle(), logsLiveGlyph) {
		t.Errorf("followTail=true: TabTitle must carry the live glyph (%q), got %q",
			logsLiveGlyph, m.TabTitle())
	}
	if strings.Contains(m.TabTitle(), logsPausedGlyph) {
		t.Errorf("followTail=true: TabTitle must NOT carry the paused glyph, got %q", m.TabTitle())
	}

	// Pause via scroll up — glyph flips to paused.
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k'))
	if m.FollowTail() {
		t.Fatal("expected followTail=false after k scroll")
	}
	if !strings.Contains(m.TabTitle(), logsPausedGlyph) {
		t.Errorf("followTail=false: TabTitle must carry the paused glyph (%q), got %q",
			logsPausedGlyph, m.TabTitle())
	}
	if strings.Contains(m.TabTitle(), logsLiveGlyph) {
		t.Errorf("followTail=false: TabTitle must NOT carry the live glyph, got %q", m.TabTitle())
	}
}

// TestDetailModel_TabTitle_LogsGlyphPersistsWhenInactive pins the
// width-stability invariant: the Logs tab carries its live/paused
// glyph regardless of active state. Without this, switching off Logs
// contracts the tab bar by 2 cells and switching back expands it by
// 2 cells, which propagates to panel 3's border and shows as a
// horizontal jitter on every tab change.
func TestDetailModel_TabTitle_LogsGlyphPersistsWhenInactive(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(1) // Relatives — Logs is now INACTIVE

	title := m.TabTitle()
	if !strings.Contains(title, logsLiveGlyph) && !strings.Contains(title, logsPausedGlyph) {
		t.Errorf("inactive Logs tab MUST still carry the follow-tail glyph (width-stability), got %q", title)
	}
}

// TestDetailModel_TabTitle_NoLogsGlyphForKindsWithoutLogsTab guards that the
// glyph only attaches to a Logs tab — non-workload kinds (no Logs tab in
// tabs list) must not have the glyph appear anywhere.
func TestDetailModel_TabTitle_NoLogsGlyphForKindsWithoutLogsTab(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceConfigMaps) // no Logs tab
	m.SetDetail(sampleDetail(), nil)

	title := m.TabTitle()
	if strings.Contains(title, logsLiveGlyph) || strings.Contains(title, logsPausedGlyph) {
		t.Errorf("kind without Logs tab must NOT carry the glyph anywhere, got %q", title)
	}
}

func TestDetailModel_BorderBottomLeftHint_RelativesDrillDepth(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(1) // Relatives — hint logic gates on ActiveTabName()=="Relatives"

	// depth=1 on Relatives → no hint (nothing to pop).
	if got := m.BorderBottomLeftHint(); got != "" {
		t.Errorf("depth=1: expected empty hint, got %q", got)
	}

	// Push a fake drill frame → depth becomes 2 → hint should appear.
	m.drillStack = append(m.drillStack, drillFrame{
		ref:  k8s.RefTarget{Type: k8s.ResourcePods, Name: "x"},
		item: k8s.ResourceItem{Name: "x"},
	})
	if got := m.BorderBottomLeftHint(); got != "esc: back" {
		t.Errorf("depth>1: expected 'esc: back', got %q", got)
	}

	// Switch to a non-Relatives tab → hint must clear even at depth>1
	// (Esc semantics only apply on Relatives).
	m = m.switchToTab(2) // Events
	if got := m.BorderBottomLeftHint(); got != "" {
		t.Errorf("non-Relatives tab: expected empty hint, got %q", got)
	}
}

func TestContainerLogColor_Stable(t *testing.T) {
	// Same name → same color across calls.
	c1 := containerLogColor("nginx")
	c2 := containerLogColor("nginx")
	if c1 != c2 {
		t.Errorf("containerLogColor not stable for nginx: %q vs %q", c1, c2)
	}
}

func TestContainerLogColor_Distinguishes(t *testing.T) {
	// Common sibling container names should not all collapse to one color.
	// Not a guarantee for any specific pair (palette is small), but the set
	// of 4 typical names should land on at least 2 distinct colors.
	names := []string{"nginx", "sidecar", "redis", "envoy"}
	seen := map[lipgloss.Color]bool{}
	for _, n := range names {
		seen[containerLogColor(n)] = true
	}
	if len(seen) < 2 {
		t.Errorf("expected ≥2 distinct colors across %v, got %d", names, len(seen))
	}
}

// TestDetailModel_EventsConditions_DimOnUnfocus pins the v1.7.5 panel-3
// dim-on-unfocus extension for the STATIC tabs: SetFocused must flip the
// rendered styling of Events / Conditions (previously only Relatives /
// History reacted to focus).
//
// Logs is deliberately excluded from this test — Logs is the streaming
// exception (Path C). Its content does NOT change on focus, since dimming
// streaming content would hide the updates the user is glancing for from
// the corner of the eye. See TestDetailModel_Logs_NoDimOnUnfocus for the
// inverse assertion.
//
// For each tab the test asserts:
//
//   - contentLines bytes differ between focused vs unfocused
//     (styling actually changed)
//   - ansi.Strip yields identical plain text
//     (no content lost in the rebuild)
//
// We deliberately don't assert specific colors — that's the existing test
// style (avoid fragility across terminal profiles). The byte-diff is enough
// to catch a "SetFocused doesn't rebuild this tab" regression. The test
// forces lipgloss to TrueColor so colour ANSI is actually emitted in the
// non-TTY `go test` environment (default profile strips it, masking the
// byte diff).
func TestDetailModel_EventsConditions_DimOnUnfocus(t *testing.T) {
	prev := lipgloss.DefaultRenderer().ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
	stripLines := func(lines []string) []string {
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = ansi.Strip(l)
		}
		return out
	}
	assertDim := func(t *testing.T, tab string, focused, unfocused []string) {
		t.Helper()
		if len(focused) != len(unfocused) {
			t.Fatalf("[%s] line count differs: focused=%d unfocused=%d", tab, len(focused), len(unfocused))
		}
		anyChanged := false
		for i := range focused {
			if focused[i] != unfocused[i] {
				anyChanged = true
			}
			if ansi.Strip(focused[i]) != ansi.Strip(unfocused[i]) {
				t.Errorf("[%s] line %d plain text changed on unfocus:\n  focused=  %q\n  unfocused=%q",
					tab, i, ansi.Strip(focused[i]), ansi.Strip(unfocused[i]))
			}
		}
		if !anyChanged {
			t.Errorf("[%s] expected at least one line to change styling on unfocus, none did", tab)
		}
		_ = stripLines // keep helper available for future per-line assertions
	}

	// --- Events tab ---
	{
		m := newTestDetail()
		m.SetResourceType(k8s.ResourcePods)
		m.SetDetail(sampleDetail(), sampleEvents())
		m = m.switchToTab(2) // Events
		focused := append([]string(nil), m.contentLines...)
		m.SetFocused(false)
		assertDim(t, "Events", focused, m.contentLines)
	}

	// --- Conditions tab ---
	{
		m := newTestDetail()
		m.SetResourceType(k8s.ResourcePods)
		d := sampleDetail()
		d.Conditions = []k8s.ConditionItem{
			{Type: "Ready", Status: "True", Reason: "", Message: "", Age: "1m"},
			{Type: "PodScheduled", Status: "False", Reason: "Unschedulable", Message: "0/3 nodes available", Age: "2m"},
		}
		m.SetDetail(d, sampleEvents())
		m = m.switchToTab(3) // Conditions
		focused := append([]string(nil), m.contentLines...)
		m.SetFocused(false)
		assertDim(t, "Conditions", focused, m.contentLines)
	}
}

// TestDetailModel_Logs_NoDimOnUnfocus pins the v1.7.5 Path-C decision:
// Logs is the streaming exception and must render IDENTICALLY whether
// focused or not. Dimming streaming content would hide log lines
// arriving from the corner of the eye — the whole point of having
// Logs visible across panel focus changes is the glance. Other static
// panel-3 tabs (Events / Conditions / Relatives / History) DO dim
// per TestDetailModel_EventsConditions_DimOnUnfocus.
func TestDetailModel_Logs_NoDimOnUnfocus(t *testing.T) {
	prev := lipgloss.DefaultRenderer().ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())
	m.AppendLogLine("pod-abcdef", "nginx", "log entry 1")
	m.AppendLogLine("pod-abcdef", "sidecar", "log entry 2")
	m = m.switchToTab(0) // Logs
	focused := append([]string(nil), m.contentLines...)
	m.SetFocused(false)
	for i := range focused {
		if focused[i] != m.contentLines[i] {
			t.Errorf("Logs line %d changed on unfocus — Logs must be identical regardless of focus (Path C exception)\n  focused=  %q\n  unfocused=%q",
				i, focused[i], m.contentLines[i])
		}
	}
}

