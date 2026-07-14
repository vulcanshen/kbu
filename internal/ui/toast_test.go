package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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

func TestToastModel_ShowStickyHasNoDismissCmd(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	cmd := m.ShowSticky("drag mode hint")
	if cmd == nil {
		t.Fatal("ShowSticky must still return the animator open cmd")
	}
	// Sticky toast must NOT batch a toastDismissMsg timer — the only
	// msg the cmd can produce is AnimTickMsg from the open animation.
	if msg := cmd(); msg == nil {
		t.Fatal("open cmd must produce a msg")
	} else if _, isDismiss := msg.(toastDismissMsg); isDismiss {
		t.Errorf("sticky toast must NOT schedule an auto-dismiss cmd, got %T", msg)
	}
	if !m.IsActive() {
		t.Error("sticky toast must be active immediately")
	}
}

func TestToastModel_StickyOutlivesStaleTick(t *testing.T) {
	// Sticky toast must survive a stale toastDismissMsg arriving
	// after it goes up — e.g. a prior transient toast's tick
	// firing late. ShowSticky bumps id so prior id's dismiss is
	// stale.
	m := NewToastModel(theme.DefaultTheme())
	m.Show("first")
	staleID := m.id
	m.ShowSticky("drag mode hint")
	m.animator.Finalize()
	m.Update(toastDismissMsg{id: staleID})
	if !m.IsActive() {
		t.Error("sticky toast must survive a stale dismiss msg from a prior toast")
	}
}

func TestToastModel_DismissTakesStickyDown(t *testing.T) {
	m := NewToastModel(theme.DefaultTheme())
	m.ShowSticky("drag mode hint")
	m.animator.Finalize()
	m.Dismiss()
	// Sticky flag clears synchronously — visual state goes through
	// the close animation; Finalize fast-forwards past it.
	if m.IsSticky() {
		t.Error("Dismiss() must clear the sticky flag")
	}
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("Dismiss() must deactivate the toast (after animation completes)")
	}
}

func TestToastModel_StickyFlagDistinguishesShowVsShowSticky(t *testing.T) {
	// IsSticky() drives View()'s render-order pick — must flip
	// correctly between the two Show variants.
	m := NewToastModel(theme.DefaultTheme())
	m.Show("transient")
	if m.IsSticky() {
		t.Error("Show() must NOT mark the toast as sticky")
	}
	m.ShowSticky("background hint")
	if !m.IsSticky() {
		t.Error("ShowSticky() must mark the toast as sticky")
	}
	// Switching back to Show clears sticky.
	m.Show("interrupt")
	if m.IsSticky() {
		t.Error("Show() must reset sticky to false when transitioning from a sticky")
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
