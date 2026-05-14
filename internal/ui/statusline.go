package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type StatusLineModel struct {
	activePanel Panel
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

func (m *StatusLineModel) SetWidth(width int) {
	m.width = width
}

func (m StatusLineModel) View() string {
	var hints string
	switch m.activePanel {
	case SidebarPanel:
		hints = " j/k: navigate  Enter/l: select  h: collapse  Tab: switch panel  q: quit"
	case TablePanel:
		hints = " j/k: navigate  gg/G: top/bottom  Tab: switch panel  q: quit"
	case DetailPanel:
		hints = " Tab: switch panel  q: quit"
	}

	bar := m.theme.StatusLineStyle().
		Width(m.width).
		Render(hints)

	return bar
}

func (m StatusLineModel) Height() int {
	return lipgloss.Height(m.View())
}

func (m StatusLineModel) String() string {
	return fmt.Sprintf("StatusLine(panel=%d)", m.activePanel)
}
