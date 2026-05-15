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

func (m StatusLineModel) View() string {
	return m.ViewWithError(0, "")
}

func (m StatusLineModel) ViewWithError(unreadErrors int, lastError string) string {
	var hints string
	switch m.activePanel {
	case SidebarPanel:
		hints = " [1] n: ns | c: ctx | e: edit"
	case TablePanel:
		if m.drillDown {
			hints = " [2] /: search | h/l: tab | s: shell | esc: back"
		} else {
			hints = " [2] /: search | h/l: tab | s: shell | e: edit | D: delete"
		}
	case DetailPanel:
		hints = " [3] h/l: tab | +/-: expand"
	}

	barStyle := m.theme.StatusBarStyle().Padding(0, 0)

	if unreadErrors > 0 && lastError != "" {
		errText := lastError
		hintsWidth := lipgloss.Width(hints)
		maxErrLen := m.width - hintsWidth - 4
		if maxErrLen > 10 {
			if len(errText) > maxErrLen {
				errText = errText[:maxErrLen-1] + "…"
			}
			leftPart := barStyle.Width(hintsWidth + 2).Render(hints)
			errPart := lipgloss.NewStyle().
				Foreground(lipgloss.Color(m.theme.Status.Error)).
				Width(m.width - hintsWidth - 2).
				Render(" " + errText)
			return leftPart + errPart
		}
	}

	return barStyle.Width(m.width).Render(hints)
}
