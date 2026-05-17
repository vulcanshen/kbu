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
	animator   PopupAnimator
	theme      *theme.Theme
}

func NewNamespacePickerModel(t *theme.Theme) NamespacePickerModel {
	return NamespacePickerModel{
		theme:    t,
		animator: NewPopupAnimator("namespace", lipgloss.Color("#74c7ec")),
	}
}

func (m *NamespacePickerModel) Open(namespaces []string) tea.Cmd {
	all := []string{"All Namespaces"}
	m.namespaces = append(all, namespaces...)
	m.cursor = 0
	return m.animator.Open()
}

func (m *NamespacePickerModel) Close() tea.Cmd {
	return m.animator.Close()
}

func (m *NamespacePickerModel) IsActive() bool      { return m.animator.IsActive() }
func (m *NamespacePickerModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *NamespacePickerModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m NamespacePickerModel) Update(msg tea.Msg) (NamespacePickerModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
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
			closeCmd := m.animator.Close()
			return m, tea.Batch(closeCmd, func() tea.Msg {
				return NamespaceChangedMsg{Namespace: ns}
			})
		case "esc", "n":
			return m, m.animator.Close()
		}
	}

	return m, nil
}

func (m NamespacePickerModel) View() string {
	return ""
}

func (m NamespacePickerModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m NamespacePickerModel) renderFullPopup() string {
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	selectedStyle := m.theme.SidebarSelectedStyle()
	normalStyle := m.theme.SidebarStyle()

	boxWidth := 44
	innerW := boxWidth - 2

	maxVisible := 10
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.namespaces) {
		end = len(m.namespaces)
	}

	var lines []string
	for i := start; i < end; i++ {
		label := " " + m.namespaces[i]
		if i == m.cursor {
			lines = append(lines, selectedStyle.Width(innerW).Render(label))
		} else {
			lines = append(lines, normalStyle.Width(innerW).Render(label))
		}
	}
	body := strings.Join(lines, "\n")

	title := "Select Namespace"
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

	hint := " Enter: select  Esc: cancel "
	bottomDashes := innerW - len(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}
