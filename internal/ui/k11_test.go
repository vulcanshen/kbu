package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// tdp K11 + D5: the selection mode's key reference lists every vim motion
// a selection mode moves by — the viewer answers them all in the mode.
func TestK11_YamlSelectionKeysListEveryMotion(t *testing.T) {
	keys := refKeys(yamlVisualRows())
	for _, k := range []string{"h/j/k/l", "w/b/e", "0/$", "gg/G", "u/d"} {
		if !contains(keys, k) {
			t.Errorf("the selection mode's ? is missing %q: %v", k, keys)
		}
	}
	m := yamlOpenFor(t, k8s.ResourcePods, k8s.ResourceItem{Name: "a"})
	_ = m.yamlPopup.Open(strings.Repeat("line\n", 40), k8s.ResourcePods, k8s.ResourceItem{Name: "a"})
	m.yamlPopup.animator.Finalize()
	m.yamlPopup, _ = m.yamlPopup.Update(key("v"))
	m.yamlPopup, _ = m.yamlPopup.Update(key("d"))
	if !m.yamlPopup.visualMode || m.yamlPopup.cursorLine == 0 {
		t.Errorf("d in the selection mode must move down and keep the mode (line %d)", m.yamlPopup.cursorLine)
	}
	m.yamlPopup, _ = m.yamlPopup.Update(key("G"))
	if !m.yamlPopup.visualMode || m.yamlPopup.cursorLine != m.yamlPopup.lastLine() {
		t.Errorf("G in the selection mode must reach the last line and keep the mode (line %d)", m.yamlPopup.cursorLine)
	}
}

// tdp K11 + D2: the YAML viewer's selection mode shows in its frame —
// border and title Yellow (selection) while it lasts, the layer colour
// again once Esc leaves it.
func TestK11_YamlSelectionFrameIsYellow(t *testing.T) {
	truecolor(t)
	m := yamlOpenFor(t, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx", Namespace: "default"})
	corner := func() [3]int { return screenCells(m.yamlPopup.renderFullPopup())[0][0].fg }
	layer := hexRGB(string(theme.PopupLayerColor(1)))
	if got := corner(); !near(got, layer) {
		t.Fatalf("setup: the frame is %v, want the layer colour", got)
	}
	m.yamlPopup, _ = m.yamlPopup.Update(key("v"))
	if got := corner(); !near(got, hexRGB(theme.Yellow)) {
		t.Errorf("in the selection mode the frame is %v, want Yellow", got)
	}
	row, at := cellsOf(t, m.yamlPopup.renderFullPopup(), "YAML")
	if !near(row[at].fg, hexRGB(theme.Yellow)) {
		t.Errorf("in the selection mode the title is %v, want Yellow", row[at].fg)
	}
	top := screenCells(m.yamlPopup.renderFullPopup())[0]
	vis := strings.Index(rowText(top), "┤Visual├─╮")
	if vis < 0 {
		t.Fatalf("the selection mode must name itself top right: %q", rowText(top))
	}
	if name := top[len([]rune(rowText(top)[:vis]))+1]; !near(name.fg, hexRGB(theme.Yellow)) {
		t.Errorf("the mode name is %v, want Yellow", name.fg)
	}
	m.yamlPopup, _ = m.yamlPopup.Update(key("esc"))
	if got := corner(); !near(got, layer) {
		t.Errorf("after the selection the frame is %v, want the layer colour back", got)
	}
	if strings.Contains(rowText(screenCells(m.yamlPopup.renderFullPopup())[0]), "Visual") {
		t.Error("after the selection the mode name must go")
	}

	// A long name gives way to the mode name — including names that would
	// fit the top border on their own but not beside the mode name.
	for n := 60; n <= 130; n++ {
		long := yamlOpenFor(t, k8s.ResourcePods, k8s.ResourceItem{Name: strings.Repeat("n", n), Namespace: "default"})
		long.yamlPopup, _ = long.yamlPopup.Update(key("v"))
		lines := strings.Split(ansi.Strip(long.yamlPopup.renderFullPopup()), "\n")
		if !strings.HasSuffix(lines[0], "┤Visual├─╮") {
			t.Errorf("a %d-letter name pushed the mode name out: %q", n, lines[0])
		}
		if w, box := ansi.StringWidth(lines[0]), ansi.StringWidth(lines[1]); w != box {
			t.Errorf("a %d-letter name: the top border is %d wide, the box %d", n, w, box)
		}
	}
}

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

// panel1Top is the top row of panel 1 in the app's view, and where its
// corner is.
func panel1Top(t *testing.T, m AppModel) ([]cell, int) {
	t.Helper()
	for _, row := range screenCells(m.View()) {
		for x, c := range row {
			if c.r == '╔' || c.r == '╭' {
				return row, x
			}
		}
	}
	t.Fatal("panel 1 has no top border")
	return nil, 0
}

// rowText is a row of cells as text.
func rowText(row []cell) string {
	rs := make([]rune, len(row))
	for i, c := range row {
		rs[i] = c.r
	}
	return string(rs)
}

// tdp K11 + D2: the drag names itself on panel 1's frame — Drag top right,
// frame and chip Yellow — and keeps the focus line style (L5). The Pinned
// title carries no mode name; the drag handle still marks the moving row.
// All of it goes when the drag ends, kept or cancelled; it fits at 80
// columns too (L1).
func TestK11_DragModeShowsInPanel1(t *testing.T) {
	truecolor(t)
	for _, width := range []int{120, 80} {
		m := dragApp(t)
		resized, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		m = resized.(AppModel)
		if !m.sidebar.IsDragging() {
			t.Fatal("setup: resizing ended the drag")
		}
		row, x := panel1Top(t, m)
		if row[x].r != '╔' || !near(row[x].fg, hexRGB(theme.Yellow)) {
			t.Errorf("%d cols, dragging: panel 1's corner %q is %v, want a Yellow ╔", width, row[x].r, row[x].fg)
		}
		if chip := row[x+3]; chip.r != '[' || !near(chip.bg, hexRGB(theme.Yellow)) {
			t.Errorf("%d cols, dragging: the [1] chip %q is %v, want Yellow", width, chip.r, chip.bg)
		}
		if len(row) != width {
			t.Errorf("%d cols, dragging: the top row is %d cells wide (L4)", width, len(row))
		}
		top := rowText(row[:x+panelSidebarWidth])
		at := strings.Index(top, "╡Drag╞═╗")
		if at < 0 {
			t.Fatalf("%d cols, dragging: panel 1's top %q does not name the mode", width, top)
		}
		if name := row[len([]rune(top[:at]))+1]; name.r != 'D' || !near(name.fg, hexRGB(theme.Yellow)) {
			t.Errorf("%d cols, dragging: the mode name is %v, want Yellow", width, name.fg)
		}
		m.sidebar.SetSize(panelSidebarWidth-2, 20) // View() sized a copy
		view := m.sidebar.View()
		if got := sidebarLine(t, view, "Pinned"); strings.TrimSpace(ansi.Strip(got)) != "Pinned" {
			t.Errorf("the Pinned title is %q: the mode is named on the frame, not here", got)
		}
		if got := sidebarLine(t, view, "Pods"); !strings.HasPrefix(got, dragHandleGlyph+" Pods") {
			t.Errorf("the dragged row is %q, want the drag handle in its first cell", got)
		}
	}

	for _, end := range []string{"enter", "esc"} {
		m := dragApp(t)
		updated, _ := m.Update(key(end))
		m = updated.(AppModel)
		row, x := panel1Top(t, m)
		if row[x].r != '╔' || !near(row[x].fg, hexRGB(m.theme.Sidebar.CategoryFg)) {
			t.Errorf("after %s: panel 1's corner %q is %v, want the focus Blue ╔", end, row[x].r, row[x].fg)
		}
		if strings.Contains(rowText(row), "Drag") {
			t.Errorf("after %s: the mode name must go", end)
		}
		m.sidebar.SetSize(panelSidebarWidth-2, 20)
		if strings.Contains(m.sidebar.View(), dragHandleGlyph) {
			t.Errorf("after %s: the drag handle must go", end)
		}
	}
}

// dragApp is an app in the pinned-kind drag mode on panel 1.
func dragApp(t *testing.T) AppModel {
	t.Helper()
	m := stackTestApp(t)
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
