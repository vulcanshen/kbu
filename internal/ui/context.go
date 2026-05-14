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
	width    int
	height   int
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

// SetSize sets the overlay dimensions (full terminal size).
func (m *ContextPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
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

// View renders the context picker as a centered overlay.
func (m ContextPickerModel) View() string {
	if !m.active {
		return ""
	}

	titleStyle := m.theme.DetailTabActiveStyle()
	selectedStyle := m.theme.SidebarSelectedStyle()
	normalStyle := m.theme.SidebarStyle()

	boxWidth := 50
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}

	var lines []string
	lines = append(lines, titleStyle.Width(boxWidth).Align(lipgloss.Center).Render("Select Context"))
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
	if end > len(m.contexts) {
		end = len(m.contexts)
	}

	for i := start; i < end; i++ {
		label := m.contexts[i]
		marker := "  "
		if m.contexts[i] == m.current {
			marker = "* "
		}
		display := marker + label
		if i == m.cursor {
			lines = append(lines, selectedStyle.Width(boxWidth).Render(display))
		} else {
			lines = append(lines, normalStyle.Width(boxWidth).Render(display))
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
