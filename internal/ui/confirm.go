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
	return ""
}

func (m ConfirmModel) RenderPopup() string {
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	msgStyle := lipgloss.NewStyle().Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Pending))

	boxWidth := 54
	innerW := boxWidth - 2

	var lines []string
	lines = append(lines, msgStyle.Render(" "+m.message))
	if m.detail != "" {
		lines = append(lines, "")
		lines = append(lines, detailStyle.Render(" "+m.detail))
	}
	body := strings.Join(lines, "\n")

	title := "Confirm"
	dashesAfter := innerW - 1 - len(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	bodyLines := append([]string{""}, strings.Split(body, "\n")...)
	bodyLines = append(bodyLines, "")
	for _, line := range bodyLines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(leftBorder + line + pad + rightBorder + "\n")
	}

	hint := " Enter/y: confirm  Esc/n: cancel "
	bottomDashes := innerW - len(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}
