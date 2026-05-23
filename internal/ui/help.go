package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	// Popup spans the full terminal width so its borders align with the
	// main view's outer panel borders (Panel 1's left edge, Panel 2/3's
	// right edge).
	innerW := m.width - 2 // minus the two vertical borders
	if innerW < 60 {
		innerW = 60
	}
	// Split innerW into left col + gutter + right col. Odd-width terminals
	// leave a remainder after integer division; absorb it into the gutter
	// so the popup hits the right border exactly (otherwise a 1-col gap
	// shows up on the right and the popup no longer aligns with panel 2's
	// right border).
	leftColW := (innerW - 4) / 2
	rightColW := leftColW
	gutterW := innerW - leftColW - rightColW
	if leftColW < 30 {
		leftColW = 30
		rightColW = 30
		gutterW = innerW - leftColW - rightColW
		if gutterW < 2 {
			gutterW = 2
		}
	}
	colW := leftColW // tests + section sizing use the smaller side

	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	groups := m.groupedContent()
	descW := colW - 2 /*indent*/ - 14 /*key*/ - 1 /*space*/
	if descW < 8 {
		descW = 8
	}
	leftGroups, rightGroups := splitGroupsForColumns(groups, descW)

	// Render each section to its own line slice first so we know the natural
	// height of every column. Padding to a common target height is then
	// distributed as gaps BETWEEN sections (not piled at the bottom) so both
	// columns end at the same row AND section breaks feel naturally spaced.
	leftSections := m.renderSections(leftGroups, colW, descW)
	rightSections := m.renderSections(rightGroups, colW, descW)
	leftLines := joinColumnSections(leftSections, columnTarget(leftSections, rightSections))
	rightLines := joinColumnSections(rightSections, columnTarget(leftSections, rightSections))

	gutter := strings.Repeat(" ", gutterW)
	var bodyLines []string
	bodyLines = append(bodyLines, "") // top breathing
	for i := range leftLines {
		bodyLines = append(bodyLines, padRight(leftLines[i], leftColW)+gutter+padRight(rightLines[i], rightColW))
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
		// Safety net: clamp to innerW so a single overlong row never punches
		// through the right border. wrapPlain breaks at word boundaries and
		// will leave words longer than width oversized — truncate here.
		if lw > innerW {
			line = ansi.Truncate(line, innerW, "")
			lw = lipgloss.Width(line)
		}
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

// splitGroupsForColumns chooses the split point that minimises the height
// difference between the two columns. Counts wrap-continuation lines (long
// descriptions wrapped to multiple rows) because they contribute to actual
// column height even though there's only "1 entry".
//
// Greedy + look-ahead: for each group, place on the side that brings the
// (currently-running) totals closer to balance. Preserves group order so
// sections still flow naturally top-to-bottom.
func splitGroupsForColumns(groups []helpGroup, descW int) (left, right []helpGroup) {
	sizes := make([]int, len(groups))
	total := 0
	for i, g := range groups {
		sizes[i] = groupHeight(g, descW)
		total += sizes[i]
	}
	leftSum := 0
	for i, g := range groups {
		// Diff if we put g on the left vs leave the running totals as-is.
		addLeft := abs(leftSum + sizes[i] - (total - leftSum - sizes[i]))
		stay := abs(leftSum - (total - leftSum))
		if addLeft < stay && len(left) <= len(right)+1 {
			left = append(left, g)
			leftSum += sizes[i]
		} else if len(right) == 0 || stay <= addLeft {
			right = append(right, g)
		} else {
			left = append(left, g)
			leftSum += sizes[i]
		}
	}
	if len(left) == 0 && len(right) > 0 {
		// Edge case: everything ended up on the right (e.g. first group huge).
		// Move first back to left so the layout is never empty-left.
		left = append(left, right[0])
		right = right[1:]
	}
	return left, right
}

// groupHeight counts visible rows for a section: 1 title + per-entry wrap
// line count (1 if desc fits in descW, more if it wraps).
func groupHeight(g helpGroup, descW int) int {
	h := 0
	if g.title != "" {
		h++
	}
	for _, e := range g.entries {
		w := wrapPlain(e.desc, descW)
		if len(w) == 0 {
			h++
		} else {
			h += len(w)
		}
	}
	return h
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// renderSections returns one rendered line slice per group (no gaps yet —
// joinColumnSections inserts gaps later, sized so both columns end at the
// same row).
func (m HelpModel) renderSections(groups []helpGroup, colW, descW int) [][]string {
	sectionStyle := lipgloss.NewStyle().Bold(true)
	keyStyle := m.theme.DetailLabelStyle()
	descStyle := m.theme.DetailValueStyle()

	const keyW = 14
	const indent = "  "
	contIndent := indent + strings.Repeat(" ", keyW+1)

	out := make([][]string, len(groups))
	for i, g := range groups {
		var lines []string
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
		out[i] = lines
	}
	return out
}

// columnTarget is the row count both columns should match. Pick the taller
// natural height — the shorter column gets its inter-section gaps inflated
// to fill the difference.
func columnTarget(left, right [][]string) int {
	sum := func(s [][]string) int {
		t := 0
		for _, b := range s {
			t += len(b)
		}
		return t + max(0, len(s)-1) // 1 minimum gap between sections
	}
	l, r := sum(left), sum(right)
	if l > r {
		return l
	}
	return r
}

// joinColumnSections concatenates sections with blank-line gaps, sized so
// the column ends up exactly `target` rows tall. Extra padding is
// distributed evenly across the inter-section gaps (instead of dumped at
// the bottom), so section headers stay vertically balanced across columns.
func joinColumnSections(sections [][]string, target int) []string {
	contentRows := 0
	for _, s := range sections {
		contentRows += len(s)
	}
	gapCount := len(sections) - 1
	if gapCount < 0 {
		gapCount = 0
	}
	extraRows := target - contentRows
	if extraRows < gapCount {
		extraRows = gapCount // at least 1 blank between every two sections
	}

	gapSize := 1
	remainder := 0
	if gapCount > 0 {
		gapSize = extraRows / gapCount
		if gapSize < 1 {
			gapSize = 1
		}
		remainder = extraRows - gapSize*gapCount
	}

	var out []string
	for i, s := range sections {
		out = append(out, s...)
		if i < len(sections)-1 {
			gap := gapSize
			if i < remainder {
				gap++
			}
			for j := 0; j < gap; j++ {
				out = append(out, "")
			}
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
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
		{key: "Y", desc: "YAML popup (y:copy e:edit /:search)"},
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
