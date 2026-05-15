package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

// HelpModel is the Bubble Tea model for the help overlay.
type HelpModel struct {
	active       bool
	width        int
	height       int
	theme        *theme.Theme
	scrollOffset int
}

// NewHelpModel creates a new help model.
func NewHelpModel(t *theme.Theme) HelpModel {
	return HelpModel{
		theme: t,
	}
}

// IsActive returns whether the help overlay is visible.
func (m HelpModel) IsActive() bool {
	return m.active
}

// Toggle switches the help overlay on or off.
func (m *HelpModel) Toggle() {
	m.active = !m.active
	if m.active {
		m.scrollOffset = 0
	}
}

// SetSize sets the overlay dimensions.
func (m *HelpModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles key events for the help overlay.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			m.active = false
			return m, nil
		case "j", "down":
			content := m.helpContent()
			maxOffset := len(content) - m.contentHeight()
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.scrollOffset < maxOffset {
				m.scrollOffset++
			}
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		}
	}

	return m, nil
}

// contentHeight returns how many lines of content can be shown.
func (m HelpModel) contentHeight() int {
	// Subtract space for border, padding, and hint line
	h := m.height - 8
	if h < 5 {
		h = 5
	}
	return h
}

// View renders the help overlay as a full-screen placement (legacy).
func (m HelpModel) View() string {
	if !m.active {
		return ""
	}
	popup := m.RenderPopup()
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		popup)
}

// RenderPopup returns just the bordered popup box (for overlay on background).
func (m HelpModel) RenderPopup() string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Sidebar.CategoryFg)).
		Bold(true).
		Background(lipgloss.Color("#1e1e2e"))
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Detail.LabelFg)).
		Background(lipgloss.Color("#1e1e2e"))
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Detail.ValueFg)).
		Background(lipgloss.Color("#1e1e2e"))
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.StatusLine.Foreground)).
		Background(lipgloss.Color("#1e1e2e"))
	bgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1e1e2e"))

	content := m.helpContent()

	boxWidth := 44
	if boxWidth > m.width-6 {
		boxWidth = m.width - 6
	}

	var lines []string
	lines = append(lines, sectionStyle.Width(boxWidth).Align(lipgloss.Center).Render("Keybindings"))

	for _, entry := range content {
		if entry.isSection {
			lines = append(lines, sectionStyle.Width(boxWidth).Render(" "+entry.text))
		} else if entry.key == "" {
			continue
		} else {
			key := keyStyle.Width(14).Render(entry.key)
			desc := descStyle.Render(entry.desc)
			lines = append(lines, bgStyle.Render(" ")+key+desc)
		}
	}

	lines = append(lines, hintStyle.Width(boxWidth).Render(" Esc/?:close  j/k:scroll"))

	body := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Detail.BorderColor)).
		Background(lipgloss.Color("#1e1e2e")).
		Padding(0, 1).
		Render(body)
}

type helpEntry struct {
	isSection bool
	text      string
	key       string
	desc      string
}

func (m HelpModel) helpContent() []helpEntry {
	return []helpEntry{
		{isSection: true, text: "Navigation"},
		{key: "j / k", desc: "Up / down"},
		{key: "u / d", desc: "Page up / down"},
		{key: "gg / G", desc: "Top / bottom"},
		{key: "1 / 2 / 3", desc: "Switch panel"},
		{key: "Tab", desc: "Cycle panels"},
		{isSection: true, text: "Table"},
		{key: "/", desc: "Search / filter"},
		{key: "Enter", desc: "Drill down"},
		{key: "e", desc: "Edit (kubectl edit)"},
		{key: "D", desc: "Delete resource"},
		{key: "s", desc: "Shell into container"},
		{isSection: true, text: "Detail"},
		{key: "h / l", desc: "Switch tab"},
		{key: "+ / -", desc: "Expand / restore"},
		{isSection: true, text: "Global"},
		{key: "n", desc: "Switch namespace"},
		{key: "c", desc: "Switch context"},
		{key: "!", desc: "App log"},
		{key: "?", desc: "Toggle help"},
		{key: "q / Esc", desc: "Quit / back"},
	}
}
