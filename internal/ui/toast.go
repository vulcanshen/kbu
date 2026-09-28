package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

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
	// tdp D3 requires `glyph + text` in border titles;
	// the level glyph + a stable "kbu" identifier tell the user "your
	// app is talking" without leaking per-toast specifics into chrome.
	toastTitleText = "kbu"
)

// ToastModel renders a transient popup that auto-dismisses. It is
// non-blocking: keys still reach the underlying panels.
type ToastModel struct {
	screenW     int
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

// SetSize records the screen width the toast's width derives from.
func (m *ToastModel) SetSize(w int) { m.screenW = w }

// Owns reports whether the toast is showing and not yet fading out —
// the only state in which Esc is its to take (tdp F3).
func (m ToastModel) Owns() bool { return m.animator.Owns() }

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

// Dismiss begins the close animation. Caller chains the returned cmd
// into its own tea.Batch — fire-and-forget Dismiss without chaining
// drops the close-animation tick.
func (m *ToastModel) Dismiss() tea.Cmd {
	m.id++
	return m.animator.Close()
}

func (m *ToastModel) show(level toastLevel, message string) tea.Cmd {
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

// toastHint returns the hint-bar text: "auto-dismiss", so the absence
// of a dismiss key reads as design, not omission (Esc still takes it
// down at once, tdp F3).
func (m ToastModel) toastHint() string {
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

	// tdp F7: the toast takes the same width as every popup; a message
	// longer than that is cut.
	innerW := popupInnerWidth(m.screenW)

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
	if lipgloss.Width(bodyText) > innerW {
		bodyText = ansi.Truncate(bodyText, innerW-1, "") + "…"
	}
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
