package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// YamlPopupModel renders the full YAML of a resource in a large overlay popup.
// Supports j/k/u/d/gg/G scroll, / search (Enter commits; n/N step through
// matches), e to dispatch kubectl edit on the same resource (no confirm — user
// already inspected before pressing), and Esc/q to close.
type YamlPopupModel struct {
	yaml string
	// rawLines is the source YAML split by newline. searchQuery matches
	// against these; matchLines indices refer to this slice.
	rawLines []string
	// contentLines are the rendered DISPLAY lines — long raw lines are word-
	// wrapped to fit the popup body width, then highlighted. One raw line
	// may produce 1+ display lines (see contentLineRaw).
	contentLines []string
	// contentLineRaw[i] = index into rawLines that display line i came from.
	// Used to (a) map matchLines (raw indices) to display positions for
	// auto-scroll and (b) extend the current-match background highlight
	// across all wrapped chunks of the same raw line.
	contentLineRaw []int
	// lastBuiltWidth caches the body width content was wrapped for, so we
	// only rebuild on actual width changes (not every render call).
	lastBuiltWidth int
	scrollOffset   int
	width          int
	height         int
	theme          *theme.Theme
	animator       PopupAnimator

	// Edit target captured at Open() time.
	resource    k8s.ResourceType
	item        k8s.ResourceItem
	contextName string

	// Search state.
	searching   bool
	searchQuery string
	matchLines  []int
	matchCursor int

	pendingG bool
}

// NewYamlPopupModel constructs a YamlPopupModel.
func NewYamlPopupModel(t *theme.Theme) YamlPopupModel {
	return YamlPopupModel{
		theme:    t,
		animator: NewPopupAnimator("yamlpopup", lipgloss.Color("#74c7ec")),
	}
}

// Open populates the popup with YAML for a specific resource, captures the
// edit target, and begins the open animation.
func (m *YamlPopupModel) Open(yaml string, rt k8s.ResourceType, item k8s.ResourceItem, ctxName string) tea.Cmd {
	m.yaml = strings.TrimRight(yaml, "\n")
	m.resource = rt
	m.item = item
	m.contextName = ctxName
	m.scrollOffset = 0
	m.searching = false
	m.searchQuery = ""
	m.matchLines = nil
	m.matchCursor = 0
	m.pendingG = false
	m.rebuildContent()
	return m.animator.Open()
}

// Close begins the close animation.
func (m *YamlPopupModel) Close() tea.Cmd { return m.animator.Close() }

// IsActive reports whether the popup is being drawn (including animations).
func (m YamlPopupModel) IsActive() bool { return m.animator.IsActive() }

// IsInteractive reports whether the popup should accept input.
func (m YamlPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

// HandleTick routes animation tick messages to the popup animator.
func (m *YamlPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// SetSize records the screen dimensions used to size the popup and rebuilds
// wrapped content when the body width changes (so reflow happens on resize).
func (m *YamlPopupModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if m.yaml != "" && m.bodyWidth() != m.lastBuiltWidth {
		m.rebuildContent()
	}
}

// bodyWidth is the column budget for a single YAML line inside the popup
// (popup inner width minus the 2 borders).
func (m YamlPopupModel) bodyWidth() int {
	w := m.popupWidth() - 2
	if w < 10 {
		w = 10
	}
	return w
}

func (m *YamlPopupModel) rebuildContent() {
	if m.yaml == "" {
		m.rawLines = nil
		m.contentLines = nil
		m.contentLineRaw = nil
		m.lastBuiltWidth = 0
		return
	}
	m.rawLines = strings.Split(m.yaml, "\n")
	w := m.bodyWidth()
	m.lastBuiltWidth = w

	m.contentLines = m.contentLines[:0]
	m.contentLineRaw = m.contentLineRaw[:0]
	for i, raw := range m.rawLines {
		chunks := wrapPlain(raw, w)
		if len(chunks) == 0 {
			// Empty raw line still takes one display row so blank lines in
			// the YAML render as blank rows (don't collapse).
			m.contentLines = append(m.contentLines, "")
			m.contentLineRaw = append(m.contentLineRaw, i)
			continue
		}
		for _, chunk := range chunks {
			m.contentLines = append(m.contentLines, highlightYAMLLine(chunk, m.theme))
			m.contentLineRaw = append(m.contentLineRaw, i)
		}
	}
}

// firstDisplayLineForRaw returns the smallest display-line index whose source
// raw line == rawIdx, or -1 if not found. Used by search auto-scroll: after
// the user commits a query, we jump the viewport to the first wrapped chunk
// of the matched raw line.
func (m YamlPopupModel) firstDisplayLineForRaw(rawIdx int) int {
	for i, r := range m.contentLineRaw {
		if r == rawIdx {
			return i
		}
	}
	return -1
}

// Update handles keyboard input.
func (m YamlPopupModel) Update(msg tea.Msg) (YamlPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.searching {
		return m.handleSearchKey(keyMsg)
	}

	switch keyMsg.String() {
	case "esc", "q", " ":
		m.pendingG = false
		return m, m.animator.Close()
	case "j", "down":
		if m.scrollOffset < m.maxScrollOffset() {
			m.scrollOffset++
		}
		m.pendingG = false
	case "k", "up":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
		m.pendingG = false
	case "d":
		half := m.contentHeight() / 2
		if half < 1 {
			half = 1
		}
		m.scrollOffset += half
		if m.scrollOffset > m.maxScrollOffset() {
			m.scrollOffset = m.maxScrollOffset()
		}
		m.pendingG = false
	case "u":
		half := m.contentHeight() / 2
		if half < 1 {
			half = 1
		}
		m.scrollOffset -= half
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		m.pendingG = false
	case "G":
		m.scrollOffset = m.maxScrollOffset()
		m.pendingG = false
	case "g":
		if m.pendingG {
			m.scrollOffset = 0
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case "/":
		m.searching = true
		m.searchQuery = ""
		m.matchLines = nil
		m.matchCursor = 0
		m.pendingG = false
	case "n":
		if len(m.matchLines) > 0 {
			m.matchCursor = (m.matchCursor + 1) % len(m.matchLines)
			m.scrollToMatch()
		}
		m.pendingG = false
	case "N":
		if len(m.matchLines) > 0 {
			m.matchCursor = (m.matchCursor - 1 + len(m.matchLines)) % len(m.matchLines)
			m.scrollToMatch()
		}
		m.pendingG = false
	case "E":
		// Direct edit dispatch. User already inspected the YAML in this popup
		// so we skip the confirm step that the table-level `E` uses.
		if m.item.Name == "" {
			return m, nil
		}
		closeCmd := m.animator.Close()
		rt, item, ctx := m.resource, m.item, m.contextName
		return m, tea.Batch(closeCmd, func() tea.Msg {
			return startEditMsg{resource: rt, item: item, contextName: ctx}
		})
	case "y":
		// Copy the full YAML (not just the visible viewport) — paste-back
		// use cases want the whole document. OSC 52 via copyToClipboardCmd
		// works through tmux / SSH without xclip / pbcopy.
		if m.yaml == "" {
			return m, nil
		}
		return m, copyToClipboardCmd(m.yaml)
	}
	return m, nil
}

func (m YamlPopupModel) handleSearchKey(msg tea.KeyMsg) (YamlPopupModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.searching = false
		m.searchQuery = ""
		m.matchLines = nil
		m.matchCursor = 0
		return m, nil
	case msg.Type == tea.KeyEnter:
		m.searching = false
		m.computeMatches()
		m.scrollToMatch()
		return m, nil
	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
		return m, nil
	case msg.Type == tea.KeyRunes:
		for _, r := range msg.Runes {
			m.searchQuery += string(r)
		}
		return m, nil
	}
	return m, nil
}

func (m *YamlPopupModel) computeMatches() {
	m.matchLines = nil
	m.matchCursor = 0
	if m.searchQuery == "" {
		return
	}
	q := strings.ToLower(m.searchQuery)
	for i, l := range m.rawLines {
		if strings.Contains(strings.ToLower(l), q) {
			m.matchLines = append(m.matchLines, i)
		}
	}
}

func (m *YamlPopupModel) scrollToMatch() {
	if len(m.matchLines) == 0 {
		return
	}
	rawTarget := m.matchLines[m.matchCursor]
	target := m.firstDisplayLineForRaw(rawTarget)
	if target < 0 {
		return
	}
	contentH := m.contentHeight()
	if target < m.scrollOffset || target >= m.scrollOffset+contentH {
		m.scrollOffset = target - contentH/2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		if m.scrollOffset > m.maxScrollOffset() {
			m.scrollOffset = m.maxScrollOffset()
		}
	}
}

// Popup sizing constants — absolute cells, no percentage math. Both popups
// (YAML + Help) leave one row/col of breathing room between their outer
// border and the terminal edge. overlay.Composite centers the popup, so a
// popup outer width of (m.width - 2*popupHMargin) ends up with exactly
// popupHMargin cells of empty space on each side.
const (
	popupHMargin = 1
	popupVMargin = 1
)

func (m YamlPopupModel) popupWidth() int {
	if m.width <= 0 {
		return 60
	}
	w := m.width - 2*popupHMargin
	if w < 40 {
		w = 40
	}
	return w
}

func (m YamlPopupModel) popupHeight() int {
	if m.height <= 0 {
		return 20
	}
	h := m.height - 2*popupVMargin
	if h < 10 {
		h = 10
	}
	return h
}

// contentHeight is how many YAML lines fit in the body, accounting for the
// optional search box at the top. Just borders (2) — no inner padding rows,
// per UX feedback the previous 1-row blank top + 1-row blank bottom was
// wasted space.
func (m YamlPopupModel) contentHeight() int {
	h := m.popupHeight() - 2 // top + bottom border
	if m.searching || m.searchQuery != "" {
		h -= 3 // search box is 3 lines
	}
	if h < 1 {
		h = 1
	}
	return h
}

// MatchCount returns the number of search matches (for tests + indicator).
func (m YamlPopupModel) MatchCount() int { return len(m.matchLines) }

// MatchCursor returns the current match index (for tests).
func (m YamlPopupModel) MatchCursor() int { return m.matchCursor }

// ScrollOffset returns the current scroll position (for tests).
func (m YamlPopupModel) ScrollOffset() int { return m.scrollOffset }

// SearchQuery returns the current search query (for tests).
func (m YamlPopupModel) SearchQuery() string { return m.searchQuery }

// IsSearching reports whether the popup is in search-input mode (for tests).
func (m YamlPopupModel) IsSearching() bool { return m.searching }

func (m YamlPopupModel) maxScrollOffset() int {
	n := len(m.contentLines) - m.contentHeight()
	if n < 0 {
		return 0
	}
	return n
}

// HandleMouse routes a click against the YAML viewer. Wheel scroll
// is already translated to u/d at the AppModel layer; this method
// only handles discrete buttons. Right-click inside the popup
// closes it (mirror of Esc / q). Left-click is no-op — YAML has
// no row cursor.
func (m YamlPopupModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (YamlPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if !popupContains(m.renderFullPopup(), msg, screenW, screenH) {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m, m.animator.Close()
	}
	return m, nil
}

// View is a no-op; rendering happens via RenderPopup + overlay composition.
func (m YamlPopupModel) View() string { return "" }

// RenderPopup returns the popup frame (respecting animation state).
func (m YamlPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m YamlPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	// matchRowStyle highlights the line under the search cursor with the same
	// background treatment as panel-view selected rows, so users get a
	// consistent "this is what you're on" cue across the app.
	matchRowStyle := m.theme.TableSelectedRowStyle()

	boxW := m.popupWidth()
	panelH := m.popupHeight()
	innerW := boxW - 2
	contentH := m.contentHeight()

	title := "  YAML — " + m.resource.KubectlName() + "/" + m.item.Name
	if m.item.Namespace != "" {
		title += " (" + m.item.Namespace + ")"
	}
	if lipgloss.Width(title) > innerW-1 {
		title = ansiTruncate(title, innerW-1)
	}
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

	var lines []string
	if m.searching || m.searchQuery != "" {
		// renderSearchBox auto-picks amber when !active && query != "" (locked
		// filter), so the call site doesn't need to branch on the two states.
		lines = append(lines, strings.Split(renderSearchBox(m.searchQuery, m.searching, innerW, m.theme), "\n")...)
	}

	// Content slice.
	start := m.scrollOffset
	if start > len(m.contentLines) {
		start = len(m.contentLines)
	}
	end := start + contentH
	if end > len(m.contentLines) {
		end = len(m.contentLines)
	}

	// Current match is identified by its RAW line index; with wrapping a
	// single raw match may span multiple consecutive display lines, so we
	// highlight every display row whose source raw line is the match.
	currentMatchRaw := -1
	if !m.searching && len(m.matchLines) > 0 {
		currentMatchRaw = m.matchLines[m.matchCursor]
	}

	for i := start; i < end; i++ {
		isMatch := currentMatchRaw >= 0 && i < len(m.contentLineRaw) && m.contentLineRaw[i] == currentMatchRaw
		if isMatch {
			// Full-row highlight. Strip ANSI from the display chunk so lipgloss
			// Background composes cleanly (per-token foregrounds inside the
			// styled line would leak default bg through the row otherwise).
			plain := ansi.Strip(m.contentLines[i])
			if lipgloss.Width(plain) > innerW {
				plain = ansiTruncate(plain, innerW)
			}
			lines = append(lines, matchRowStyle.Width(innerW).Render(plain))
		} else {
			line := m.contentLines[i]
			if lipgloss.Width(line) > innerW {
				line = ansiTruncate(line, innerW)
			}
			lines = append(lines, line)
		}
	}
	if len(m.contentLines) == 0 {
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
		lines = append(lines, dim.Render("  (no YAML — resource may still be loading)"))
	}

	for len(lines) < panelH-2 {
		lines = append(lines, "")
	}
	lines = lines[:panelH-2]

	for _, l := range lines {
		lw := lipgloss.Width(l)
		if lw > innerW {
			l = ansiTruncate(l, innerW)
			lw = lipgloss.Width(l)
		}
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		if l == "" {
			b.WriteString(leftBorder + strings.Repeat(" ", innerW) + rightBorder)
		} else {
			b.WriteString(leftBorder + l + pad + rightBorder)
		}
		b.WriteString("\n")
	}

	hint, indicator := m.bottomBarStrings(contentH, innerW-1)
	bottomDashes := innerW - lipgloss.Width(hint) - lipgloss.Width(indicator) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─"))
	b.WriteString(tStyle.Render(hint))
	b.WriteString(bStyle.Render(strings.Repeat("─", bottomDashes) + indicator + "╯"))

	return b.String()
}

// bottomBarStrings produces the bottom-border hint + indicator pair that fits
// in `available` columns. Vim conventions (j/k u/d gg/G n/N) are intentionally
// omitted — they apply everywhere in km8 and don't need to live in the local
// hint. Falls back to a short hint and finally drops the indicator if the
// popup width is too tight.
func (m YamlPopupModel) bottomBarStrings(contentH, available int) (hint, indicator string) {
	const hintFull = " E:edit  y:copy  /:search  Space:close "
	const hintShort = " E  y  /  Space "
	hint = hintFull

	total := len(m.contentLines)
	if total > 0 {
		shownEnd := m.scrollOffset + contentH
		if shownEnd > total {
			shownEnd = total
		}
		indicator = fmt.Sprintf(" %d-%d/%d ", m.scrollOffset+1, shownEnd, total)
		if len(m.matchLines) > 0 && !m.searching {
			indicator = fmt.Sprintf(" %d/%d %s", m.matchCursor+1, len(m.matchLines), strings.TrimLeft(indicator, " "))
		}
	}

	fits := func() bool {
		return lipgloss.Width(hint)+lipgloss.Width(indicator) <= available
	}
	if fits() {
		return
	}
	hint = hintShort
	if fits() {
		return
	}
	// Drop the verbose part of the indicator (match counter) but keep range.
	if total > 0 {
		shownEnd := m.scrollOffset + contentH
		if shownEnd > total {
			shownEnd = total
		}
		indicator = fmt.Sprintf(" %d-%d/%d ", m.scrollOffset+1, shownEnd, total)
	}
	if fits() {
		return
	}
	indicator = ""
	return
}
