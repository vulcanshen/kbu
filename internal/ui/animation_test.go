package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func newTestAnimator() PopupAnimator {
	return NewPopupAnimator("test", lipgloss.Color("#74c7ec"))
}

// ── Initial state ──────────────────────────────────────────────────────────

func TestPopupAnimator_InitialState(t *testing.T) {
	a := newTestAnimator()

	if a.IsActive() {
		t.Error("new animator should be inactive (Closed)")
	}
	if a.IsInteractive() {
		t.Error("new animator should not be interactive")
	}
	if a.State != PopupClosed {
		t.Errorf("expected state PopupClosed, got %v", a.State)
	}
}

// ── Open ───────────────────────────────────────────────────────────────────

func TestPopupAnimator_Open_SetsOpeningState(t *testing.T) {
	a := newTestAnimator()
	cmd := a.Open()

	if a.State != PopupOpeningLine {
		t.Errorf("Open() must set state to PopupOpeningLine, got %v", a.State)
	}
	if cmd == nil {
		t.Error("Open() must return a tick cmd")
	}
	if !a.IsActive() {
		t.Error("IsActive() must be true while opening")
	}
}

func TestPopupAnimator_Open_NoopWhenAlreadyOpen(t *testing.T) {
	a := newTestAnimator()
	a.Open()
	a.Finalize()

	cmd := a.Open() // already open — no-op
	if cmd != nil {
		t.Error("Open() on an already-open animator must be a no-op")
	}
	if a.State != PopupOpen {
		t.Errorf("state must remain PopupOpen, got %v", a.State)
	}
}

// ── Close ──────────────────────────────────────────────────────────────────

func TestPopupAnimator_Close_SetsClosingState(t *testing.T) {
	a := newTestAnimator()
	a.Open()
	a.Finalize()

	cmd := a.Close()

	if a.State != PopupClosingCompress {
		t.Errorf("Close() must set state to PopupClosingCompress, got %v", a.State)
	}
	if cmd == nil {
		t.Error("Close() must return a tick cmd")
	}
}

func TestPopupAnimator_Close_NoopWhenAlreadyClosed(t *testing.T) {
	a := newTestAnimator()

	cmd := a.Close()
	if cmd != nil {
		t.Error("Close() on an already-closed animator must be a no-op")
	}
}

// ── Finalize ───────────────────────────────────────────────────────────────

func TestPopupAnimator_Finalize_OpeningToOpen(t *testing.T) {
	a := newTestAnimator()
	a.Open()

	a.Finalize()

	if a.State != PopupOpen {
		t.Errorf("Finalize() during opening must set state to PopupOpen, got %v", a.State)
	}
	if !a.IsInteractive() {
		t.Error("IsInteractive() must be true after Finalize()")
	}
}

func TestPopupAnimator_Finalize_ClosingToClosed(t *testing.T) {
	a := newTestAnimator()
	a.Open()
	a.Finalize()
	a.Close()

	a.Finalize()

	if a.State != PopupClosed {
		t.Errorf("Finalize() during closing must set state to PopupClosed, got %v", a.State)
	}
	if a.IsActive() {
		t.Error("IsActive() must be false after close + Finalize()")
	}
}

// ── Tick ───────────────────────────────────────────────────────────────────

func TestPopupAnimator_Tick_AdvancesFrames(t *testing.T) {
	a := newTestAnimator()
	a.Open()

	// Tick through all opening frames.
	for i := 0; i < animLineFrames+animExpandFrames+10; i++ {
		cmd := a.Tick()
		if a.State == PopupOpen {
			if cmd != nil {
				t.Error("no tick cmd expected once fully open")
			}
			break
		}
		if cmd == nil {
			t.Errorf("tick must return next cmd during animation (frame %d, state %v)", i, a.State)
		}
	}

	if a.State != PopupOpen {
		t.Errorf("expected PopupOpen after all ticks, got %v", a.State)
	}
}

func TestPopupAnimator_Tick_ClosingReachesClosed(t *testing.T) {
	a := newTestAnimator()
	a.Open()
	a.Finalize()
	a.Close()

	for i := 0; i < animExpandFrames+animLineFrames+10; i++ {
		cmd := a.Tick()
		if a.State == PopupClosed {
			if cmd != nil {
				t.Error("no tick cmd expected once fully closed")
			}
			break
		}
		if cmd == nil {
			t.Errorf("tick must return next cmd during closing (frame %d, state %v)", i, a.State)
		}
	}

	if a.State != PopupClosed {
		t.Errorf("expected PopupClosed after all ticks, got %v", a.State)
	}
}

// ── IsInteractive ──────────────────────────────────────────────────────────

func TestPopupAnimator_IsInteractive_OnlyWhenOpen(t *testing.T) {
	a := newTestAnimator()

	if a.IsInteractive() {
		t.Error("must not be interactive when Closed")
	}

	a.Open()
	if a.IsInteractive() {
		t.Error("must not be interactive while opening")
	}

	a.Finalize()
	if !a.IsInteractive() {
		t.Error("must be interactive when Open")
	}

	a.Close()
	if a.IsInteractive() {
		t.Error("must not be interactive while closing")
	}
}

// ── RenderFrame ────────────────────────────────────────────────────────────

const testPopup = "╭────╮\n│ hi │\n╰────╯"

func TestPopupAnimator_RenderFrame_EmptyWhenClosed(t *testing.T) {
	a := newTestAnimator()
	out := a.RenderFrame(testPopup)
	if out != "" {
		t.Errorf("RenderFrame must return empty string when closed, got %q", out)
	}
}

func TestPopupAnimator_RenderFrame_FullWhenOpen(t *testing.T) {
	a := newTestAnimator()
	a.Open()
	a.Finalize()

	out := a.RenderFrame(testPopup)
	if out != testPopup {
		t.Errorf("RenderFrame must return full popup when open\ngot:  %q\nwant: %q", out, testPopup)
	}
}

func TestPopupAnimator_RenderFrame_PartialDuringAnimation(t *testing.T) {
	a := newTestAnimator()
	a.Open() // state = PopupOpeningLine, frame = 0

	out := a.RenderFrame(testPopup)
	if out == "" {
		t.Error("RenderFrame must render something during opening animation")
	}
	if out == testPopup {
		t.Error("RenderFrame must not render full popup during opening animation")
	}
}

// ── centerSlice ────────────────────────────────────────────────────────────

func TestCenterSlice_FewerThanTotal(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	out := centerSlice(lines, 3)
	got := strings.Split(out, "\n")
	if len(got) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(got), got)
	}
	// Center 3 of 5 → index 1,2,3.
	if got[0] != "b" || got[1] != "c" || got[2] != "d" {
		t.Errorf("expected [b c d], got %v", got)
	}
}

func TestCenterSlice_AllLines(t *testing.T) {
	lines := []string{"x", "y", "z"}
	out := centerSlice(lines, 3)
	if out != "x\ny\nz" {
		t.Errorf("expected all lines joined, got %q", out)
	}
}

func TestCenterSlice_MoreThanTotal(t *testing.T) {
	lines := []string{"a", "b"}
	out := centerSlice(lines, 10)
	if out != "a\nb" {
		t.Errorf("expected all lines when n >= height, got %q", out)
	}
}

func TestAnimator_Swap_TransitionsThroughBothPhases(t *testing.T) {
	// Swap from PopupOpen runs Compress → Expand and returns to
	// PopupOpen. Total tick count is swapHalfFrames * 2.
	a := NewPopupAnimator("test", "#ffffff")
	a.State = PopupOpen
	cmd := a.Swap()
	if cmd == nil {
		t.Fatal("Swap from PopupOpen must return a tick cmd")
	}
	if a.State != PopupSwappingCompress {
		t.Fatalf("Swap must set state to PopupSwappingCompress, got %v", a.State)
	}
	// Drive compress phase.
	for i := 0; i < swapHalfFrames; i++ {
		a.Tick()
	}
	if a.State != PopupSwappingExpand {
		t.Errorf("after swapHalfFrames ticks, state should be PopupSwappingExpand, got %v", a.State)
	}
	// Drive expand phase.
	for i := 0; i < swapHalfFrames; i++ {
		a.Tick()
	}
	if a.State != PopupOpen {
		t.Errorf("after full swap, state should be PopupOpen, got %v", a.State)
	}
}

func TestAnimator_Swap_NoOpWhenNotOpen(t *testing.T) {
	// Swap requires the popup be fully open. Calling it during any
	// other state (Closed / opening / closing) is a silent no-op so
	// the caller can blindly invoke Open which routes to either
	// Swap or fresh Open based on current state.
	for _, state := range []PopupAnimState{
		PopupClosed, PopupOpeningLine, PopupOpeningExpand,
		PopupClosingCompress, PopupClosingLine,
	} {
		a := NewPopupAnimator("test", "#ffffff")
		a.State = state
		cmd := a.Swap()
		if cmd != nil {
			t.Errorf("Swap from state %v must return nil cmd, got non-nil", state)
		}
		if a.State != state {
			t.Errorf("Swap from state %v must not change state, got %v", state, a.State)
		}
	}
}

func TestAnimator_Finalize_DuringSwap(t *testing.T) {
	// Finalize during a swap snaps back to PopupOpen — same end
	// state the animation reaches naturally. Used by tests to skip
	// the animation entirely.
	a := NewPopupAnimator("test", "#ffffff")
	a.State = PopupSwappingCompress
	a.Frame = 1
	a.Finalize()
	if a.State != PopupOpen {
		t.Errorf("Finalize during compress should land on PopupOpen, got %v", a.State)
	}
	a.State = PopupSwappingExpand
	a.Frame = 2
	a.Finalize()
	if a.State != PopupOpen {
		t.Errorf("Finalize during expand should land on PopupOpen, got %v", a.State)
	}
}
