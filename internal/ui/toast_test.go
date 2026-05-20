package ui

import (
	"strings"
	"testing"

	"github.com/vulcanshen/km8/internal/theme"
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
		t.Fatal("expected non-nil dismiss cmd")
	}
	if !m.IsActive() {
		t.Error("expected toast active after Show")
	}
	if !strings.Contains(m.RenderPopup(), "Copied!") {
		t.Errorf("expected popup to contain message, got %q", m.RenderPopup())
	}
}

func TestToastModel_MatchingDismissDeactivates(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	m.Show("hi")
	m.Update(toastDismissMsg{id: m.id})
	if m.IsActive() {
		t.Error("expected toast inactive after matching dismiss")
	}
}

func TestToastModel_StaleDismissIgnored(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	m.Show("first")
	staleID := m.id
	m.Show("second") // bumps id; stale tick from "first" should now be ignored
	m.Update(toastDismissMsg{id: staleID})
	if !m.IsActive() {
		t.Error("expected toast still active after stale dismiss")
	}
	if !strings.Contains(m.RenderPopup(), "second") {
		t.Errorf("expected popup to show latest message, got %q", m.RenderPopup())
	}
}
