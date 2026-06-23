package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/km8/internal/theme"
)

func newHintPopup(t *testing.T) HintPopupModel {
	t.Helper()
	th, err := theme.LoadTheme("")
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	m := NewHintPopupModel(th)
	m.SetSize(120, 40)
	return m
}

func drainHintToInteractive(t *testing.T, m *HintPopupModel, openCmd tea.Cmd) {
	t.Helper()
	if openCmd == nil {
		return
	}
	for i := 0; i < 50; i++ {
		if m.IsInteractive() {
			return
		}
		msg := openCmd()
		if tick, ok := msg.(AnimTickMsg); ok {
			openCmd = m.HandleTick(tick)
			continue
		}
		break
	}
	if !m.IsInteractive() {
		t.Fatalf("popup never became interactive after 50 ticks")
	}
}

func TestHintPopup_Sidebar_RendersAllRows(t *testing.T) {
	m := newHintPopup(t)
	title, rows := sidebarHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	for _, want := range []string{"j/k", "Enter", "/", "Esc", "N", "C"} {
		if !strings.Contains(view, want) {
			t.Errorf("sidebar hint missing key %q", want)
		}
	}
}

func TestHintPopup_Logs_RendersAllRows(t *testing.T) {
	m := newHintPopup(t)
	title, rows := logsHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	for _, want := range []string{"j/k", "u/d", "gg", "G", "y", "z"} {
		if !strings.Contains(view, want) {
			t.Errorf("logs hint missing key %q", want)
		}
	}
}

func TestHintPopup_Events_RendersAllRows(t *testing.T) {
	m := newHintPopup(t)
	title, rows := eventsHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	for _, want := range []string{"j/k", "u/d", "gg/G", "y", "z"} {
		if !strings.Contains(view, want) {
			t.Errorf("events hint missing key %q", want)
		}
	}
}

func TestHintPopup_Conditions_RendersAllRows(t *testing.T) {
	m := newHintPopup(t)
	title, rows := conditionsHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	for _, want := range []string{"Conditions", "j/k", "u/d", "gg/G", "y", "z"} {
		if !strings.Contains(view, want) {
			t.Errorf("conditions hint missing %q", want)
		}
	}
}

func TestHintPopup_RelativesDrill_RendersAllRows(t *testing.T) {
	m := newHintPopup(t)
	title, rows := relativesDrillHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	for _, want := range []string{"j/k", "Y", "Enter", "drill"} {
		if !strings.Contains(view, want) {
			t.Errorf("relatives-drill hint missing key %q", want)
		}
	}
	// Depth=1 hint should NOT show the drill-up icon (which only the Esc
	// row uses) — there's no parent in the chain to pop back to. The
	// popup's bottom border legend mentions "Esc/q/Space: close" so we
	// can't just grep for "Esc" — checking the unique icon is safer.
	if strings.Contains(view, drillUpIcon) {
		t.Errorf("depth=1 relatives hint must not show the Esc row (nothing to pop)")
	}
}

func TestHintPopup_Panel2Empty_RendersAllRows(t *testing.T) {
	m := newHintPopup(t)
	title, rows := panel2EmptyHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	view := m.RenderPopup()
	for _, want := range []string{"N", "/", ".", "C", "No items here"} {
		if !strings.Contains(view, want) {
			t.Errorf("panel2-empty hint missing %q", want)
		}
	}
}

func TestHintPopup_SpaceCloses(t *testing.T) {
	m := newHintPopup(t)
	title, rows := sidebarHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key(" "))
	if closeCmd == nil {
		t.Fatal("Space must trigger animator close")
	}
}

func TestHintPopup_EscCloses(t *testing.T) {
	m := newHintPopup(t)
	title, rows := sidebarHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key("esc"))
	if closeCmd == nil {
		t.Fatal("Esc must trigger animator close")
	}
}

func TestHintPopup_EnterCloses(t *testing.T) {
	m := newHintPopup(t)
	title, rows := sidebarHintContent()
	cmd := m.Open(title, rows)
	drainHintToInteractive(t, &m, cmd)

	_, closeCmd := m.Update(key("enter"))
	if closeCmd == nil {
		t.Fatal("Enter must close the read-only help popup")
	}
}

func TestHintPopup_Actions_HotkeyCommitsAndCloses(t *testing.T) {
	// OpenWithActions registers "P" as the hotkey for the Pin action.
	// Pressing capital P (Shift+P) directly must commit + close in
	// one batch — same UX as panel-2's Y/E/S/D direct dispatch.
	m := newHintPopup(t)
	actions := []hintAction{
		{label: "Pin Pods", key: "P", action: "PinKind"},
	}
	title, rows := sidebarHintContent()
	openCmd := m.OpenWithActions(title, actions, rows)
	drainHintToInteractive(t, &m, openCmd)

	_, batchCmd := m.Update(key("P"))
	if batchCmd == nil {
		t.Fatal("hotkey P must return a Cmd batch (close + action msg)")
	}
	// Drain the batch — expect to see HintActionMsg.
	saw := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(HintActionMsg)
		if !ok {
			return false
		}
		if am.Action != "PinKind" {
			t.Errorf("hotkey P committed action %q, want PinKind", am.Action)
		}
		saw = true
		return true
	})
	if !saw {
		t.Error("hotkey P did not produce a HintActionMsg")
	}
}

func TestHintPopup_Actions_EnterOnCursorCommits(t *testing.T) {
	// Without the hotkey, cursor + Enter is the menu-only fallback —
	// same path Panel 2's multi-char "Enter"/"LockCompare" keys use.
	m := newHintPopup(t)
	actions := []hintAction{
		{label: "Unpin Pods", key: "P", action: "UnpinKind"},
	}
	title, rows := sidebarHintContent()
	openCmd := m.OpenWithActions(title, actions, rows)
	drainHintToInteractive(t, &m, openCmd)

	_, batchCmd := m.Update(key("enter"))
	if batchCmd == nil {
		t.Fatal("cursor + Enter must commit the only action")
	}
	saw := false
	expectMsg(t, batchCmd, func(msg tea.Msg) bool {
		am, ok := msg.(HintActionMsg)
		if !ok {
			return false
		}
		if am.Action != "UnpinKind" {
			t.Errorf("Enter committed %q, want UnpinKind", am.Action)
		}
		saw = true
		return true
	})
	if !saw {
		t.Error("cursor Enter did not produce HintActionMsg")
	}
}

func TestHintPopup_Actions_SpaceClosesWithoutCommit(t *testing.T) {
	// Space (the "close" convention across km8 popups) must close the
	// popup WITHOUT committing any action — pressing Space mid-decision
	// would be a confusing way to silently pin/unpin.
	m := newHintPopup(t)
	actions := []hintAction{
		{label: "Pin Pods", key: "P", action: "PinKind"},
	}
	title, rows := sidebarHintContent()
	openCmd := m.OpenWithActions(title, actions, rows)
	drainHintToInteractive(t, &m, openCmd)

	_, closeCmd := m.Update(key(" "))
	if closeCmd == nil {
		t.Fatal("Space must trigger close")
	}
	// Expect NO HintActionMsg emitted — only animator close ticks.
	expectMsg(t, closeCmd, func(msg tea.Msg) bool {
		if _, ok := msg.(HintActionMsg); ok {
			t.Error("Space must NOT commit an action — only close")
		}
		return false
	})
}
