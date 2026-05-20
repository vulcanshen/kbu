package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/km8/internal/theme"
)

func newTestStatusLine() StatusLineModel {
	t := theme.DefaultTheme()
	return NewStatusLineModel(t)
}

// ── hints by panel ─────────────────────────────────────────────────────────

func TestStatusLineModel_Hints_Sidebar(t *testing.T) {
	m := newTestStatusLine()
	m.SetActivePanel(SidebarPanel)

	hints := m.hints()
	keys := hintKeys(hints)

	mustContain(t, keys, "n", "sidebar must have namespace hint")
	mustContain(t, keys, "c", "sidebar must have context hint")
	mustContain(t, keys, "y", "sidebar must have copy hint (global)")
}

func TestStatusLineModel_Hints_CopyHintOnAllPanels(t *testing.T) {
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusLine()
		m.SetActivePanel(p)
		keys := hintKeys(m.hints())
		mustContain(t, keys, "y", "panel must show global copy hint")
	}
}

func TestStatusLineModel_Hints_TablePanel(t *testing.T) {
	m := newTestStatusLine()
	m.SetActivePanel(TablePanel)

	hints := m.hints()
	keys := hintKeys(hints)

	mustContain(t, keys, "e", "table panel must have edit hint")
	mustContain(t, keys, "D", "table panel must have delete hint")
	mustContain(t, keys, "/", "table panel must have search hint")
}

func TestStatusLineModel_Hints_TablePanel_DrillDown(t *testing.T) {
	m := newTestStatusLine()
	m.SetActivePanel(TablePanel)
	m.SetDrillDown(true)

	hints := m.hints()
	keys := hintKeys(hints)

	mustContain(t, keys, "esc", "drill-down must have esc hint")
	// No edit/delete when drilled down.
	for _, k := range keys {
		if k == "e" {
			t.Error("drill-down table must not show edit hint")
		}
		if k == "D" {
			t.Error("drill-down table must not show delete hint")
		}
	}
}

func TestStatusLineModel_Hints_DetailPanel(t *testing.T) {
	m := newTestStatusLine()
	m.SetActivePanel(DetailPanel)

	hints := m.hints()
	keys := hintKeys(hints)

	mustContain(t, keys, "h/l", "detail panel must have tab hint")
	mustContain(t, keys, "=/-", "detail panel must have expand/restore hint")
}

// ── LineCount ─────────────────────────────────────────────────────────────

func TestStatusLineModel_LineCount_WideEnough_IsOne(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(200)
	m.SetActivePanel(TablePanel)

	// At 200 chars wide, all hints should fit in one row.
	if c := m.LineCount(); c != 1 {
		t.Errorf("expected 1 line at wide width, got %d", c)
	}
}

func TestStatusLineModel_LineCount_VeryNarrow_IsTwo(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(20) // too narrow for all hints
	m.SetActivePanel(TablePanel)

	// At 20 chars, hints must spill to a second row.
	if c := m.LineCount(); c < 2 {
		t.Errorf("expected ≥2 lines at narrow width, got %d", c)
	}
}

func TestStatusLineModel_LineCount_NeverExceedsMax(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(1) // extreme narrow
	m.SetActivePanel(TablePanel)

	if c := m.LineCount(); c > maxStatusLineRows {
		t.Errorf("LineCount must never exceed %d, got %d", maxStatusLineRows, c)
	}
}

// ── ViewWithNotice ─────────────────────────────────────────────────────────

func TestStatusLineModel_ViewWithNotice_ShowsError(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(120)
	view := m.ViewWithNotice(1, "fetch failed", "")

	if !strings.Contains(view, "fetch failed") {
		t.Error("ViewWithNotice must show error message when errors > 0")
	}
}

func TestStatusLineModel_ViewWithNotice_ShowsSuccess(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(120)
	view := m.ViewWithNotice(0, "", "applied")

	if !strings.Contains(view, "applied") {
		t.Error("ViewWithNotice must show success message when no errors")
	}
}

func TestStatusLineModel_ViewWithNotice_ErrorPriorityOverSuccess(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(120)
	view := m.ViewWithNotice(1, "fetch failed", "applied")

	if !strings.Contains(view, "fetch failed") {
		t.Error("error must appear when both error and success are present")
	}
	if strings.Contains(view, "applied") {
		t.Error("success must not appear when there are unread errors")
	}
}

func TestStatusLineModel_ViewWithNotice_NoNotice(t *testing.T) {
	m := newTestStatusLine()
	m.SetWidth(120)
	view := m.ViewWithNotice(0, "", "")

	// Should still render something (the hints).
	if view == "" {
		t.Error("ViewWithNotice must render something even with no notice")
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func hintKeys(hints []hint) []string {
	keys := make([]string, len(hints))
	for i, h := range hints {
		keys[i] = h.key
	}
	return keys
}

func mustContain(t *testing.T, slice []string, item, msg string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			return
		}
	}
	t.Errorf("%s: %q not found in %v", msg, item, slice)
}
