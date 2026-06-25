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
// v1.7+ status-line mental model: only the universal cross-panel gestures
// stay here. `?` for the full reference + the four core gestures
// (Esc/Space/Enter/Tab, per the popup-design mindset) + Alt-t KM8erm.
// Everything panel-specific (N/C, `/`, trigger letters) lives in the
// statusbar labels or the per-row Space menus / popups.

func TestStatusLineModel_Hints_Universal(t *testing.T) {
	// The five core gestures + help + KM8erm + settings — present on
	// every panel, independent of activePanel state.
	want := []string{"?", "Esc", "Space", "Enter", "Tab", "Alt-t", ">"}
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusLine()
		m.SetActivePanel(p)
		keys := hintKeys(m.hints())
		for _, k := range want {
			mustContain(t, keys, k, "panel must show "+k+" hint")
		}
	}
}

func TestStatusLineModel_Hints_RetiredKeysNotSurfaced(t *testing.T) {
	// Keys retired from the status-line in v1.7: q (close gesture is
	// universal Esc), N/C (now `[N]amespace:` / `[C]ontext:` on the
	// statusbar above), `/` filter, and the panel-specific trigger
	// letters (E/S/D/Y, h/l, =/-, z) that always lived in popups.
	m := newTestStatusLine()
	m.SetActivePanel(TablePanel)
	keys := hintKeys(m.hints())
	for _, banned := range []string{"q", "N", "C", "/", "E", "S", "D", "Y", "h/l", "=/-", "z"} {
		for _, k := range keys {
			if k == banned {
				t.Errorf("status line must NOT show %q (retired in v1.7 — see statusbar / context menu / ? help), got hints=%v", banned, keys)
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
