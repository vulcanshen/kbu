package ui

import (
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
	var hints string
	switch m.activePanel {
	case SidebarPanel:
		hints = " [1] Sidebar | n: ns | c: ctx | e: edit"
	case TablePanel:
		if m.drillDown {
			hints = " [2] Containers | /: search | h/l: tab | esc: back"
		} else {
			hints = " [2] List | /: search | h/l: tab | e: edit"
		}
	case DetailPanel:
		hints = " [3] Detail | h/l: tab | +/-: expand"
	}

	return m.theme.StatusBarStyle().
		Width(m.width).
		Render(hints)
}
