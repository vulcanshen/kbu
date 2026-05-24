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

	keys := hintKeys(m.hints())
	mustContain(t, keys, "n", "sidebar must have namespace hint")
	mustContain(t, keys, "c", "sidebar must have context hint")
}

func TestStatusLineModel_Hints_GlobalsOnAllPanels(t *testing.T) {
	// `?` (help), `q` (quit), `Y` (YAML popup), `M-t` (KM8erm) are global
	// one-key actions that should be discoverable from any panel.
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusLine()
		m.SetActivePanel(p)
		keys := hintKeys(m.hints())
		mustContain(t, keys, "?", "panel must show help hint")
		mustContain(t, keys, "q", "panel must show quit hint")
		mustContain(t, keys, "Y", "panel must show YAML popup hint")
		mustContain(t, keys, "M-t", "panel must show KM8erm hint")
	}
}

func TestStatusLineModel_Hints_TablePanel(t *testing.T) {
	m := newTestStatusLine()
	m.SetActivePanel(TablePanel)

	keys := hintKeys(m.hints())
	mustContain(t, keys, "e", "table panel must have edit hint")
	mustContain(t, keys, "D", "table panel must have delete hint")
	mustContain(t, keys, "/", "table panel must have filter hint")
	// Enter (focus → detail) is omitted intentionally — it's the obvious
	// adjacent-panel motion and not worth a slot in the hints bar.
	for _, k := range keys {
		if k == "Enter" {
			t.Errorf("Enter hint should be hidden on table panel (focus-shift is obvious), got hints=%v", keys)
		}
	}
}

func TestStatusLineModel_Hints_TablePanel_DrillDown(t *testing.T) {
	m := newTestStatusLine()
	m.SetActivePanel(TablePanel)
	m.SetDrillDown(true)

	keys := hintKeys(m.hints())
	mustContain(t, keys, "esc", "drill-down must have esc hint")
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

	keys := hintKeys(m.hints())
	mustContain(t, keys, "h/l", "detail panel must have tab hint")
	mustContain(t, keys, "=/-", "detail panel must have expand hint")
}

// ── LineCount ─────────────────────────────────────────────────────────────

func TestStatusLineModel_LineCount_IsAlwaysOne(t *testing.T) {
	// Post-refactor: status line is strictly one row. Narrow terminals drop
	// trailing hints rather than wrapping to a second row.
	for _, width := range []int{200, 80, 40, 20, 1} {
		m := newTestStatusLine()
		m.SetWidth(width)
		m.SetActivePanel(TablePanel)
		if c := m.LineCount(); c != 1 {
			t.Errorf("LineCount must always be 1, got %d at width=%d", c, width)
		}
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
