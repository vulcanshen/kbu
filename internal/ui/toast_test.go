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

func TestToastModel_ShortMessagePadsToMinWidth(t *testing.T) {
	// v1.7.10 UX polish: short auto-dismiss messages ("Copied!" and
	// friends) used to size the popup down to the hint-bar floor and
	// looked cramped. toastMinInnerW gives every toast a consistent
	// visual weight regardless of payload length. Assert the render
	// has at least that many cells in the body row so a short message
	// doesn't collapse into the chrome.
	th := theme.DefaultTheme()
	m := NewToastModel(th)
	m.Show("Copied!") // 7-cell payload
	m.animator.Finalize()

	view := m.RenderPopup()
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines in toast render, got %d", len(lines))
	}
	// Body row is the middle line (top border + pad + body + pad + bottom).
	// Pick the widest to be safe against future format tweaks; all rows
	// should be innerW wide (+ 2 border chars).
	widest := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > widest {
			widest = w
		}
	}
	// innerW >= toastMinInnerW → whole popup width >= toastMinInnerW + 2.
	minPopupW := toastMinInnerW + 2
	if widest < minPopupW {
		t.Errorf("expected toast width >= %d for short message, got %d\n%s",
			minPopupW, widest, view)
	}
}

func TestToastModel_LongMessageStillGrowsPastMinWidth(t *testing.T) {
	// Regression: the min-width floor must not clamp long messages.
	th := theme.DefaultTheme()
	m := NewToastModel(th)
	long := strings.Repeat("x", 60)
	m.Show(long)
	m.animator.Finalize()

	view := m.RenderPopup()
	if !strings.Contains(view, long) {
		t.Errorf("long message must survive into render, got:\n%s", view)
	}
	widest := 0
	for _, l := range strings.Split(view, "\n") {
		if w := lipgloss.Width(l); w > widest {
			widest = w
		}
	}
	if widest < 62 { // 60 payload + 2 padding
		t.Errorf("long message should grow toast past min floor; got width %d", widest)
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
