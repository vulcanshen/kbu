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

// renderFullPopup builds the complete popup. Two-column layout — sections
// are packed left-to-right to keep total height short on modern keybinding
// lists (~30 entries) without making the popup tall enough to need scroll.
// Long descriptions wrap onto continuation lines indented under the desc
// column so the popup fits a standard 80-col terminal.
func (m HelpModel) renderFullPopup() string {
	const colW = 38
	const gutterW = 2
	innerW := colW*2 + gutterW

	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	groups := m.groupedContent()
	leftGroups, rightGroups := splitGroupsForColumns(groups)
	leftLines := m.renderColumn(leftGroups, colW)
	rightLines := m.renderColumn(rightGroups, colW)
	for len(leftLines) < len(rightLines) {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < len(leftLines) {
		rightLines = append(rightLines, "")
	}

	gutter := strings.Repeat(" ", gutterW)
	var bodyLines []string
	bodyLines = append(bodyLines, "") // top breathing
	for i := range leftLines {
		bodyLines = append(bodyLines, padRight(leftLines[i], colW)+gutter+padRight(rightLines[i], colW))
	}
	bodyLines = append(bodyLines, "") // bottom breathing

	title := " Keybindings"
	dashesAfter := innerW - 1 - lipgloss.Width(title)
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
	for _, line := range bodyLines {
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
	bottomDashes := innerW - lipgloss.Width(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}

// helpGroup is one section + its entries — the atomic unit balanced across
// the two columns. Sections never split across columns; the gutter alone is
// already a strong-enough visual separator without splitting a coherent
// section across it.
type helpGroup struct {
	title   string
	entries []helpEntry
}

func (m HelpModel) groupedContent() []helpGroup {
	var groups []helpGroup
	var cur helpGroup
	flush := func() {
		if cur.title != "" || len(cur.entries) > 0 {
			groups = append(groups, cur)
		}
	}
	for _, e := range m.helpContent() {
		if e.isSection {
			flush()
			cur = helpGroup{title: e.text}
			continue
		}
		if e.key == "" {
			continue
		}
		cur.entries = append(cur.entries, e)
	}
	flush()
	return groups
}

// splitGroupsForColumns greedily packs groups into the left column until the
// running line-count exceeds half of the total, then dumps the rest into the
// right column. Keeps each section intact.
func splitGroupsForColumns(groups []helpGroup) (left, right []helpGroup) {
	total := 0
	for _, g := range groups {
		total += 1 + len(g.entries) // section header + entries
	}
	target := total / 2
	running := 0
	for _, g := range groups {
		if running >= target && len(left) > 0 {
			right = append(right, g)
			continue
		}
		left = append(left, g)
		running += 1 + len(g.entries)
	}
	return left, right
}

func (m HelpModel) renderColumn(groups []helpGroup, colW int) []string {
	sectionStyle := lipgloss.NewStyle().Bold(true)
	keyStyle := m.theme.DetailLabelStyle()
	descStyle := m.theme.DetailValueStyle()

	// Layout: 2-space indent + 14-col key + 1 space + desc, so descriptions
	// can use the rest. Long descs wrap onto continuation lines that align
	// under the desc column.
	const keyW = 14
	const indent = "  "
	descW := colW - len(indent) - keyW - 1
	if descW < 8 {
		descW = 8
	}
	contIndent := indent + strings.Repeat(" ", keyW+1)

	var lines []string
	for i, g := range groups {
		if i > 0 {
			lines = append(lines, "") // single blank between sections
		}
		if g.title != "" {
			lines = append(lines, sectionStyle.Render(" "+g.title))
		}
		for _, e := range g.entries {
			wrapped := wrapPlain(e.desc, descW)
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}
			key := keyStyle.Width(keyW).Render(e.key)
			lines = append(lines, indent+key+" "+descStyle.Render(wrapped[0]))
			for _, w := range wrapped[1:] {
				lines = append(lines, contIndent+descStyle.Render(w))
			}
		}
	}
	return lines
}

// padRight extends a styled string with trailing spaces so its visual width
// equals width. ANSI escapes are ignored via lipgloss.Width.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
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
		{key: "Alt+T", desc: "Toggle KM8erm (spawn/show/hide)"},
		{key: "y", desc: "Copy focused panel"},
		{key: "Y", desc: "YAML popup (e:edit /:search)"},
		{key: "!", desc: "App log"},
		{key: "?", desc: "Toggle help"},
		{key: "q / Esc", desc: "Quit / back"},
		{isSection: true, text: "PTY popup (KM8erm/edit/shell)"},
		{key: "PgUp / PgDn", desc: "Scroll history page"},
		{key: "Home / End", desc: "Top / back to live"},
		{key: "(typing)", desc: "Snap back to live"},
		{key: "vim / less", desc: "Keys forward; no scrollback"},
	}
}
