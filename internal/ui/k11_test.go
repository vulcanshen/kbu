package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// sidebarLine is the first line of panel 1 that holds text.
func sidebarLine(t *testing.T, view, text string) string {
	t.Helper()
	for _, l := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(l, text) {
			return l
		}
	}
	t.Fatalf("panel 1 has no %q line", text)
	return ""
}

// The drag mode names itself in panel 1, since the footer holds keys only
// (tdp M5): the Pinned title reads [D]rag mode and the drag handle takes
// the first cell of the dragged row. Both go when the drag ends; every
// row stays the panel's width.
func TestK11_DragModeShowsInPanel1(t *testing.T) {
	m := dragApp(t)
	m.sidebar.SetSize(30, 20)
	view := m.sidebar.View()
	if got := sidebarLine(t, view, "Pinned"); !strings.HasPrefix(got, "Pinned [D]rag mode") || strings.Contains(got, dragHandleGlyph) {
		t.Errorf("the dragging Pinned title is %q, want Pinned [D]rag mode", got)
	}
	if got := sidebarLine(t, view, "Pods"); !strings.HasPrefix(got, dragHandleGlyph+" Pods") {
		t.Errorf("the dragged row is %q, want the drag handle in its first cell", got)
	}
	if got := sidebarLine(t, view, "Deployments"); !strings.HasPrefix(got, "  Deployments") {
		t.Errorf("a row not being dragged is %q, want its plain gap", got)
	}
	for _, l := range strings.Split(ansi.Strip(view), "\n") {
		if w := ansi.StringWidth(l); w != 30 {
			t.Errorf("row %q is %d wide, want 30", l, w)
		}
	}

	updated, _ := m.Update(key("esc"))
	view = updated.(AppModel).sidebar.View()
	if got := sidebarLine(t, view, "Pinned"); strings.TrimSpace(got) != "Pinned" {
		t.Errorf("after the drag the title is %q, want Pinned", got)
	}
	if strings.Contains(view, dragHandleGlyph) {
		t.Error("the drag handle must go when the drag ends")
	}
}

// dragApp is an app in the pinned-kind drag mode on panel 1.
func dragApp(t *testing.T) AppModel {
	t.Helper()
	m := menuFixture(t, nil, 0)
	m.activePanel = SidebarPanel
	m.sidebar.SetPinned([]k8s.ResourceType{k8s.ResourcePods, k8s.ResourceDeployments})
	m.sidebar.SnapCursorToKind(k8s.ResourcePods)
	updated, cmd := m.Update(key("D"))
	m = updated.(AppModel)
	for _, msg := range drainCmd(cmd) {
		next, _ := m.Update(msg)
		m = next.(AppModel)
	}
	if !m.sidebar.IsDragging() {
		t.Fatal("setup: D on a pinned kind must start the drag")
	}
	return m
}

// tdp K11: in a mode, Space opens no menu and doesn't end the mode.
func TestK11_DragSpaceDoesNothing(t *testing.T) {
	m := dragApp(t)
	updated, _ := m.Update(key(" "))
	got := updated.(AppModel)
	if got.topLayer() != nil {
		t.Error("Space in drag mode must not open anything")
	}
	if !got.sidebar.IsDragging() {
		t.Error("Space in drag mode must not cancel the drag")
	}
}

// tdp K11: ? in a mode is the mode's key reference.
func TestK11_DragQuestionMarkListsTheModesKeys(t *testing.T) {
	m := dragApp(t)
	got := pressQuestion(t, m)
	if got.help.title != "Drag mode keys" {
		t.Errorf("? in drag mode must open the drag mode keys, got %q", got.help.title)
	}
	if !got.sidebar.IsDragging() {
		t.Error("? must not end the drag")
	}
}

// tdp K11: Tab is suspended in the mode but answers — the drag stays,
// a toast says how to leave.
func TestK11_DragTabAnswersWithAToast(t *testing.T) {
	m := dragApp(t)
	updated, _ := m.Update(key("tab"))
	got := updated.(AppModel)
	if !got.sidebar.IsDragging() || got.activePanel != SidebarPanel {
		t.Error("Tab in drag mode must not move focus or cancel the drag")
	}
	if !got.toast.Owns() || !strings.Contains(got.toast.message, "Esc") {
		t.Errorf("Tab in drag mode must answer with a toast naming Esc, got %q", got.toast.message)
	}
}

// tdp K11, M1: the footer shows ? and the mode's keys while the mode is
// on; the sticky toast that carried them is gone. Keys only, written
// key:description (M5).
func TestK11_DragFooterListsTheModesKeys(t *testing.T) {
	m := dragApp(t)
	footer := m.statusLine.layoutLine()
	if want := " ?:keys j/k:move Enter:drop Esc:cancel"; footer != want {
		t.Errorf("the drag footer is %q, want %q", footer, want)
	}
	for _, want := range []string{"?:keys", "j/k:move", "Enter:drop", "Esc:cancel"} {
		if !strings.Contains(footer, want) {
			t.Errorf("the drag footer must show %q, got %q", want, footer)
		}
	}
	if strings.Contains(footer, "Space") {
		t.Error("Space does nothing in a mode: the footer must not offer it")
	}
	if m.toast.Owns() {
		t.Error("drag mode must no longer raise a sticky toast")
	}
}

// tdp K11: in the YAML selection mode, ? lists the mode's keys and Tab
// answers with a toast.
func TestK11_YamlSelectionMode(t *testing.T) {
	m := stackTestApp(t)
	_ = m.yamlPopup.Open("a: 1\nb: 2\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"})
	m.yamlPopup.animator.Finalize()
	m.yamlPopup, _ = m.yamlPopup.Update(key("v"))

	got := pressQuestion(t, m)
	if got.help.title != "Selection keys" {
		t.Errorf("? in the selection mode must list its keys, got %q", got.help.title)
	}
	updated, _ := m.Update(key("tab"))
	after := updated.(AppModel)
	if !after.yamlPopup.visualMode || !after.toast.Owns() {
		t.Error("Tab in the selection mode must keep the selection and answer with a toast")
	}
	if !strings.Contains(m.yamlPopup.renderFullPopup(), "?:keys") {
		t.Error("the selection mode's hint must start from ?")
	}
}
