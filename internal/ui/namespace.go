package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type NamespacePickerModel struct {
	namespaces []string
	cursor     int
	active     bool
	width      int
	height     int
	theme      *theme.Theme
}

func NewNamespacePickerModel(t *theme.Theme) NamespacePickerModel {
	return NamespacePickerModel{
		theme: t,
	}
}

func (m *NamespacePickerModel) Open(namespaces []string) {
	all := []string{"All Namespaces"}
	m.namespaces = append(all, namespaces...)
	m.cursor = 0
	m.active = true
}

func (m *NamespacePickerModel) Close() {
	m.active = false
}

func (m *NamespacePickerModel) IsActive() bool {
	return m.active
}

func (m *NamespacePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m NamespacePickerModel) Update(msg tea.Msg) (NamespacePickerModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.namespaces)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			ns := ""
			if m.cursor > 0 {
				ns = m.namespaces[m.cursor]
			}
			m.active = false
			return m, func() tea.Msg {
				return NamespaceChangedMsg{Namespace: ns}
			}
		case "esc", "n":
			m.active = false
			return m, nil
		}
	}

	return m, nil
}

func (m NamespacePickerModel) View() string {
	if !m.active {
		return ""
	}

	titleStyle := m.theme.DetailTabActiveStyle()
	selectedStyle := m.theme.SidebarSelectedStyle()
	normalStyle := m.theme.SidebarStyle()

	boxWidth := 40
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}

	var lines []string
	lines = append(lines, titleStyle.Width(boxWidth).Align(lipgloss.Center).Render("Select Namespace"))
	lines = append(lines, "")

	maxVisible := m.height - 6
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.namespaces) {
		end = len(m.namespaces)
	}

	for i := start; i < end; i++ {
		label := m.namespaces[i]
		if i == m.cursor {
			lines = append(lines, selectedStyle.Width(boxWidth).Render(" "+label))
		} else {
			lines = append(lines, normalStyle.Width(boxWidth).Render(" "+label))
		}
	}

	lines = append(lines, "")
	hintStyle := m.theme.StatusLineStyle()
	lines = append(lines, hintStyle.Render(" j/k: navigate  Enter: select  Esc: cancel"))

	content := strings.Join(lines, "\n")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Detail.BorderColor)).
		Padding(1, 2).
		Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay)
}
