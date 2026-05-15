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

// View renders the help overlay as a centered box.
func (m HelpModel) View() string {
	if !m.active {
		return ""
	}

	titleStyle := m.theme.DetailTabActiveStyle()
	sectionStyle := m.theme.SidebarCategoryStyle()
	keyStyle := m.theme.DetailLabelStyle()
	descStyle := m.theme.DetailValueStyle()
	hintStyle := m.theme.StatusLineStyle()

	content := m.helpContent()

	boxWidth := 50
	if boxWidth > m.width-6 {
		boxWidth = m.width - 6
	}

	var lines []string
	lines = append(lines, titleStyle.Width(boxWidth).Align(lipgloss.Center).Render("Keybindings"))
	lines = append(lines, "")

	// Apply scroll
	visibleHeight := m.contentHeight()
	start := m.scrollOffset
	end := start + visibleHeight
	if end > len(content) {
		end = len(content)
	}
	if start > len(content) {
		start = len(content)
	}

	for _, entry := range content[start:end] {
		if entry.isSection {
			lines = append(lines, sectionStyle.Render(entry.text))
		} else {
			key := keyStyle.Width(16).Render(entry.key)
			desc := descStyle.Render(entry.desc)
			lines = append(lines, "  "+key+desc)
		}
	}

	lines = append(lines, "")
	lines = append(lines, hintStyle.Render(" Esc/q/?: close  j/k: scroll"))

	body := strings.Join(lines, "\n")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Detail.BorderColor)).
		Padding(1, 2).
		Render(body)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay)
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
		{key: "j / k", desc: "Move cursor up/down"},
		{key: "gg / G", desc: "Jump to top/bottom"},
		{key: "1 / 2 / 3", desc: "Switch to panel"},
		{key: "Tab", desc: "Cycle panels"},
		{},
		{isSection: true, text: "Table"},
		{key: "/", desc: "Search/filter"},
		{key: "Esc", desc: "Clear filter"},
		{key: "Enter", desc: "Drill down / confirm filter"},
		{key: "e", desc: "Edit resource (kubectl edit)"},
		{key: "d", desc: "Delete resource"},
		{key: "s", desc: "Shell into container"},
		{},
		{isSection: true, text: "Detail"},
		{key: "h / l", desc: "Switch tab"},
		{key: "+ / -", desc: "Expand / restore panel"},
		{},
		{isSection: true, text: "Global"},
		{key: "n", desc: "Switch namespace"},
		{key: "c", desc: "Switch context"},
		{key: "!", desc: "App log"},
		{key: "?", desc: "Toggle help"},
		{key: "q", desc: "Quit"},
	}
}
