package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

// ContextPickerModel is an overlay that lets the user switch kubeconfig contexts.
type ContextPickerModel struct {
	contexts []string
	current  string
	cursor   int
	active   bool
	theme    *theme.Theme
}

// NewContextPickerModel creates a new context picker.
func NewContextPickerModel(t *theme.Theme) ContextPickerModel {
	return ContextPickerModel{
		theme: t,
	}
}

// Open populates the picker with available contexts and sets the cursor to
// the currently active context.
func (m *ContextPickerModel) Open(contexts []string, current string) {
	m.contexts = contexts
	m.current = current
	m.cursor = 0
	for i, c := range contexts {
		if c == current {
			m.cursor = i
			break
		}
	}
	m.active = true
}

// Close hides the picker.
func (m *ContextPickerModel) Close() {
	m.active = false
}

// IsActive returns whether the picker overlay is shown.
func (m *ContextPickerModel) IsActive() bool {
	return m.active
}

// Update handles keyboard input for the context picker.
func (m ContextPickerModel) Update(msg tea.Msg) (ContextPickerModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.contexts)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			selected := m.contexts[m.cursor]
			m.active = false
			return m, func() tea.Msg {
				return ContextChangedMsg{Context: selected}
			}
		case "esc", "c":
			m.active = false
			return m, nil
		}
	}

	return m, nil
}

// View renders the context picker (no-op; rendering via RenderPopup + overlay).
func (m ContextPickerModel) View() string {
	return ""
}

// RenderPopup returns the context picker box for use with overlay.Composite.
func (m ContextPickerModel) RenderPopup() string {
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	selectedStyle := m.theme.SidebarSelectedStyle()
	normalStyle := m.theme.SidebarStyle()

	boxWidth := 54
	innerW := boxWidth - 2

	maxVisible := 10
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.contexts) {
		end = len(m.contexts)
	}

	var lines []string
	for i := start; i < end; i++ {
		marker := "  "
		if m.contexts[i] == m.current {
			marker = "* "
		}
		label := marker + m.contexts[i]
		if i == m.cursor {
			lines = append(lines, selectedStyle.Width(innerW).Render(label))
		} else {
			lines = append(lines, normalStyle.Width(innerW).Render(label))
		}
	}
	body := strings.Join(lines, "\n")

	title := "Select Context"
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
