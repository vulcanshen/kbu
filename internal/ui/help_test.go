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

// TestHelpModel_RenderNeverPanicsAcrossSizes was added after a real
// crash at 60x30 in v1.7.10: the desc-string change ("Copy focus:
// cursor row when it has one, else full content") pushed the
// splitGroupsForColumns balance heuristic to put every section on
// the left column, and the "empty right column" case wasn't
// guarded before the row-zip loop indexed rightLines[i] out of
// bounds. Guards against a recurrence across the terminal sizes
// users are likely to hit.
func TestHelpModel_RenderNeverPanicsAcrossSizes(t *testing.T) {
	sizes := []struct{ w, h int }{
		{40, 20}, {50, 25}, {60, 30}, {70, 30},
		{80, 40}, {100, 40}, {120, 50}, {160, 60}, {220, 80},
	}
	for _, sz := range sizes {
		sz := sz
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic at %dx%d: %v", sz.w, sz.h, r)
				}
			}()
			m := newTestHelp()
			m.SetSize(sz.w, sz.h)
			m.Toggle()
			m.animator.Finalize()
			_ = m.View()
		})
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
	// (cursor + panel) / Global (app-level) / Alterm.
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
