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
	if m.tabs[0] != "Detail" || m.tabs[1] != "Events" {
		t.Errorf("expected tabs=[Detail, Events], got %v", m.tabs)
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
	if m.ActiveTabName() != "Detail" {
		t.Errorf("expected Detail after wrap ']', got %s", m.ActiveTabName())
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
	if !strings.Contains(m.logLines[0], "nginx") {
		t.Errorf("expected logLine to contain container name 'nginx', got %q", m.logLines[0])
	}
	if !strings.Contains(m.logLines[0], "hello world") {
		t.Errorf("expected logLine to contain text 'hello world', got %q", m.logLines[0])
	}
	// Check separator character.
	if !strings.Contains(m.logLines[0], "│") {
		t.Errorf("expected logLine to contain separator, got %q", m.logLines[0])
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
	if !strings.Contains(m.logLines[0], "line 5") {
		t.Errorf("expected first logLine to be 'line 5', got %q", m.logLines[0])
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
