package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/kbu/internal/theme"
)

// HelpModel is the key reference `?` opens (tdp K6, M4): the keys of the
// frontmost surface — the popup on top, the mode the user is in, or the
// focused panel. It is a note (F1): read-only and scrollable, no cursor,
// nothing to run. `?` again or Esc closes it.
type HelpModel struct {
	animator     PopupAnimator
	width        int
	height       int
	theme        *theme.Theme
	scrollOffset int
	layer        int
	borderColor  lipgloss.Color

	title string
	rows  []helpRow
}

// helpRow is one line of a key reference: a key and what it does, or a
// section header.
type helpRow struct {
	header bool
	key    string
	desc   string
}

// helpTitleGlyph marks the key reference's title (tdp D3: glyph + text).
const helpTitleGlyph = "󰘳"

// NewHelpModel creates a new help model.
func NewHelpModel(t *theme.Theme) HelpModel {
	bc := theme.PopupLayerColor(1)
	return HelpModel{
		theme:       t,
		animator:    NewPopupAnimator("help", bc),
		borderColor: bc,
		layer:       1,
	}
}

// SetLayer stamps nesting depth + derives border / animator color.
func (m *HelpModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	m.animator.Color = m.borderColor
}

// IsActive returns whether the help overlay is visible (including animations).
func (m HelpModel) IsActive() bool {
	return m.animator.IsActive()
}

// IsInteractive returns whether the help overlay should accept input.
func (m HelpModel) IsInteractive() bool {
	return m.animator.IsInteractive()
}

// Open shows the key reference with the given title and rows.
func (m *HelpModel) Open(title string, rows []helpRow) tea.Cmd {
	m.title = title
	m.rows = rows
	m.scrollOffset = 0
	return m.animator.Open()
}

// Close starts the close animation unconditionally — used by
// AppModel.closeAllBlockingPopups when a context-shift target
// (PTY / drill-down) pre-empts the popup stack.
func (m *HelpModel) Close() tea.Cmd { return m.animator.Close() }

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

// Update scrolls the reference; `?` and Esc close it. Nothing else does
// anything: there is no cursor and nothing to run.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc", "?":
		return m, m.animator.Close()
	case "j", "down":
		m.scrollOffset = min(m.scrollOffset+1, m.maxScroll())
	case "k", "up":
		m.scrollOffset = max(m.scrollOffset-1, 0)
	case "d":
		m.scrollOffset = min(m.scrollOffset+m.bodyHeight()/2, m.maxScroll())
	case "u":
		m.scrollOffset = max(m.scrollOffset-m.bodyHeight()/2, 0)
	case "G":
		m.scrollOffset = m.maxScroll()
	case "g":
		m.scrollOffset = 0
	}
	return m, nil
}

// bodyHeight is how many rows fit: the reference is as tall as its
// content, capped by the screen minus the frame and a row of margin
// above and below (then it scrolls).
func (m HelpModel) bodyHeight() int {
	limit := m.height - 2*popupVMargin - 4 // borders + padding rows
	if limit < 3 {
		limit = 3
	}
	return min(len(m.rows), limit)
}

func (m HelpModel) maxScroll() int {
	return max(len(m.rows)-m.bodyHeight(), 0)
}

// HandleMouse: right-click inside the reference closes it (mirror of
// Esc). Left-click does nothing — the reference is read-only. Wheel
// scrolls via the AppModel-layer u/d synthesis.
func (m HelpModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (HelpModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if !popupContains(m.RenderPopup(), msg, screenW, screenH) {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m, m.animator.Close()
	}
	return m, nil
}

// RenderPopup returns the reference box (animated per the animator state).
func (m HelpModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m HelpModel) renderFullPopup() string {
	innerW := popupInnerWidth(m.width) // tdp F7
	bc := m.borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	headerStyle := lipgloss.NewStyle().Bold(true)
	keyStyle := m.theme.DetailLabelStyle()
	descStyle := m.theme.DetailValueStyle()

	keyW := 0
	for _, r := range m.rows {
		if !r.header {
			keyW = max(keyW, lipgloss.Width(r.key))
		}
	}
	keyW = min(keyW, 18)

	var lines []string
	end := min(m.scrollOffset+m.bodyHeight(), len(m.rows))
	for _, r := range m.rows[m.scrollOffset:end] {
		if r.header {
			lines = append(lines, " "+headerStyle.Render(r.desc))
			continue
		}
		lines = append(lines, "   "+keyStyle.Render(padRight(r.key, keyW))+"  "+descStyle.Render(r.desc))
	}

	title := " " + helpTitleGlyph + " " + m.title + " "
	dashesAfter := max(innerW-1-lipgloss.Width(title), 0)
	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")
	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right + "\n"
	b.WriteString(padRow)
	for _, line := range lines {
		if lipgloss.Width(line) > innerW {
			line = ansi.Truncate(line, innerW, "")
		}
		b.WriteString(left + padRight(line, innerW) + right + "\n")
	}
	b.WriteString(padRow)
	hint := " j/k: scroll  ?/Esc: close "
	bottomDashes := max(innerW-lipgloss.Width(hint)-1, 0)
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))
	return b.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
