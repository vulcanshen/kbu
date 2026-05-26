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
//
// v1.5.x status-line mental model: keep only universal navigation keys
// (`?`/`q` + `N`/`C` + space/enter, plus panel-scoped `/` filter on panels
// 1+2). Trigger letters (E/S/D/Y) live in the per-row Space context menu,
// not on the status line — the menu surfaces them in context, the status
// line doesn't duplicate.

func TestStatusLineModel_Hints_Universal(t *testing.T) {
	// `?` help, `q` quit, `N` ns, `C` ctx, space menu, enter into —
	// the always-on core, present on every panel.
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusLine()
		m.SetActivePanel(p)
		keys := hintKeys(m.hints())
		mustContain(t, keys, "?", "panel must show help hint")
		mustContain(t, keys, "q", "panel must show quit hint")
		mustContain(t, keys, "N", "panel must show namespace hint")
		mustContain(t, keys, "C", "panel must show context hint")
		mustContain(t, keys, "space", "panel must show space (menu) hint")
		mustContain(t, keys, "enter", "panel must show enter (into) hint")
	}
}

func TestStatusLineModel_Hints_FilterOnPanels12Only(t *testing.T) {
	// `/` filter only renders on panel 1 and panel 2 — panel 3's in-panel
	// search was retired in v1.5.0, so showing the hint on panel 3 would
	// mislead users.
	for _, p := range []Panel{SidebarPanel, TablePanel} {
		m := newTestStatusLine()
		m.SetActivePanel(p)
		keys := hintKeys(m.hints())
		mustContain(t, keys, "/", "panel 1/2 must show filter hint")
	}
	m := newTestStatusLine()
	m.SetActivePanel(DetailPanel)
	keys := hintKeys(m.hints())
	for _, k := range keys {
		if k == "/" {
			t.Errorf("panel 3 must NOT show filter hint (search retired in v1.5.0), got hints=%v", keys)
		}
	}
}

func TestStatusLineModel_Hints_TriggerLettersHiddenFromStatusLine(t *testing.T) {
	// Trigger letters (E edit, S shell, D delete, Y yaml) are surfaced
	// via the per-row Space context menu — the status line no longer
	// duplicates them. Same for h/l (tab switch) and =/- (now z).
	m := newTestStatusLine()
	m.SetActivePanel(TablePanel)
	keys := hintKeys(m.hints())
	for _, banned := range []string{"E", "S", "D", "Y", "h/l", "=/-", "z", "M-t"} {
		for _, k := range keys {
			if k == banned {
				t.Errorf("status line must NOT show %q (lives in context menu / ? help), got hints=%v", banned, keys)
			}
		}
	}
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
