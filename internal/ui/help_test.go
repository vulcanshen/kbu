package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestHelp() HelpModel {
	t := theme.DefaultTheme()
	m := NewHelpModel(t)
	m.SetSize(80, 40)
	return m
}

func TestHelpModel_InitialState(t *testing.T) {
	m := newTestHelp()

	if m.IsActive() {
		t.Error("expected help to be inactive initially")
	}

	// View should be empty when inactive.
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when inactive, got %q", view)
	}
}

func TestHelpModel_Toggle(t *testing.T) {
	m := newTestHelp()

	// Toggle on.
	m.Toggle()
	m.animator.Finalize()
	if !m.IsActive() {
		t.Error("expected help to be active after toggle")
	}

	// Toggle off.
	m.Toggle()
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected help to be inactive after second toggle")
	}
}

func TestHelpModel_CloseWithEsc(t *testing.T) {
	m := newTestHelp()
	m.Toggle()
	m.animator.Finalize()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected help to be inactive after Esc")
	}
}

func TestHelpModel_CloseWithQuestionMark(t *testing.T) {
	m := newTestHelp()
	m.Toggle()
	m.animator.Finalize()

	m, _ = m.Update(keyMsg('?'))
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected help to be inactive after ?")
	}
}

func TestHelpModel_ViewContainsKeybindings(t *testing.T) {
	m := newTestHelp()
	m.Toggle()
	m.animator.Finalize()

	view := m.View()

	// v1.7+ trimmed help is intentionally minimal — "Space opens the
	// menu, 一看就懂" eliminates the need to spell out per-context
	// triggers. Section names: Core (4 universal gestures) / Navigation
	// (cursor + panel) / Global (app-level) / KM8erm.
	expectedSections := []string{
		"Core",
		"Navigation",
		"Global",
	}
	for _, section := range expectedSections {
		if !strings.Contains(view, section) {
			t.Errorf("expected help view to contain section %q", section)
		}
	}

	expectedKeys := []string{
		"Tab",
		"Enter",
		"Esc",
		"Space",
		"h / l",
		"j / k",
		"gg / G",
		">",
		"?",
	}
	for _, key := range expectedKeys {
		if !strings.Contains(view, key) {
			t.Errorf("expected help view to contain key %q", key)
		}
	}
}

func TestHelpModel_Scroll(t *testing.T) {
	m := newTestHelp()
	m.SetSize(80, 10) // Small height to force scrolling
	m.Toggle()
	m.animator.Finalize()

	// Initial scroll offset should be 0.
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 initially, got %d", m.scrollOffset)
	}

	// Scroll down.
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after j, got %d", m.scrollOffset)
	}

	// Scroll back up.
	m, _ = m.Update(keyMsg('k'))
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after k, got %d", m.scrollOffset)
	}

	// Scroll up at top should stay at 0.
	m, _ = m.Update(keyMsg('k'))
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 at top boundary, got %d", m.scrollOffset)
	}
}

func TestHelpModel_ScrollResetOnToggle(t *testing.T) {
	m := newTestHelp()
	m.SetSize(80, 10)
	m.Toggle()
	m.animator.Finalize()

	// Scroll down.
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 2 {
		t.Fatalf("expected scrollOffset=2 after 2 j's, got %d", m.scrollOffset)
	}

	// Close and reopen — scroll should reset.
	m.Toggle() // close
	m.animator.Finalize()
	m.Toggle() // reopen
	m.animator.Finalize()
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after reopen, got %d", m.scrollOffset)
	}
}
