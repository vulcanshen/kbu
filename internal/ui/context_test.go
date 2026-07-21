package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/kbu/internal/theme"
)

func newTestContextPicker() ContextPickerModel {
	t := theme.DefaultTheme()
	return NewContextPickerModel(t)
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
	m.animator.Finalize()

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
	m.animator.Finalize()

	if m.cursor != 2 {
		t.Errorf("expected cursor=2 (gamma), got %d", m.cursor)
	}
}

func TestContextPickerModel_OpenCurrentNotFound(t *testing.T) {
	m := newTestContextPicker()

	contexts := []string{"alpha", "beta", "gamma"}
	m.Open(contexts, "nonexistent")
	m.animator.Finalize()

	// Should default to cursor=0 when current is not in the list.
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 when current not found, got %d", m.cursor)
	}
}

func TestContextPickerModel_NavigateDown(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b", "c"}, "a")
	m.animator.Finalize()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after j, got %d", m.cursor)
	}

	// Past end should wrap to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after j wraps at bottom, got %d", m.cursor)
	}
}

func TestContextPickerModel_NavigateUp(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b", "c"}, "c")
	m.animator.Finalize()

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

	// Past top should wrap to last.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after k wraps at top, got %d", m.cursor)
	}
}

func TestContextPickerModel_SelectEnter(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"dev", "staging", "prod"}, "dev")
	m.animator.Finalize()

	// Navigate to "staging".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", m.cursor)
	}

	// Press Enter.
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("expected picker to be inactive after Enter")
	}
	if cmd == nil {
		t.Fatal("expected cmd after Enter")
	}

	// Enter returns a batch: [closeCmd, contextChangedCmd]. Execute it to find ContextChangedMsg.
	msg := cmd()
	batch, isBatch := msg.(tea.BatchMsg)
	var ccm ContextChangedMsg
	found := false
	if isBatch {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			submsg := sub()
			if c, ok := submsg.(ContextChangedMsg); ok {
				ccm = c
				found = true
			}
		}
	} else if c, ok := msg.(ContextChangedMsg); ok {
		ccm = c
		found = true
	}
	if !found {
		t.Fatalf("expected ContextChangedMsg in cmd output, got %T", msg)
	}
	if ccm.Context != "staging" {
		t.Errorf("expected Context='staging', got %q", ccm.Context)
	}
}

func TestContextPickerModel_CloseOnEsc(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b"}, "a")
	m.animator.Finalize()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("expected picker to be inactive after Esc")
	}
}

func TestContextPickerModel_CloseOnC(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b"}, "a")
	m.animator.Finalize()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("expected picker to be inactive after c")
	}
}

func TestContextPickerModel_CloseOnUppercaseC(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a", "b"}, "a")
	m.animator.Finalize()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("expected picker to be inactive after C (alias)")
	}
}

func TestContextPickerModel_InactiveIgnoresInput(t *testing.T) {
	m := newTestContextPicker()
	// Not opened, should be inactive.

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Error("expected nil cmd when inactive")
	}
	_ = m
}

func TestContextPickerModel_Close(t *testing.T) {
	m := newTestContextPicker()
	m.Open([]string{"a"}, "a")
	m.animator.Finalize()

	if !m.IsActive() {
		t.Fatal("expected active after Open")
	}
	m.Close()
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected inactive after Close")
	}
}
