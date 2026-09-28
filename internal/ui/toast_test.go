package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/kbu/internal/theme"
)

func TestToastModel_InitialInactive(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	if m.IsActive() {
		t.Error("expected toast inactive initially")
	}
	if m.RenderPopup() != "" {
		t.Error("expected empty render when inactive")
	}
}

func TestToastModel_ShowActivates(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	cmd := m.Show("Copied!")
	if cmd == nil {
		t.Fatal("expected non-nil dismiss/open cmd batch")
	}
	if !m.IsActive() {
		t.Error("expected toast active after Show")
	}
	m.animator.Finalize()
	if !strings.Contains(m.RenderPopup(), "Copied!") {
		t.Errorf("expected popup to contain message, got %q", m.RenderPopup())
	}
}

func TestToastModel_MatchingDismissDeactivates(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	m.Show("hi")
	m.animator.Finalize()
	m.Update(toastDismissMsg{id: m.id})
	// Update kicks off the close animation; Finalize fast-forwards
	// past it so the test can assert the post-animation steady state.
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected toast inactive after matching dismiss + animation finalize")
	}
}

// tdp F7, D4: a message longer than the popup width is cut, not let
// through to widen the toast past every other popup.
func TestToastModel_LongMessageCutAtPopupWidth(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	m.SetSize(80)
	m.Show(strings.Repeat("x", 200))
	m.animator.Finalize()
	for i, l := range strings.Split(m.RenderPopup(), "\n") {
		if w := lipgloss.Width(l); w != 78 {
			t.Errorf("row %d is %d wide, want 78 (min(80 − 2, 120))", i, w)
		}
	}
}

func TestToastModel_StaleDismissIgnored(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	m.Show("first")
	staleID := m.id
	m.Show("second") // bumps id; stale tick from "first" should now be ignored
	m.animator.Finalize()
	m.Update(toastDismissMsg{id: staleID})
	if !m.IsActive() {
		t.Error("expected toast still active after stale dismiss")
	}
	if !strings.Contains(m.RenderPopup(), "second") {
		t.Errorf("expected popup to show latest message, got %q", m.RenderPopup())
	}
}
