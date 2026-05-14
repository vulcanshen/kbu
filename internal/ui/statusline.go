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
		hints = " 1 Sidebar │ j/k: navigate  n: namespace  q: quit"
	case TablePanel:
		hints = " 2 List │ j/k: navigate  gg/G: top/bottom  n: namespace  q: quit"
	case DetailPanel:
		hints = " 3 Detail │ j/k: scroll  [/]: tab  n: namespace  q: quit"
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
