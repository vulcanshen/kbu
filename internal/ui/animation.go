package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PopupAnimState int

const (
	PopupClosed PopupAnimState = iota
	PopupOpeningLine
	PopupOpeningExpand
	PopupOpen
	PopupClosingCompress
	PopupClosingLine

	// PopupSwappingCompress / PopupSwappingExpand drive the mini
	// "yawn" animation when a popup's content is replaced in place
	// (e.g. listpicker chain step: column → direction). Compress
	// shrinks from 100% to 50% vertical height; expand grows back.
	// Total ~120ms split evenly between the two phases.
	PopupSwappingCompress
	PopupSwappingExpand
)

const (
	animFrameDuration = 20 * time.Millisecond
	animLineFrames    = 4
	animExpandFrames  = 4
	swapHalfFrames    = 3 // each swap phase runs this many ticks
)

// AnimTickMsg drives popup animations. Target identifies which popup.
type AnimTickMsg struct {
	Target string
}

// PopupAnimator wraps a popup's rendering with open/close animations.
type PopupAnimator struct {
	State  PopupAnimState
	Frame  int
	Target string
	Color  lipgloss.Color
}

func NewPopupAnimator(target string, color lipgloss.Color) PopupAnimator {
	return PopupAnimator{
		State:  PopupClosed,
		Target: target,
		Color:  color,
	}
}

// IsActive reports whether the popup should be drawn (any non-closed state).
func (a PopupAnimator) IsActive() bool {
	return a.State != PopupClosed
}

// IsInteractive reports whether the popup should accept input.
func (a PopupAnimator) IsInteractive() bool {
	return a.State == PopupOpen
}

// Open begins the opening animation. No-op if already opening/open.
func (a *PopupAnimator) Open() tea.Cmd {
	if a.State == PopupOpen || a.State == PopupOpeningLine || a.State == PopupOpeningExpand {
		return nil
	}
	a.State = PopupOpeningLine
	a.Frame = 0
	return a.tickCmd()
}

// Close begins the closing animation. No-op if already closing/closed.
func (a *PopupAnimator) Close() tea.Cmd {
	if a.State == PopupClosed || a.State == PopupClosingCompress || a.State == PopupClosingLine {
		return nil
	}
	a.State = PopupClosingCompress
	a.Frame = 0
	return a.tickCmd()
}

// Swap begins the mini in-place content-swap animation. Only valid
// when the popup is fully open; no-op otherwise. The animation
// runs Compress → midpoint → Expand and returns to PopupOpen. The
// caller is expected to detect the Compress → Expand transition
// (e.g. via state inspection in HandleTick) and substitute new
// content there, so the user perceives the swap as the popup
// briefly "yawning" with new content already inside.
func (a *PopupAnimator) Swap() tea.Cmd {
	if a.State != PopupOpen {
		return nil
	}
	a.State = PopupSwappingCompress
	a.Frame = 0
	return a.tickCmd()
}

// Finalize completes any in-progress animation immediately. Used by tests.
func (a *PopupAnimator) Finalize() {
	switch a.State {
	case PopupOpeningLine, PopupOpeningExpand:
		a.State = PopupOpen
		a.Frame = 0
	case PopupClosingCompress, PopupClosingLine:
		a.State = PopupClosed
		a.Frame = 0
	case PopupSwappingCompress, PopupSwappingExpand:
		a.State = PopupOpen
		a.Frame = 0
	}
}

// Tick advances the animation by one frame. Returns the next tick cmd or nil.
func (a *PopupAnimator) Tick() tea.Cmd {
	a.Frame++
	switch a.State {
	case PopupOpeningLine:
		if a.Frame >= animLineFrames {
			a.State = PopupOpeningExpand
			a.Frame = 0
		}
		return a.tickCmd()
	case PopupOpeningExpand:
		if a.Frame >= animExpandFrames {
			a.State = PopupOpen
			a.Frame = 0
			return nil
		}
		return a.tickCmd()
	case PopupClosingCompress:
		if a.Frame >= animExpandFrames {
			a.State = PopupClosingLine
			a.Frame = 0
		}
		return a.tickCmd()
	case PopupClosingLine:
		if a.Frame >= animLineFrames {
			a.State = PopupClosed
			a.Frame = 0
			return nil
		}
		return a.tickCmd()
	case PopupSwappingCompress:
		if a.Frame >= swapHalfFrames {
			a.State = PopupSwappingExpand
			a.Frame = 0
		}
		return a.tickCmd()
	case PopupSwappingExpand:
		if a.Frame >= swapHalfFrames {
			a.State = PopupOpen
			a.Frame = 0
			return nil
		}
		return a.tickCmd()
	}
	return nil
}

func (a PopupAnimator) tickCmd() tea.Cmd {
	target := a.Target
	return tea.Tick(animFrameDuration, func(t time.Time) tea.Msg {
		return AnimTickMsg{Target: target}
	})
}

// RenderFrame transforms a fully-rendered popup according to current animation state.
// During PopupOpen it returns the popup unchanged. During PopupClosed it returns "".
func (a PopupAnimator) RenderFrame(fullPopup string) string {
	if a.State == PopupClosed {
		return ""
	}
	if a.State == PopupOpen {
		return fullPopup
	}

	width := lipgloss.Width(fullPopup)
	lines := strings.Split(fullPopup, "\n")
	height := len(lines)
	style := lipgloss.NewStyle().Foreground(a.Color)

	switch a.State {
	case PopupOpeningLine:
		// Grow horizontally from center: 1/N → N/N of width.
		progress := float64(a.Frame+1) / float64(animLineFrames)
		lineWidth := int(float64(width) * progress)
		if lineWidth < 1 {
			lineWidth = 1
		}
		if lineWidth > width {
			lineWidth = width
		}
		return style.Render(strings.Repeat("─", lineWidth))

	case PopupClosingLine:
		// Shrink horizontally to center.
		progress := 1.0 - float64(a.Frame+1)/float64(animLineFrames)
		lineWidth := int(float64(width) * progress)
		if lineWidth < 1 {
			lineWidth = 1
		}
		if lineWidth > width {
			lineWidth = width
		}
		return style.Render(strings.Repeat("─", lineWidth))

	case PopupOpeningExpand:
		// Vertical reveal from center outward.
		progress := float64(a.Frame+1) / float64(animExpandFrames)
		visibleHeight := int(float64(height) * progress)
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		if visibleHeight > height {
			visibleHeight = height
		}
		return centerSlice(lines, visibleHeight)

	case PopupClosingCompress:
		// Vertical compress toward center.
		progress := 1.0 - float64(a.Frame+1)/float64(animExpandFrames)
		visibleHeight := int(float64(height) * progress)
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		if visibleHeight > height {
			visibleHeight = height
		}
		return centerSlice(lines, visibleHeight)

	case PopupSwappingCompress:
		// Mini compress: 100% → 50% height. Stops at 50% (the
		// midpoint where the caller swaps content); never hits the
		// border line state. Same centerSlice rendering as the full
		// close so the visual reads consistent.
		progress := 1.0 - 0.5*float64(a.Frame+1)/float64(swapHalfFrames)
		visibleHeight := int(float64(height) * progress)
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		if visibleHeight > height {
			visibleHeight = height
		}
		return centerSlice(lines, visibleHeight)

	case PopupSwappingExpand:
		// Mini expand: 50% → 100% height. Mirror of compress.
		progress := 0.5 + 0.5*float64(a.Frame+1)/float64(swapHalfFrames)
		visibleHeight := int(float64(height) * progress)
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		if visibleHeight > height {
			visibleHeight = height
		}
		return centerSlice(lines, visibleHeight)
	}

	return fullPopup
}

// centerSlice returns n lines centered around the middle of lines.
func centerSlice(lines []string, n int) string {
	height := len(lines)
	if n >= height {
		return strings.Join(lines, "\n")
	}
	start := (height - n) / 2
	end := start + n
	return strings.Join(lines[start:end], "\n")
}
