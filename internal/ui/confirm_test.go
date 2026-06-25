package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestConfirm() ConfirmModel {
	t := theme.DefaultTheme()
	m := NewConfirmModel(t)
	m.SetSize(120, 40)
	return m
}

// openConfirm calls Show and finalizes animation so the model is interactive.
func openConfirm(m *ConfirmModel, onConfirm tea.Cmd) {
	m.Show(ConfirmDelete, "Delete pod/nginx?", "namespace: default", onConfirm)
	m.animator.Finalize()
}

// ── Initial state ──────────────────────────────────────────────────────────

func TestConfirmModel_InitialState(t *testing.T) {
	m := newTestConfirm()

	if m.IsActive() {
		t.Error("confirm model should be inactive initially")
	}
	if m.IsInteractive() {
		t.Error("confirm model should not be interactive initially")
	}
}

// ── Show ───────────────────────────────────────────────────────────────────

func TestConfirmModel_Show_Activates(t *testing.T) {
	m := newTestConfirm()
	m.Show(ConfirmDelete, "msg", "detail", nil)
	m.animator.Finalize()

	if !m.IsActive() {
		t.Error("Show() must activate the confirm dialog")
	}
	if !m.IsInteractive() {
		t.Error("Show() must make the dialog interactive after animation")
	}
}

func TestConfirmModel_Show_StoresContent(t *testing.T) {
	m := newTestConfirm()
	m.Show(ConfirmDelete, "Delete pod?", "pod/nginx", nil)

	if m.message != "Delete pod?" {
		t.Errorf("expected message %q, got %q", "Delete pod?", m.message)
	}
	if m.detail != "pod/nginx" {
		t.Errorf("expected detail %q, got %q", "pod/nginx", m.detail)
	}
	if m.action != ConfirmDelete {
		t.Errorf("expected action ConfirmDelete, got %v", m.action)
	}
}

func TestConfirmModel_Show_StoresOnConfirm(t *testing.T) {
	m := newTestConfirm()
	sentinel := func() tea.Msg { return nil }
	m.Show(ConfirmDelete, "msg", "", sentinel)

	if m.onConfirm == nil {
		t.Error("Show() must store the onConfirm cmd")
	}
}

// ── Confirm (Enter / y) ───────────────────────────────────────────────────

func TestConfirmModel_Enter_ClearsOnConfirmAndReturnsCmd(t *testing.T) {
	m := newTestConfirm()
	openConfirm(&m, func() tea.Msg { return nil })

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.onConfirm != nil {
		t.Error("onConfirm must be cleared after Enter to prevent double-fire")
	}
	if cmd == nil {
		t.Error("Enter must return a non-nil cmd (batch of onConfirm + close)")
	}
}

func TestConfirmModel_Y_ClearsOnConfirmAndReturnsCmd(t *testing.T) {
	m := newTestConfirm()
	openConfirm(&m, func() tea.Msg { return nil })

	m, cmd := m.Update(keyMsg('y'))
	if m.onConfirm != nil {
		t.Error("onConfirm must be cleared after y")
	}
	if cmd == nil {
		t.Error("y must return a non-nil cmd")
	}
}

// ── Cancel (Esc / n / q) ─────────────────────────────────────────────────

func TestConfirmModel_Esc_CancelsWithoutFiringOnConfirm(t *testing.T) {
	m := newTestConfirm()
	openConfirm(&m, func() tea.Msg { return nil })

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("Esc must close the dialog")
	}
	if m.onConfirm != nil {
		t.Error("Esc must clear onConfirm without calling it")
	}
}

func TestConfirmModel_N_Cancels(t *testing.T) {
	m := newTestConfirm()
	openConfirm(&m, func() tea.Msg { return nil })

	m, _ = m.Update(keyMsg('n'))
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("n must close the dialog")
	}
	if m.onConfirm != nil {
		t.Error("n must clear onConfirm")
	}
}

// ── Close ─────────────────────────────────────────────────────────────────

func TestConfirmModel_Close_ClearsOnConfirm(t *testing.T) {
	m := newTestConfirm()
	openConfirm(&m, func() tea.Msg { return nil })

	m.Close()
	if m.onConfirm != nil {
		t.Error("Close() must clear onConfirm")
	}
}

// ── Inactive guard ────────────────────────────────────────────────────────

func TestConfirmModel_InactiveIgnoresKeys(t *testing.T) {
	m := newTestConfirm()
	// Do NOT call Show.

	m, cmd := m.Update(keyMsg('y'))
	if m.IsActive() || cmd != nil {
		t.Error("inactive confirm must ignore key messages")
	}
}
