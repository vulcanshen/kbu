package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type StatusLineModel struct {
	activePanel Panel
	drillDown   bool
	width       int
	theme       *theme.Theme
	lastError   string
}

func NewStatusLineModel(t *theme.Theme) StatusLineModel {
	return StatusLineModel{
		activePanel: SidebarPanel,
		theme:       t,
	}
}

func (m *StatusLineModel) SetActivePanel(p Panel) {
	m.activePanel = p
}

func (m *StatusLineModel) SetDrillDown(d bool) {
	m.drillDown = d
}

func (m *StatusLineModel) SetWidth(width int) {
	m.width = width
}

func (m *StatusLineModel) SetLastError(msg string) {
	m.lastError = msg
}

func (m StatusLineModel) View() string {
	var hints string
	switch m.activePanel {
	case SidebarPanel:
		hints = " [1] Sidebar | n: ns | c: ctx | e: edit"
	case TablePanel:
		if m.drillDown {
			hints = " [2] Containers | /: search | h/l: tab | s: shell | esc: back"
		} else {
			hints = " [2] List | /: search | h/l: tab | s: shell | e: edit | d: delete"
		}
	case DetailPanel:
		hints = " [3] Detail | h/l: tab | +/-: expand"
	}

	if m.lastError != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8"))
		errText := m.lastError
		maxLen := m.width - lipgloss.Width(hints) - 4
		if maxLen > 0 && len(errText) > maxLen {
			errText = errText[:maxLen-1] + "…"
		}
		if maxLen > 0 {
			hints += " " + errStyle.Render(errText)
		}
	}

	return m.theme.StatusBarStyle().
		Width(m.width).
		Render(hints)
}
