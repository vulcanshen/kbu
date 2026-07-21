package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/kbu/internal/theme"
)

// toastLevel categorizes toasts by severity. Each level picks its own
// duration, glyph, and border color so a glance is enough to tell
// "casual confirmation" from "something didn't work" — without the
// caller having to remember the styling.
//
// Add toastError when the first error caller lands; until then keep the
// surface small (YAGNI).
type toastLevel int

const (
	toastInfo toastLevel = iota
	toastWarn
)

const (
	toastInfoDuration = 1 * time.Second
	toastWarnDuration = 2 * time.Second

	toastInfoGlyph = "󰵅"
	toastWarnGlyph = "󰀦"

	// toastWarnColor — Catppuccin Peach. Warning is the only toast
	// level that overrides the popup-layer scheme; the peach signal
	// "something didn't work" takes precedence over the layer color
	// so the user catches it at a glance.
	toastWarnColor = "#fab387"

	// toastTitleText is the fixed title text for every toast — the
	// popup-convention rule requires `glyph + text` in border titles;
	// the level glyph + a stable "km8" identifier tell the user "your
	// app is talking" without leaking per-toast specifics into chrome.
	toastTitleText = "kbu"

	// toastMinInnerW is the minimum cell budget for the toast body
	// row. Short messages like "Copied!" (7 cells) used to size the
	// popup down to the hint-bar floor (~14 cells) and looked cramped
	// on screen — the text sat swallowed by chrome. The 28-cell floor
	// gives every auto-dismiss / sticky toast a consistent visual
	// weight regardless of how short the payload is; long messages
	// still grow past this via the max() below.
	toastMinInnerW = 28
)

// ToastModel renders a transient centered popup that auto-dismisses.
// It is non-blocking: keys still reach the underlying panels.
//
// sticky distinguishes background-reminder toasts (sticky=true, set
// up BEFORE the user opens any popup — e.g. drag mode's keyboard
// contract) from transient interrupts (sticky=false, fired AS A
// RESULT of user action). View() uses this to decide rendering
// order vs the popup stack: sticky sits BELOW popups (a popup the
// user just opened should win over a pre-existing background hint);
// non-sticky sits ABOVE popups (a freshly-fired error or status
// should interrupt whatever popup is on screen).
type ToastModel struct {
	sticky      bool
	level       toastLevel
	message     string
	id          int // generation counter, so stale Tick fires are ignored
	theme       *theme.Theme
	animator    PopupAnimator
	layer       int
	borderColor lipgloss.Color
}

type toastDismissMsg struct{ id int }

func NewToastModel(t *theme.Theme) ToastModel {
	bc := theme.PopupLayerColor(1)
	return ToastModel{
		theme:       t,
		animator:    NewPopupAnimator("toast", bc),
		borderColor: bc,
		layer:       1,
	}
}

// SetLayer stamps nesting depth + derives the info/sticky border
// color from the popup-layer scale. Warn toasts override to Peach
// at show time (toastBorderColor); SetLayer's layer color only
// applies when level != toastWarn.
func (m *ToastModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	// Warn keeps Peach regardless of layer; info / sticky pick up
	// the new layer color immediately.
	if m.level != toastWarn {
		m.animator.Color = m.borderColor
	}
}

// IsActive reports whether the toast frame should be drawn. Includes
// the closing-animation window so the popup fades out instead of
// snapping away when the dismiss timer fires.
func (m ToastModel) IsActive() bool { return m.animator.IsActive() }
func (m ToastModel) IsSticky() bool { return m.sticky }

// Show is the info-level toast — short reminders, "Copied!", PTY hints.
// 1s duration, popup-layer border (stamped via SetLayer).
func (m *ToastModel) Show(message string) tea.Cmd {
	return m.show(toastInfo, message)
}

// ShowWarn is the warning-level toast — something the user tried got
// blocked or failed (cycle blocked, drill failed, ...). 2s duration so
// there's time to read the reason, peach border + warning glyph for at-a-
// glance distinction from a casual info toast.
func (m *ToastModel) ShowWarn(message string) tea.Cmd {
	return m.show(toastWarn, message)
}

// ShowSticky displays an info-level toast that does NOT auto-dismiss —
// caller MUST call Dismiss() to take it down. Used for modal states
// where the toast is the persistent visual contract (e.g. sidebar
// drag mode — the keyboard contract stays on screen until commit /
// cancel). Increments id so any prior in-flight auto-dismiss tick
// becomes stale and won't take this sticky one down.
func (m *ToastModel) ShowSticky(message string) tea.Cmd {
	m.sticky = true
	m.level = toastInfo
	m.message = message
	m.id++
	m.animator.Color = m.borderColor
	return m.animator.Open()
}

// Dismiss begins the close animation. Caller chains the returned cmd
// into its own tea.Batch — fire-and-forget Dismiss without chaining
// drops the close-animation tick.
func (m *ToastModel) Dismiss() tea.Cmd {
	m.sticky = false
	m.id++
	return m.animator.Close()
}

func (m *ToastModel) show(level toastLevel, message string) tea.Cmd {
	m.sticky = false
	m.level = level
	m.message = message
	m.id++
	id := m.id
	m.animator.Color = m.toastBorderColor()
	dismissCmd := tea.Tick(toastDuration(level), func(time.Time) tea.Msg {
		return toastDismissMsg{id: id}
	})
	return tea.Batch(m.animator.Open(), dismissCmd)
}

// toastBorderColor picks the active border color: warn always Peach,
// info / sticky use the popup-layer color stamped via SetLayer.
func (m ToastModel) toastBorderColor() lipgloss.Color {
	if m.level == toastWarn {
		return lipgloss.Color(toastWarnColor)
	}
	return m.borderColor
}

// Update routes toastDismissMsg into the close animation. Returns the
// close tick cmd so the caller can batch it into the main loop —
// previously Update had no return because dismiss was synchronous.
func (m *ToastModel) Update(msg tea.Msg) tea.Cmd {
	if dismiss, ok := msg.(toastDismissMsg); ok && dismiss.id == m.id {
		return m.animator.Close()
	}
	return nil
}

func (m *ToastModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func toastDuration(level toastLevel) time.Duration {
	if level == toastWarn {
		return toastWarnDuration
	}
	return toastInfoDuration
}

func toastGlyph(level toastLevel) string {
	if level == toastWarn {
		return toastWarnGlyph
	}
	return toastInfoGlyph
}

// toastHint returns the hint-bar text. Sticky toasts include the
// keyboard escape so the user always knows how to take down a
// background-mode reminder; transient toasts surface "auto-dismiss"
// so the absence of a dismiss key reads as design, not omission.
func (m ToastModel) toastHint() string {
	if m.sticky {
		return " Esc: close "
	}
	return " auto-dismiss "
}

func (m ToastModel) RenderPopup() string {
	if !m.animator.IsActive() {
		return ""
	}
	bc := m.toastBorderColor()
	glyph := toastGlyph(m.level)
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))

	title := fmt.Sprintf(" %s %s ", glyph, toastTitleText)
	titleW := lipgloss.Width(title)
	hint := m.toastHint()
	hintW := lipgloss.Width(hint)

	// innerW must fit the widest of: title (+ 2 lead dashes + 1 trail
	// minimum), message body (+2 padding 1 each side), the bottom
	// hint, AND the toastMinInnerW floor so short messages don't
	// visually collapse into the chrome. Taking the max keeps the
	// borders straight across all rows.
	innerW := toastMinInnerW
	if w := titleW + 3; w > innerW {
		innerW = w
	}
	if hintW > innerW {
		innerW = hintW
	}
	if w := lipgloss.Width(m.message) + 2; w > innerW {
		innerW = w
	}

	leadDashCount := 2
	trailDashCount := innerW - leadDashCount - titleW
	if trailDashCount < 1 {
		trailDashCount = 1
	}
	top := bStyle.Render("╭"+strings.Repeat("─", leadDashCount)) +
		tStyle.Render(title) +
		bStyle.Render(strings.Repeat("─", trailDashCount)+"╮")

	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right

	bodyText := " " + m.message + " "
	bw := lipgloss.Width(bodyText)
	if bw < innerW {
		bodyText += strings.Repeat(" ", innerW-bw)
	}
	bodyRow := left + bodyText + right

	tail := innerW - hintW
	if tail < 1 {
		tail = 1
	}
	bot := bStyle.Render("╰") + hintStyle.Render(hint) +
		bStyle.Render(strings.Repeat("─", tail)+"╯")

	frame := strings.Join([]string{top, padRow, bodyRow, padRow, bot}, "\n")
	return m.animator.RenderFrame(frame)
}
