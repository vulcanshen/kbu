package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestYamlPopup() YamlPopupModel {
	t := theme.DefaultTheme()
	m := NewYamlPopupModel(t)
	m.SetSize(120, 40)
	return m
}

const sampleYAML = `apiVersion: v1
kind: Pod
metadata:
  name: nginx-789abc
  namespace: default
  labels:
    app: nginx
    version: 1.27.1
spec:
  containers:
  - name: nginx
    image: nginx:1.27.1
    ports:
    - containerPort: 80
  - name: sidecar
    image: busybox:latest
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"
  - type: ContainersReady
    status: "True"`

func openTestPopup(m YamlPopupModel, yaml string) YamlPopupModel {
	item := k8s.ResourceItem{Name: "nginx-789abc", Namespace: "default"}
	m.Open(yaml, k8s.ResourcePods, item, "test-ctx")
	m.animator.Finalize()
	return m
}

func TestYamlPopup_InitialState(t *testing.T) {
	m := newTestYamlPopup()
	if m.IsActive() {
		t.Error("expected popup to be inactive initially")
	}
	if m.View() != "" {
		t.Error("expected empty view when inactive")
	}
}

func TestYamlPopup_OpenActivates(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	if !m.IsActive() {
		t.Error("expected popup to be active after Open")
	}
	if !m.IsInteractive() {
		t.Error("expected popup to be interactive after animator finalize")
	}
}

func TestYamlPopup_RenderContainsResourceTitle(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	view := m.RenderPopup()
	if !strings.Contains(view, "pod/nginx-789abc") {
		t.Errorf("expected popup to show pod/nginx-789abc in title, got:\n%s", view)
	}
	if !strings.Contains(view, "default") {
		t.Error("expected popup to show namespace in title")
	}
}

func TestYamlPopup_ScrollJK(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10) // small height → force scrollable content
	m = openTestPopup(m, sampleYAML)
	if m.ScrollOffset() != 0 {
		t.Fatalf("expected initial scrollOffset=0, got %d", m.ScrollOffset())
	}

	m, _ = m.Update(keyMsg('j'))
	if m.ScrollOffset() != 1 {
		t.Errorf("expected scrollOffset=1 after j, got %d", m.ScrollOffset())
	}

	m, _ = m.Update(keyMsg('k'))
	if m.ScrollOffset() != 0 {
		t.Errorf("expected scrollOffset=0 after k, got %d", m.ScrollOffset())
	}

	// k at top stays at 0
	m, _ = m.Update(keyMsg('k'))
	if m.ScrollOffset() != 0 {
		t.Errorf("expected scrollOffset=0 at top, got %d", m.ScrollOffset())
	}
}

func TestYamlPopup_PageScroll(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('d'))
	if m.ScrollOffset() == 0 {
		t.Error("expected scrollOffset > 0 after d page-down")
	}
	prev := m.ScrollOffset()

	m, _ = m.Update(keyMsg('u'))
	if m.ScrollOffset() >= prev {
		t.Errorf("expected scrollOffset to decrease after u; was %d, now %d", prev, m.ScrollOffset())
	}
}

func TestYamlPopup_GotoBottomAndTop(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('G'))
	if m.ScrollOffset() == 0 {
		t.Error("expected scrollOffset > 0 after G")
	}

	// gg: needs two g's
	m, _ = m.Update(keyMsg('g'))
	m, _ = m.Update(keyMsg('g'))
	if m.ScrollOffset() != 0 {
		t.Errorf("expected scrollOffset=0 after gg, got %d", m.ScrollOffset())
	}
}

func TestYamlPopup_SearchFlow(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	// Open search
	m, _ = m.Update(keyMsg('/'))
	if !m.IsSearching() {
		t.Fatal("expected searching mode after /")
	}

	// Type "image"
	for _, r := range "image" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.SearchQuery() != "image" {
		t.Errorf("expected query=image, got %q", m.SearchQuery())
	}

	// Commit with Enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsSearching() {
		t.Error("expected searching mode off after Enter")
	}
	if m.MatchCount() == 0 {
		t.Fatal("expected at least one match for 'image' in sample YAML")
	}

	// Two matches in sample (nginx + busybox); n cycles
	firstCursor := m.MatchCursor()
	m, _ = m.Update(keyMsg('n'))
	if m.MatchCursor() == firstCursor && m.MatchCount() > 1 {
		t.Errorf("expected match cursor to advance after n, stayed at %d", firstCursor)
	}
}

func TestYamlPopup_SearchBackspace(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('/'))
	for _, r := range "img" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.SearchQuery() != "im" {
		t.Errorf("expected query=im after backspace, got %q", m.SearchQuery())
	}
}

func TestYamlPopup_EditEmitsStartEditMsg(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	_, cmd := m.Update(keyMsg('e'))
	if cmd == nil {
		t.Fatal("expected non-nil cmd from e key")
	}
	msg := cmd()
	// Cmd returns tea.Batch result — for the e path it returns nil due to
	// the closure firing both close + startEditMsg as a batch. Walk the batch.
	// tea.Batch returns a tea.BatchMsg which is a slice of cmds — we need to
	// find the inner startEditMsg.
	foundEdit := false
	switch v := msg.(type) {
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			inner := c()
			if _, ok := inner.(startEditMsg); ok {
				foundEdit = true
				break
			}
		}
	case startEditMsg:
		foundEdit = true
	}
	if !foundEdit {
		t.Errorf("expected startEditMsg in cmd output, got %T", msg)
	}
}

func TestYamlPopup_EditNoOpWithoutItem(t *testing.T) {
	m := newTestYamlPopup()
	// Open with empty item (drill-down container case)
	m.Open(sampleYAML, k8s.ResourcePods, k8s.ResourceItem{}, "test-ctx")
	m.animator.Finalize()

	_, cmd := m.Update(keyMsg('e'))
	if cmd != nil {
		t.Errorf("expected no cmd when item.Name empty, got %T", cmd())
	}
}

func TestYamlPopup_CloseWithEsc(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected popup to be inactive after Esc")
	}
}

func TestYamlPopup_CloseWithQ(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(keyMsg('q'))
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected popup to be inactive after q")
	}
}

func TestYamlPopup_SearchLockedStateRenders(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	// Open search, type, commit
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "image" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsSearching() {
		t.Fatal("post-Enter: expected IsSearching()=false (filter locked, not editing)")
	}
	if m.SearchQuery() == "" {
		t.Fatal("post-Enter: expected SearchQuery to persist")
	}
	if m.MatchCount() == 0 {
		t.Fatal("post-Enter: expected matches")
	}

	view := m.RenderPopup()
	if !strings.Contains(view, "image") {
		t.Error("locked-state render must still surface the committed query")
	}
}

func TestYamlPopup_CurrentMatchRowHighlighted(t *testing.T) {
	// Open popup, commit a search, render. Strip ANSI from output and ensure
	// the current-match line is present in plain form (full-row highlight uses
	// a background which leaves the plain content intact).
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx:1.27" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.MatchCount() == 0 {
		t.Fatal("expected at least one match for 'nginx:1.27'")
	}

	view := m.RenderPopup()
	if !strings.Contains(view, "nginx:1.27.1") {
		t.Error("rendered popup must contain the matched image string on the highlighted line")
	}
}

func TestYamlPopup_EmptyYAMLDoesNotCrash(t *testing.T) {
	m := newTestYamlPopup()
	m.Open("", k8s.ResourcePods, k8s.ResourceItem{Name: "x"}, "ctx")
	m.animator.Finalize()
	// Drive a few keys
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('G'))
	m, _ = m.Update(keyMsg('/'))
	// Render should not panic
	view := m.RenderPopup()
	if view == "" {
		t.Error("expected non-empty render even for empty YAML")
	}
}
