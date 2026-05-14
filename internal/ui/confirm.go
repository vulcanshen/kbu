package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type ConfirmAction int

const (
	ConfirmShellExec ConfirmAction = iota
	ConfirmDelete
)

type ConfirmModel struct {
	active    bool
	action    ConfirmAction
	message   string
	detail    string
	width     int
	height    int
	theme     *theme.Theme
	onConfirm tea.Cmd
}

func NewConfirmModel(t *theme.Theme) ConfirmModel {
	return ConfirmModel{theme: t}
}

func (m *ConfirmModel) Show(action ConfirmAction, message, detail string, onConfirm tea.Cmd) {
	m.active = true
	m.action = action
	m.message = message
	m.detail = detail
	m.onConfirm = onConfirm
}

func (m *ConfirmModel) Close() {
	m.active = false
	m.onConfirm = nil
}

func (m ConfirmModel) IsActive() bool { return m.active }

func (m *ConfirmModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			cmd := m.onConfirm
			m.active = false
			m.onConfirm = nil
			return m, cmd
		case "esc", "n", "q":
			m.active = false
			m.onConfirm = nil
			return m, nil
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	bc := lipgloss.Color(m.theme.Sidebar.CategoryFg)
	titleStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Pending))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusLine.Foreground))

	boxW := 50
	if boxW > m.width-4 {
		boxW = m.width - 4
	}

	var lines []string
	lines = append(lines, titleStyle.Render(" "+m.message))
	lines = append(lines, "")
	if m.detail != "" {
		lines = append(lines, detailStyle.Render(" "+m.detail))
		lines = append(lines, "")
	}
	lines = append(lines, hintStyle.Render(" Enter/y: confirm | Esc/n: cancel"))

	content := strings.Join(lines, "\n")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bc).
		Padding(1, 2).
		Width(boxW).
		Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay)
}
