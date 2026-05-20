package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

const (
	toastDuration = 1 * time.Second
	popupGlyph    = "󰵅" // Nerd Font glyph (PUA U+F19CC), shared by popup titles
	toastTitle    = popupGlyph + " "
)

// ToastModel renders a transient centered popup that auto-dismisses.
// It is non-blocking: keys still reach the underlying panels.
type ToastModel struct {
	active  bool
	message string
	id      int // generation counter, so stale Tick fires are ignored
	theme   *theme.Theme
}

type toastDismissMsg struct{ id int }

func NewToastModel(t *theme.Theme) ToastModel {
	return ToastModel{theme: t}
}

func (m ToastModel) IsActive() bool { return m.active }

// Show activates the toast with the given message and returns a Cmd that
// will dismiss it after toastDuration. Calling Show again while a toast is
// already active replaces the message and resets the timer (old tick fires
// are dropped via the id check in Update).
func (m *ToastModel) Show(message string) tea.Cmd {
	m.active = true
	m.message = message
	m.id++
	id := m.id
	return tea.Tick(toastDuration, func(time.Time) tea.Msg {
		return toastDismissMsg{id: id}
	})
}

func (m *ToastModel) Update(msg tea.Msg) {
	if dismiss, ok := msg.(toastDismissMsg); ok && dismiss.id == m.id {
		m.active = false
	}
}

func (m ToastModel) RenderPopup() string {
	if !m.active {
		return ""
	}
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	titleW := lipgloss.Width(toastTitle)
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
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(toastTitle) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

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
