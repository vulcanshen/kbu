package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

// HelpModel is the Bubble Tea model for the help overlay.
type HelpModel struct {
	animator     PopupAnimator
	width        int
	height       int
	theme        *theme.Theme
	scrollOffset int
}

// NewHelpModel creates a new help model.
func NewHelpModel(t *theme.Theme) HelpModel {
	return HelpModel{
		theme:    t,
		animator: NewPopupAnimator("help", lipgloss.Color("#74c7ec")),
	}
}

// IsActive returns whether the help overlay is visible (including animations).
func (m HelpModel) IsActive() bool {
	return m.animator.IsActive()
}

// IsInteractive returns whether the help overlay should accept input.
func (m HelpModel) IsInteractive() bool {
	return m.animator.IsInteractive()
}

// Toggle switches the help overlay on or off, returning the animation tick cmd.
func (m *HelpModel) Toggle() tea.Cmd {
	if m.animator.IsActive() {
		return m.animator.Close()
	}
	m.scrollOffset = 0
	return m.animator.Open()
}

// HandleTick processes an animation tick. Returns a new tick cmd if animation continues.
func (m *HelpModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// SetSize sets the overlay dimensions.
func (m *HelpModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles key events for the help overlay.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			return m, m.animator.Close()
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
	if !m.animator.IsActive() {
		return ""
	}
	popup := m.RenderPopup()
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		popup)
}

// RenderPopup returns the help box (animated based on current animator state).
func (m HelpModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

// renderFullPopup builds the complete popup without animation.
func (m HelpModel) renderFullPopup() string {
	content := m.helpContent()

	boxWidth := 52
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	sectionStyle := lipgloss.NewStyle().Bold(true)
	keyStyle := m.theme.DetailLabelStyle()
	descStyle := m.theme.DetailValueStyle()

	var lines []string
	for _, entry := range content {
		if entry.isSection {
			lines = append(lines, sectionStyle.Render(" "+entry.text))
		} else if entry.key == "" {
			continue
		} else {
			key := keyStyle.Width(14).Render(entry.key)
			desc := descStyle.Render(entry.desc)
			lines = append(lines, "  "+key+desc)
		}
	}
	body := strings.Join(lines, "\n")
	panelH := len(lines) + 4

	title := " Keybindings"
	innerW := boxWidth - 2
	dashesAfter := innerW - 1 - len(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─"))
	b.WriteString(tStyle.Render(title))
	b.WriteString(bStyle.Render(strings.Repeat("─", dashesAfter) + "╮"))
	b.WriteString("\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	bodyLines := append([]string{""}, strings.Split(body, "\n")...)
	bodyLines = append(bodyLines, "")
	for len(bodyLines) < panelH-2 {
		bodyLines = append(bodyLines, "")
	}
	for _, line := range bodyLines[:panelH-2] {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		if line == "" {
			b.WriteString(leftBorder + strings.Repeat(" ", innerW) + rightBorder)
		} else {
			b.WriteString(leftBorder + line + pad + rightBorder)
		}
		b.WriteString("\n")
	}
	hint := " Esc/?:close j/k:scroll "
	bottomDashes := innerW - len(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
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
		{key: "= / -", desc: "Expand / restore"},
		{isSection: true, text: "Global"},
		{key: "n / N", desc: "Switch namespace"},
		{key: "c / C", desc: "Switch context"},
		{key: "T", desc: "Show / reattach KM8erm"},
		{key: "y", desc: "Copy focused panel"},
		{key: "Y", desc: "YAML popup (e:edit /:search)"},
		{key: "!", desc: "App log"},
		{key: "?", desc: "Toggle help"},
		{key: "q / Esc", desc: "Quit / back"},
		{isSection: true, text: "PTY popup (KM8erm/edit/shell)"},
		{key: "Alt+T", desc: "Hide KM8erm (shell stays alive)"},
		{key: "PgUp / PgDn", desc: "Scroll history page"},
		{key: "Home / End", desc: "Top / back to live"},
		{key: "(typing)", desc: "Snap back to live"},
		{key: "vim / less", desc: "Keys forward; no scrollback"},
	}
}
