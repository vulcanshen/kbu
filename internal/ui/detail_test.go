package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
	if m.tabs[0] != "Links" || m.tabs[1] != "Events" {
		t.Errorf("expected tabs=[Links, Events], got %v", m.tabs)
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

func TestDetailModel_SwitchTab(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods) // 3 tabs: Logs, Links, Events
	m.SetDetail(sampleDetail(), sampleEvents())

	if m.activeTab != 0 {
		t.Fatalf("expected activeTab=0 (Logs), got %d", m.activeTab)
	}
	if m.ActiveTabName() != "Logs" {
		t.Fatalf("expected default tab=Logs for Pod, got %s", m.ActiveTabName())
	}

	// ']' cycles Logs → Links
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Links" {
		t.Errorf("expected Links after first ']', got %s", m.ActiveTabName())
	}

	// ']' cycles Links → Events
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Events" {
		t.Errorf("expected Events after second ']', got %s", m.ActiveTabName())
	}

	// ']' wraps Events → Logs
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Logs" {
		t.Errorf("expected Logs after wrap ']', got %s", m.ActiveTabName())
	}

	// '[' wraps Logs → Events (backward)
	m, _ = m.Update(keyMsg('['))
	if m.ActiveTabName() != "Events" {
		t.Errorf("expected Events after '[' from Logs, got %s", m.ActiveTabName())
	}
}

func TestDetailModel_ScrollDown(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods) // gives us Logs tab at index 0
	m.SetDetail(sampleDetail(), sampleEvents())
	// Logs tab scrolls by line — Links tab uses j/k for cursor navigation,
	// so use Logs as the scroll-mechanics testbed.
	m = m.switchToTab(0)
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

func TestDetailModel_Deployment_TabOrderLogsFirst(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceDeployments)
	if len(m.tabs) != 3 {
		t.Fatalf("expected 3 tabs for Deployment, got %d (%v)", len(m.tabs), m.tabs)
	}
	wantOrder := []string{"Logs", "Links", "Events"}
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
	m = m.switchToTab(0) // Logs is first for Deployment

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

// ── Links tab + drill ─────────────────────────────────────────────────

func samplePodLinksDetail() k8s.ResourceDetail {
	return k8s.ResourceDetail{
		Name:      "nginx-7f9c4d-abc12",
		Namespace: "default",
		Kind:      "Pod",
		PodLinks: &k8s.PodLinksData{
			Owner: &k8s.RefTarget{
				Type: k8s.ResourceDeployments, Name: "nginx", Namespace: "default",
			},
			Node:           &k8s.RefTarget{Type: k8s.ResourceNodes, Name: "worker-3"},
			ServiceAccount: &k8s.RefTarget{Type: k8s.ResourceServiceAccounts, Name: "nginx-sa", Namespace: "default"},
			Images:         []string{"nginx:1.27.1"},
		},
	}
}

func TestDetailModel_LinksTab_RendersDrillableRefs(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodLinksDetail(), nil)
	m = m.switchToTab(1) // Links

	joined := strings.Join(m.contentLines, "\n")
	for _, want := range []string{"Owner", "Node", "ServiceAccount", "worker-3", "nginx-sa"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Links must contain %q, got:\n%s", want, joined)
		}
	}
	// Strict Links: container images are NOT included (not a K8s resource).
	if strings.Contains(joined, "nginx:1.27.1") {
		t.Errorf("Links must not include image strings (use Y popup for that), got:\n%s", joined)
	}
}

func TestDetailModel_LinksCursor_LandsOnFirstSelectable(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodLinksDetail(), nil)
	m = m.switchToTab(1)

	if m.linkCursor < 0 || m.linkCursor >= len(m.linkEntries) {
		t.Fatalf("cursor out of bounds: %d (entries %d)", m.linkCursor, len(m.linkEntries))
	}
	got := m.linkEntries[m.linkCursor]
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
	m.SetDetail(samplePodLinksDetail(), nil)
	m = m.switchToTab(1)

	// Initial: Owner
	if m.linkEntries[m.linkCursor].label != "Owner" {
		t.Fatalf("setup: cursor expected on Owner, got %q", m.linkEntries[m.linkCursor].label)
	}
	// j → Node
	m, _ = m.Update(keyMsg('j'))
	if m.linkEntries[m.linkCursor].label != "Node" {
		t.Errorf("after j: expected Node, got %q", m.linkEntries[m.linkCursor].label)
	}
	// j → ServiceAccount
	m, _ = m.Update(keyMsg('j'))
	if m.linkEntries[m.linkCursor].label != "ServiceAccount" {
		t.Errorf("after j×2: expected ServiceAccount, got %q", m.linkEntries[m.linkCursor].label)
	}
	// k → Node
	m, _ = m.Update(keyMsg('k'))
	if m.linkEntries[m.linkCursor].label != "Node" {
		t.Errorf("after k: expected Node back, got %q", m.linkEntries[m.linkCursor].label)
	}
}

func TestDetailModel_LinksEnter_EmitsDrillMsg(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(samplePodLinksDetail(), nil)
	m = m.switchToTab(1)

	// Cursor on Owner; Enter must emit LinkDrillMsg with the Owner ref.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on drillable entry must return a Cmd")
	}
	drill, ok := cmd().(LinkDrillMsg)
	if !ok {
		t.Fatalf("expected LinkDrillMsg, got %T", cmd())
	}
	if drill.Ref.Type != k8s.ResourceDeployments || drill.Ref.Name != "nginx" {
		t.Errorf("expected drill to deployment/nginx, got %v", drill.Ref)
	}
}

// TestDetailModel_LinksTab_EmptyShowsPlaceholder verifies the "no links to
// show" placeholder renders for a supported kind whose specific instance
// happens to have no link refs (e.g. ConfigMap with no consumer Pods).
func TestDetailModel_LinksTab_EmptyShowsPlaceholder(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceConfigMaps) // supported, but instance has no consumers
	m.SetDetail(k8s.ResourceDetail{Name: "x", Namespace: "default", Kind: "ConfigMap"}, nil)
	m = m.switchToTab(0) // Links

	joined := strings.Join(m.contentLines, "\n")
	if !strings.Contains(joined, "no links to show") {
		t.Errorf("supported-but-empty Links must show 'no links to show' placeholder, got:\n%s", joined)
	}
	if strings.Contains(joined, "not yet supported") {
		t.Errorf("supported kind must not show 'not yet supported' placeholder")
	}
}

// TestDetailModel_NamespaceHidesLinksTab verifies the Links tab is dropped
// entirely for Namespace — there are no meaningful refs to surface, so the
// tab strip skips straight to Events.
func TestDetailModel_NamespaceHidesLinksTab(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourceNamespaces)

	for _, tab := range m.tabs {
		if tab == "Links" {
			t.Fatalf("Namespace should not show Links tab, got: %v", m.tabs)
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
	m = m.switchToTab(0) // Logs tab for Pods is index 0 (was 1 before YAML→Y popup migration)

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
	m = m.switchToTab(0)
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
	m = m.switchToTab(0) // Logs tab for Pods is index 0 (was 1 before YAML→Y popup migration)

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

func TestDetailModel_SearchJKAreTypedNotNavigation(t *testing.T) {
	m := newTestDetail()
	detail := sampleDetail()
	// Add enough labels to make content scrollable.
	detail.Labels = make(map[string]string)
	for i := 0; i < 30; i++ {
		detail.Labels[fmt.Sprintf("label-%02d", i)] = fmt.Sprintf("value-%02d", i)
	}
	m.SetDetail(detail, sampleEvents())

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('k'))

	if m.scrollOffset != 0 {
		t.Errorf("j/k in search must not scroll, got scrollOffset=%d", m.scrollOffset)
	}
	if m.searchQuery != "jk" {
		t.Errorf("j/k in search must be typed, got query %q", m.searchQuery)
	}
}

// YAML-rendering tests were removed in the Links migration — YAML now
// lives in the `Y` popup, covered by yamlpopup_test.go. CopyableContent's
// YAML special-case is gone too; users copy raw YAML from inside the popup.

func TestDetailModel_CopyableContent_StripsANSI(t *testing.T) {
	// Use Events tab — generic Links tab returns a placeholder for non-Pod,
	// non-Deployment kinds; Events is a reliable source of styled content.
	m := newTestDetail()
	m.SetDetail(sampleDetail(), sampleEvents())
	m = m.switchToTab(1) // Events (index 1 for default 2-tab layout)

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

	// Links tab (index 1): scrollUp must NOT touch followTail.
	m = m.switchToTab(1)
	if m.ActiveTabName() != "Links" {
		t.Fatalf("expected Links active, got %s", m.ActiveTabName())
	}
	m, _ = m.Update(keyMsg('k'))
	if !m.followTail {
		t.Error("scrolling up on Links tab must not disable followTail")
	}

	// Logs tab (index 0): scrollUp disables.
	m = m.switchToTab(0)
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
	m = m.switchToTab(0)
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
	m = m.switchToTab(1) // Links
	m = m.switchToTab(0) // Logs
	if !m.followTail {
		t.Error("re-entering Logs tab must reset followTail to true")
	}
}

func TestDetailModel_ActiveTabTitle_FollowMarker(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(0) // Logs, followTail=true

	if got := m.ActiveTabTitle(); got != "Logs ▼" {
		t.Errorf("expected active tab title 'Logs ▼' when following, got %q", got)
	}

	// Pause via scroll up.
	for i := 0; i < 50; i++ {
		m.AppendLogLine("", "nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k'))
	if got := m.ActiveTabTitle(); got != "Logs" {
		t.Errorf("expected active tab title 'Logs' when paused, got %q", got)
	}

	// The multi-tab bar (TabTitle, Panel 2) must NOT carry the marker —
	// it now only reflects which tab is active, not Logs state.
	if strings.Contains(m.TabTitle(), "▼") {
		t.Errorf("TabTitle (Panel 2 tab bar) must not carry follow marker, got %q", m.TabTitle())
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

func TestDetailModel_SpinnerLifecycle(t *testing.T) {
	m := newTestDetail()

	if m.IsRefetching() {
		t.Fatal("new model must not be refetching")
	}
	if got := m.SpinnerSuffix(); got != "" {
		t.Errorf("SpinnerSuffix when not refetching = %q, want empty", got)
	}

	cmd := m.BeginRefetch()
	if cmd == nil {
		t.Fatal("BeginRefetch must return a tick Cmd")
	}
	if !m.IsRefetching() {
		t.Error("after BeginRefetch, IsRefetching must be true")
	}
	if got := m.SpinnerSuffix(); got == "" {
		t.Error("SpinnerSuffix while refetching must be non-empty")
	}

	// SetDetail clears the spinner.
	m.SetDetail(k8s.ResourceDetail{}, nil)
	if m.IsRefetching() {
		t.Error("SetDetail must clear refetching")
	}
	if got := m.SpinnerSuffix(); got != "" {
		t.Errorf("SpinnerSuffix after SetDetail = %q, want empty", got)
	}

	// BeginRefetch is idempotent — second call returns nil.
	_ = m.BeginRefetch()
	if c := m.BeginRefetch(); c != nil {
		t.Error("BeginRefetch while already refetching must return nil")
	}
}
