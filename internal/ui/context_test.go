package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestContextPicker() ContextPickerModel {
	t := theme.DefaultTheme()
	m := NewContextPickerModel(t)
	m.SetSize(80, 40)
	return m
}

func TestContextPickerModel_InitialState(t *testing.T) {
	m := newTestContextPicker()

	if m.IsActive() {
		t.Error("expected picker to be inactive initially")
	}
}

func TestContextPickerModel_Open(t *testing.T) {
	m := newTestContextPicker()

	contexts := []string{"dev", "staging", "prod"}
	m.Open(contexts, "staging")

	if !m.IsActive() {
		t.Error("expected picker to be active after Open")
	}
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 (staging), got %d", m.cursor)
	}
	if m.current != "staging" {
		t.Errorf("expected current='staging', got %q", m.current)
	}
}

func TestContextPickerModel_OpenCursorOnCurrent(t *testing.T) {
	m := newTestContextPicker()

	contexts := []string{"alpha", "beta", "gamma"}
	m.Open(contexts, "gamma")

	if m.cursor != 2 {
		t.Errorf("expected cursor=2 (gamma), got %d", m.cursor)
	}
}

func TestContextPickerModel_OpenCurrentNotFound(t *testing.T) {
	m := newTestContextPicker()

	contexts := []string{"alpha", "beta", "gamma"}
	m.Open(contexts, "nonexistent")

	// Should default to cursor=0 when current is not in the list.
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 when current not found, got %d", m.cursor)
	}
}

func TestContextPickerModel_NavigateDown(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b", "c"}, "a")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after j, got %d", m.cursor)
	}

	// Should not go past end.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 at bottom boundary, got %d", m.cursor)
	}
}

func TestContextPickerModel_NavigateUp(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b", "c"}, "c")

	if m.cursor != 2 {
		t.Fatalf("expected cursor=2, got %d", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after k, got %d", m.cursor)
	}

	// Should not go below 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 at top boundary, got %d", m.cursor)
	}
}

func TestContextPickerModel_SelectEnter(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"dev", "staging", "prod"}, "dev")

	// Navigate to "staging".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}

	// Press Enter.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsActive() {
		t.Error("expected picker to be inactive after Enter")
	}
	if cmd == nil {
		t.Fatal("expected cmd after Enter")
	}

	msg := cmd()
	ccm, ok := msg.(ContextChangedMsg)
	if !ok {
		t.Fatalf("expected ContextChangedMsg, got %T", msg)
	}
	if ccm.Context != "staging" {
		t.Errorf("expected Context='staging', got %q", ccm.Context)
	}
}

func TestContextPickerModel_CloseOnEsc(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b"}, "a")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.IsActive() {
		t.Error("expected picker to be inactive after Esc")
	}
	if cmd != nil {
		t.Error("expected nil cmd after Esc (no context change)")
	}
}

func TestContextPickerModel_CloseOnC(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b"}, "a")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if m.IsActive() {
		t.Error("expected picker to be inactive after c")
	}
	if cmd != nil {
		t.Error("expected nil cmd after c (no context change)")
	}
}

func TestContextPickerModel_InactiveIgnoresInput(t *testing.T) {
	m := newTestContextPicker()
	// Not opened, should be inactive.

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Error("expected nil cmd when inactive")
	}
}

func TestContextPickerModel_Close(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a"}, "a")

	if !m.IsActive() {
		t.Fatal("expected active after Open")
	}
	m.Close()
	if m.IsActive() {
		t.Error("expected inactive after Close")
	}
}
