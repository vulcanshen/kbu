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
	if !m.IsActive() {
		t.Error("expected help to be active after toggle")
	}

	// Toggle off.
	m.Toggle()
	if m.IsActive() {
		t.Error("expected help to be inactive after second toggle")
	}
}

func TestHelpModel_CloseWithEsc(t *testing.T) {
	m := newTestHelp()
	m.Toggle() // activate

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.IsActive() {
		t.Error("expected help to be inactive after Esc")
	}
}

func TestHelpModel_CloseWithQ(t *testing.T) {
	m := newTestHelp()
	m.Toggle() // activate

	m, _ = m.Update(keyMsg('q'))
	if m.IsActive() {
		t.Error("expected help to be inactive after q")
	}
}

func TestHelpModel_CloseWithQuestionMark(t *testing.T) {
	m := newTestHelp()
	m.Toggle() // activate

	m, _ = m.Update(keyMsg('?'))
	if m.IsActive() {
		t.Error("expected help to be inactive after ?")
	}
}

func TestHelpModel_ViewContainsKeybindings(t *testing.T) {
	m := newTestHelp()
	m.Toggle()

	view := m.View()

	// Check that key sections are present.
	expectedSections := []string{
		"Navigation",
		"Sidebar",
		"Table",
		"Detail",
		"Global",
	}
	for _, section := range expectedSections {
		if !strings.Contains(view, section) {
			t.Errorf("expected help view to contain section %q", section)
		}
	}

	// Check some specific keybindings.
	expectedKeys := []string{
		"j / k",
		"gg / G",
		"/",
		"[ / ]",
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

	// Scroll down.
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 2 {
		t.Fatalf("expected scrollOffset=2 after 2 j's, got %d", m.scrollOffset)
	}

	// Close and reopen — scroll should reset.
	m.Toggle() // close
	m.Toggle() // reopen
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after reopen, got %d", m.scrollOffset)
	}
}
