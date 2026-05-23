package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
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

	toastInfoGlyph = "󰵅 "
	toastWarnGlyph = "󰀦 "

	toastInfoColor = "#74c7ec" // Catppuccin Sky — neutral confirmation
	toastWarnColor = "#fab387" // Catppuccin Peach — caution, action blocked
)

// ToastModel renders a transient centered popup that auto-dismisses.
// It is non-blocking: keys still reach the underlying panels.
type ToastModel struct {
	active  bool
	level   toastLevel
	message string
	id      int // generation counter, so stale Tick fires are ignored
	theme   *theme.Theme
}

type toastDismissMsg struct{ id int }

func NewToastModel(t *theme.Theme) ToastModel {
	return ToastModel{theme: t}
}

func (m ToastModel) IsActive() bool { return m.active }

// Show is the info-level toast — short reminders, "Copied!", PTY hints.
// 1s duration, sky-blue border.
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

func (m *ToastModel) show(level toastLevel, message string) tea.Cmd {
	m.active = true
	m.level = level
	m.message = message
	m.id++
	id := m.id
	return tea.Tick(toastDuration(level), func(time.Time) tea.Msg {
		return toastDismissMsg{id: id}
	})
}

func (m *ToastModel) Update(msg tea.Msg) {
	if dismiss, ok := msg.(toastDismissMsg); ok && dismiss.id == m.id {
		m.active = false
	}
}

func toastDuration(level toastLevel) time.Duration {
	if level == toastWarn {
		return toastWarnDuration
	}
	return toastInfoDuration
}

func toastBorderColor(level toastLevel) lipgloss.Color {
	if level == toastWarn {
		return lipgloss.Color(toastWarnColor)
	}
	return lipgloss.Color(toastInfoColor)
}

func toastGlyph(level toastLevel) string {
	if level == toastWarn {
		return toastWarnGlyph
	}
	return toastInfoGlyph
}

func (m ToastModel) RenderPopup() string {
	if !m.active {
		return ""
	}
	bc := toastBorderColor(m.level)
	glyph := toastGlyph(m.level)
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	titleW := lipgloss.Width(glyph)
	contentW := lipgloss.Width(m.message) + 2 // padding 1 each side
	if w := titleW + 4; w > contentW {
		contentW = w
	}
	innerW := contentW

	dashesAfter := innerW - 1 - titleW
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(glyph) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	body := " " + m.message + " "
	pad := ""
	if w := lipgloss.Width(body); w < innerW {
		pad = strings.Repeat(" ", innerW-w)
	}
	b.WriteString(leftBorder + body + pad + rightBorder + "\n")

	b.WriteString(bStyle.Render("╰" + strings.Repeat("─", innerW) + "╯"))
	return b.String()
}
