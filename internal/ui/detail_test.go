package ui

import (
	"fmt"
	"strings"
	"testing"

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
	if m.tabs[0] != "YAML" || m.tabs[1] != "Events" {
		t.Errorf("expected tabs=[YAML, Events], got %v", m.tabs)
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
	m.SetResourceType(k8s.ResourcePods) // 3 tabs: Detail, Logs, Events
	m.SetDetail(sampleDetail(), sampleEvents())

	if m.activeTab != 0 {
		t.Fatalf("expected activeTab=0 (Detail), got %d", m.activeTab)
	}

	// ']' cycles Detail → Logs
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Logs" {
		t.Errorf("expected Logs after first ']', got %s", m.ActiveTabName())
	}

	// ']' cycles Logs → Events
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "Events" {
		t.Errorf("expected Events after second ']', got %s", m.ActiveTabName())
	}

	// ']' wraps Events → Detail
	m, _ = m.Update(keyMsg(']'))
	if m.ActiveTabName() != "YAML" {
		t.Errorf("expected YAML after wrap ']', got %s", m.ActiveTabName())
	}

	// '[' wraps Detail → Events (backward)
	m, _ = m.Update(keyMsg('['))
	if m.ActiveTabName() != "Events" {
		t.Errorf("expected Events after '[' from Detail, got %s", m.ActiveTabName())
	}

	// '[' cycles Events → Logs
	m, _ = m.Update(keyMsg('['))
	if m.ActiveTabName() != "Logs" {
		t.Errorf("expected Logs after '[' from Events, got %s", m.ActiveTabName())
	}

	// '[' cycles Logs → Detail
	m, _ = m.Update(keyMsg('['))
	if m.activeTab != DetailTabInfo {
		t.Errorf("expected activeTab=DetailTabInfo after '[' from Events, got %d", m.activeTab)
	}
}

func TestDetailModel_ScrollDown(t *testing.T) {
	m := newTestDetail()
	// Generate enough content to scroll.
	detail := sampleDetail()
	detail.Labels = make(map[string]string)
	for i := 0; i < 30; i++ {
		detail.Labels[fmt.Sprintf("label-%02d", i)] = fmt.Sprintf("value-%02d", i)
	}
	m.SetDetail(detail, sampleEvents())

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
	detail := sampleDetail()
	detail.Labels = make(map[string]string)
	for i := 0; i < 30; i++ {
		detail.Labels[fmt.Sprintf("label-%02d", i)] = fmt.Sprintf("value-%02d", i)
	}
	m.SetDetail(detail, sampleEvents())

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
	detail := sampleDetail()
	detail.Labels = make(map[string]string)
	for i := 0; i < 30; i++ {
		detail.Labels[fmt.Sprintf("label-%02d", i)] = fmt.Sprintf("value-%02d", i)
	}
	m.SetDetail(detail, sampleEvents())

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
	detail := sampleDetail()
	detail.Labels = make(map[string]string)
	for i := 0; i < 30; i++ {
		detail.Labels[fmt.Sprintf("label-%02d", i)] = fmt.Sprintf("value-%02d", i)
	}
	m.SetDetail(detail, sampleEvents())

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

func TestDetailModel_LogsTab_PodWaiting(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), sampleEvents())

	// Switch to Logs tab — no log lines yet.
	m = m.switchToTab(1) // Logs tab for Pods is index 1

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
	m.AppendLogLine("nginx", "hello world")

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

	m.AppendLogLine("nginx", longText)
	// Storage stores raw — exactly one entry, unwrapped.
	if len(m.logLines) != 1 {
		t.Fatalf("expected 1 raw log entry, got %d", len(m.logLines))
	}

	// Render-time wrap: switch to Logs tab and inspect contentLines.
	m = m.switchToTab(1)
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
	m = m.switchToTab(1) // Logs
	longText := strings.Repeat("foo bar baz ", 20)
	m.AppendLogLine("nginx", longText)

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
		m.AppendLogLine("test", fmt.Sprintf("line %d", i))
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

	m.AppendLogLine("nginx", "log entry 1")
	m.AppendLogLine("sidecar", "log entry 2")

	// Switch to Logs tab.
	m = m.switchToTab(1) // Logs tab for Pods is index 1

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
	m.AppendLogLine("nginx", "some log")

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

func TestDetailModel_BuildInfo_YAMLPath(t *testing.T) {
	m := newTestDetail()
	d := sampleDetail()
	d.YAML = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx"
	m.SetDetail(d, nil)

	joined := strings.Join(m.contentLines, "\n")
	if !strings.Contains(joined, "apiVersion: v1") {
		t.Errorf("expected YAML to be rendered, got:\n%s", joined)
	}
	if !strings.Contains(joined, "kind: Pod") {
		t.Errorf("expected YAML kind line, got:\n%s", joined)
	}
}

func TestDetailModel_CopyableContent_YAMLReturnsRawWhenNoSearch(t *testing.T) {
	m := newTestDetail()
	d := sampleDetail()
	d.YAML = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx\n"
	m.SetDetail(d, nil)

	got := m.CopyableContent()
	want := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx"
	if got != want {
		t.Errorf("expected raw YAML for copy, got:\n%s", got)
	}
}

func TestDetailModel_CopyableContent_YAMLFallsBackToFilteredWhenSearching(t *testing.T) {
	m := newTestDetail()
	d := sampleDetail()
	d.YAML = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx"
	m.SetDetail(d, nil)
	m.searchQuery = "kind"

	got := m.CopyableContent()
	if !strings.Contains(got, "kind: Pod") {
		t.Errorf("expected filtered output to include kind line, got:\n%s", got)
	}
	if strings.Contains(got, "apiVersion") {
		t.Errorf("expected non-matching lines to be filtered out, got:\n%s", got)
	}
}

func TestDetailModel_CopyableContent_StripsANSI(t *testing.T) {
	m := newTestDetail()
	m.SetDetail(sampleDetail(), sampleEvents())

	plain := m.CopyableContent()
	if plain == "" {
		t.Fatal("expected non-empty copyable content")
	}
	if strings.Contains(plain, "\x1b[") {
		t.Errorf("expected no ANSI escapes in copyable content, got:\n%s", plain)
	}
	if !strings.Contains(plain, "nginx-7b4f6c8d4-abc12") {
		t.Errorf("expected pod name in copyable content, got:\n%s", plain)
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
	m = m.switchToTab(1) // Logs

	// Spam enough lines that scroll has somewhere to go.
	for i := 0; i < 100; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
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
	m = m.switchToTab(1) // Logs, followTail=true at bottom

	// Fill some lines, then user scrolls up — disables follow.
	for i := 0; i < 50; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k'))
	if m.followTail {
		t.Fatal("expected scrolling up to disable followTail")
	}
	pausedAt := m.scrollOffset

	// New lines arrive — scroll offset must not change.
	for i := 50; i < 60; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
	}
	if m.scrollOffset != pausedAt {
		t.Errorf("expected scroll to stay put while paused: was %d, now %d", pausedAt, m.scrollOffset)
	}
}

func TestDetailModel_FollowTail_ScrollUpDisablesOnLogsOnly(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)

	// YAML tab: scrollUp must NOT touch followTail (out-of-context).
	if m.ActiveTabName() != "YAML" {
		t.Fatalf("expected YAML active, got %s", m.ActiveTabName())
	}
	m, _ = m.Update(keyMsg('k'))
	if !m.followTail {
		t.Error("scrolling up on YAML tab must not disable followTail")
	}

	// Logs tab: scrollUp disables.
	m = m.switchToTab(1)
	for i := 0; i < 50; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
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
	m = m.switchToTab(1)
	for i := 0; i < 50; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
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
	// YAML tab — followTail should remain whatever it is; we don't want G on
	// a non-Logs tab to flip a state that's irrelevant there.
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
	m = m.switchToTab(1) // Logs
	for i := 0; i < 50; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
	}
	m, _ = m.Update(keyMsg('k')) // pause
	if m.followTail {
		t.Fatal("setup: k must disable followTail")
	}

	// Leave Logs and return → state resets to follow.
	m = m.switchToTab(0) // YAML
	m = m.switchToTab(1) // Logs
	if !m.followTail {
		t.Error("re-entering Logs tab must reset followTail to true")
	}
}

func TestDetailModel_ActiveTabTitle_FollowMarker(t *testing.T) {
	m := newTestDetail()
	m.SetResourceType(k8s.ResourcePods)
	m.SetDetail(sampleDetail(), nil)
	m = m.switchToTab(1) // Logs, followTail=true

	if got := m.ActiveTabTitle(); got != "Logs ▼" {
		t.Errorf("expected active tab title 'Logs ▼' when following, got %q", got)
	}

	// Pause via scroll up.
	for i := 0; i < 50; i++ {
		m.AppendLogLine("nginx", fmt.Sprintf("line %d", i))
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
