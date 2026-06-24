package ui

import (
	"fmt"
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"github.com/vulcanshen/km8/internal/theme"
)

// CompareLayout selects how a diff renders. Split shows old/new side-by-side
// (git diff --side-by-side); Unified shows a single column with -/+ markers
// (git diff default). Persisted in the user config so the choice survives
// a restart.
type CompareLayout int

const (
	CompareLayoutSplit CompareLayout = iota
	CompareLayoutUnified
)

func (l CompareLayout) String() string {
	if l == CompareLayoutUnified {
		return "unified"
	}
	return "split"
}

// CompareYamlPopupModel renders a YAML diff between a locked baseline and a
// chosen comparison target. Driven from the panel-2 Space menu's "Compare
// to this resource" action. Both YAMLs arrive pre-cleaned by
// k8s.MarshalItemYAMLForCompare (status block stripped, managedFields /
// resourceVersion / uid / generation / creationTimestamp wiped). Diff
// computation goes through go-udiff; rendering is a hand-rolled side-by-
// side or +/- unified view, coloured via lipgloss.
//
// Keybindings (popup-only):
//
//	j/k/down/up      scroll one line
//	u/d              scroll half-page
//	g g              jump to top
//	G                jump to bottom
//	Space            open the in-popup action menu
//	Esc / q          close the popup
//
// Action menu (in-popup, Space-triggered):
//
//	Toggle layout    flip split ↔ unified (persisted via callback)
//	Close            close the popup
//
// The user explicitly asked for compare actions to be discoverable via
// menu rather than hotkey — same rationale as the panel-2 Lock /
// Compare-to / Exit entries — so there's no direct `s`/`u`/`t` toggle.
type CompareYamlPopupModel struct {
	leftYAML   string
	rightYAML  string
	leftLabel  string // "kind/name" of the locked baseline
	rightLabel string // "kind/name" of the comparison target

	layout CompareLayout

	// Rendered display lines for the current layout. Rebuilt whenever
	// layout or popup width changes. scrollOffset indexes into this slice.
	contentLines []string
	scrollOffset int

	// menuOpen + menuCursor drive the in-popup Space menu. Two items
	// (toggle, close), no submenu nesting.
	menuOpen   bool
	menuCursor int

	width    int
	height   int
	theme    *theme.Theme
	animator PopupAnimator

	// lastBuiltWidth + lastBuiltLayout cache the conditions under which
	// contentLines was built so we only rebuild on actual changes
	// (popup resize, layout toggle).
	lastBuiltWidth  int
	lastBuiltLayout CompareLayout

	pendingG bool

	// onLayoutChange is invoked when the user toggles split ↔ unified.
	// AppModel uses it to persist the new layout into the config file
	// so the choice survives a restart. nil = no persistence.
	onLayoutChange func(CompareLayout)
}

// NewCompareYamlPopupModel constructs a compare popup with the default
// (Unified) layout. Unified survives narrow panels and reads like a
// standard `git diff` to anyone who's used one, so it makes the
// safer default than Split. Callers override via SetDefaultLayout
// before Open if the user's config carries a different preference.
func NewCompareYamlPopupModel(t *theme.Theme) CompareYamlPopupModel {
	return CompareYamlPopupModel{
		theme:    t,
		animator: NewPopupAnimator("comparepopup", lipgloss.Color("#9DDAEA")),
		layout:   CompareLayoutUnified,
	}
}

// SetDefaultLayout sets the initial layout used by subsequent Open calls.
// Hook for config-driven defaults — AppModel wires this from the loaded
// km8 config.yaml `compare.layout` value at startup.
func (m *CompareYamlPopupModel) SetDefaultLayout(l CompareLayout) {
	m.layout = l
}

// SetOnLayoutChange registers a callback invoked when the user toggles
// the layout via the in-popup menu. AppModel uses this to persist the
// choice into the user config file.
func (m *CompareYamlPopupModel) SetOnLayoutChange(fn func(CompareLayout)) {
	m.onLayoutChange = fn
}

// Open populates the popup with both YAML payloads + the per-instance
// labels rendered in the column / line headers, then begins the open
// animation. Both payloads should already be compare-cleaned (status
// block + per-instance noise stripped — see k8s.MarshalItemYAMLForCompare).
func (m *CompareYamlPopupModel) Open(left, right, leftLabel, rightLabel string) tea.Cmd {
	m.leftYAML = left
	m.rightYAML = right
	m.leftLabel = leftLabel
	m.rightLabel = rightLabel
	m.scrollOffset = 0
	m.menuOpen = false
	m.menuCursor = 0
	m.pendingG = false
	m.rebuildContent()
	return m.animator.Open()
}

func (m *CompareYamlPopupModel) Close() tea.Cmd       { return m.animator.Close() }
func (m CompareYamlPopupModel) IsActive() bool        { return m.animator.IsActive() }
func (m CompareYamlPopupModel) IsInteractive() bool   { return m.animator.IsInteractive() }
func (m CompareYamlPopupModel) ScrollOffset() int     { return m.scrollOffset }
func (m CompareYamlPopupModel) Layout() CompareLayout { return m.layout }
func (m CompareYamlPopupModel) MenuOpen() bool        { return m.menuOpen }

func (m *CompareYamlPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m *CompareYamlPopupModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if m.leftYAML == "" && m.rightYAML == "" {
		return
	}
	if m.bodyWidth() != m.lastBuiltWidth || m.layout != m.lastBuiltLayout {
		m.rebuildContent()
	}
}

func (m CompareYamlPopupModel) Update(msg tea.Msg) (CompareYamlPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.menuOpen {
		return m.handleMenuKey(keyMsg)
	}
	return m.handlePopupKey(keyMsg)
}

func (m CompareYamlPopupModel) handlePopupKey(keyMsg tea.KeyMsg) (CompareYamlPopupModel, tea.Cmd) {
	switch keyMsg.String() {
	case "esc", "q":
		m.pendingG = false
		return m, m.animator.Close()
	case " ":
		m.menuOpen = true
		m.menuCursor = 0
		m.pendingG = false
		return m, nil
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
	case "g":
		if m.pendingG {
			m.scrollOffset = 0
			m.pendingG = false
		} else {
			m.pendingG = true
		}
	case "G":
		m.scrollOffset = m.maxScrollOffset()
		m.pendingG = false
	default:
		m.pendingG = false
	}
	return m, nil
}

// menuItems builds the in-popup Space menu. Returned as labels because
// the menu is two items — a slice indexed by menuCursor is the simplest
// possible structure.
func (m CompareYamlPopupModel) menuItems() []string {
	other := CompareLayoutSplit
	if m.layout == CompareLayoutSplit {
		other = CompareLayoutUnified
	}
	return []string{
		fmt.Sprintf("Switch to %s view", other.String()),
		"Close",
	}
}

func (m CompareYamlPopupModel) handleMenuKey(keyMsg tea.KeyMsg) (CompareYamlPopupModel, tea.Cmd) {
	items := m.menuItems()
	switch keyMsg.String() {
	case "esc", "q", " ":
		// Space / Esc close the menu but leave the popup open — gives
		// the user an "oops" path back without restarting the compare.
		m.menuOpen = false
		return m, nil
	case "j", "down":
		if m.menuCursor < len(items)-1 {
			m.menuCursor++
		}
		return m, nil
	case "k", "up":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
		return m, nil
	case "enter":
		switch m.menuCursor {
		case 0:
			if m.layout == CompareLayoutSplit {
				m.layout = CompareLayoutUnified
			} else {
				m.layout = CompareLayoutSplit
			}
			if m.onLayoutChange != nil {
				m.onLayoutChange(m.layout)
			}
			m.scrollOffset = 0
			m.rebuildContent()
			m.menuOpen = false
			return m, nil
		case 1:
			m.menuOpen = false
			return m, m.animator.Close()
		}
	}
	return m, nil
}

// rebuildContent recomputes contentLines for the current layout +
// popup body width. Called whenever Open / SetSize-width-changed /
// layout toggle happens.
func (m *CompareYamlPopupModel) rebuildContent() {
	bodyW := m.bodyWidth()
	m.lastBuiltWidth = bodyW
	m.lastBuiltLayout = m.layout
	if m.layout == CompareLayoutUnified {
		m.contentLines = renderUnifiedDiff(m.leftYAML, m.rightYAML, m.leftLabel, m.rightLabel, bodyW, m.theme)
	} else {
		m.contentLines = renderSplitDiff(m.leftYAML, m.rightYAML, m.leftLabel, m.rightLabel, bodyW, m.theme)
	}
	if m.scrollOffset > m.maxScrollOffset() {
		m.scrollOffset = m.maxScrollOffset()
	}
}

func (m CompareYamlPopupModel) bodyWidth() int {
	w := m.popupWidth() - 2
	if w < 20 {
		w = 20
	}
	return w
}

func (m CompareYamlPopupModel) popupWidth() int {
	if m.width <= 0 {
		return 80
	}
	w := m.width - 2*popupHMargin
	if w < 40 {
		w = 40
	}
	return w
}

func (m CompareYamlPopupModel) popupHeight() int {
	if m.height <= 0 {
		return 20
	}
	h := m.height - 2*popupVMargin
	if h < 10 {
		h = 10
	}
	return h
}

func (m CompareYamlPopupModel) contentHeight() int {
	h := m.popupHeight() - 2 // top + bottom border
	if h < 1 {
		h = 1
	}
	return h
}

func (m CompareYamlPopupModel) maxScrollOffset() int {
	max := len(m.contentLines) - m.contentHeight()
	if max < 0 {
		return 0
	}
	return max
}

// renderUnifiedDiff returns display lines for unified-diff mode. Wraps
// go-udiff's Unified() output with lipgloss colouring: red on `-`,
// green on `+`, dimmed on `@@` hunk headers, default on context.
func renderUnifiedDiff(left, right, leftLabel, rightLabel string, width int, t *theme.Theme) []string {
	diff := udiff.Unified(leftLabel, rightLabel, left, right)
	if diff == "" {
		return []string{centerNoDiff(width, t)}
	}
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Status.Running))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Status.Error))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9DDAEA")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	var out []string
	for _, raw := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		// Truncate to body width to avoid wrap drift between hunks.
		truncated := ansiTruncate(raw, width)
		switch {
		case strings.HasPrefix(raw, "+++") || strings.HasPrefix(raw, "---"):
			out = append(out, dimStyle.Render(truncated))
		case strings.HasPrefix(raw, "@@"):
			out = append(out, hunkStyle.Render(truncated))
		case strings.HasPrefix(raw, "+"):
			out = append(out, addStyle.Render(truncated))
		case strings.HasPrefix(raw, "-"):
			out = append(out, delStyle.Render(truncated))
		default:
			out = append(out, truncated)
		}
	}
	return out
}

// renderSplitDiff returns display lines for side-by-side mode. Walks the
// go-udiff edit list, building a synchronized left/right pair of lines
// for each contiguous chunk. Half-width column per side, separated by a
// vertical bar.
func renderSplitDiff(left, right, leftLabel, rightLabel string, width int, t *theme.Theme) []string {
	// 1 separator column ` │ `, 3 chars.
	colW := (width - 3) / 2
	if colW < 8 {
		colW = 8
	}
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Status.Running))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(t.Status.Error))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9DDAEA")).Bold(true)

	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	if left == "" {
		leftLines = nil
	}
	if right == "" {
		rightLines = nil
	}

	// Compute LCS-style alignment via go-udiff's Edit list.
	edits := udiff.Strings(left, right)
	pairs := alignSplitDiff(leftLines, rightLines, edits)

	// fit clips s to colW and right-pads with spaces — split-view column
	// alignment depends on EVERY cell being exactly colW wide. The shared
	// padRight in help.go only pads (no truncate), so long YAML lines
	// (e.g. last-applied-configuration JSON, base64 cert blobs) used to
	// blow past the column and the separator vanished.
	fit := func(s string) string {
		if lipgloss.Width(s) > colW {
			s = ansiTruncate(s, colW)
		}
		return padRight(s, colW)
	}
	out := make([]string, 0, len(pairs)+2)
	// Header row: locked label on the left, compare label on the right.
	out = append(out, headerStyle.Render(fit(leftLabel))+
		sepStyle.Render(" │ ")+
		headerStyle.Render(fit(rightLabel)))
	out = append(out, sepStyle.Render(strings.Repeat("─", colW))+
		sepStyle.Render("─┼─")+
		sepStyle.Render(strings.Repeat("─", colW)))

	if len(pairs) == 0 {
		out = append(out, centerNoDiff(width, t))
		return out
	}
	for _, p := range pairs {
		var ls, rs string
		switch {
		case p.left == "" && p.right != "":
			ls = dimStyle.Render(fit(""))
			rs = addStyle.Render(fit("+ " + p.right))
		case p.left != "" && p.right == "":
			ls = delStyle.Render(fit("- " + p.left))
			rs = dimStyle.Render(fit(""))
		case p.changed:
			ls = delStyle.Render(fit("- " + p.left))
			rs = addStyle.Render(fit("+ " + p.right))
		default:
			ls = fit("  " + p.left)
			rs = fit("  " + p.right)
		}
		out = append(out, ls+sepStyle.Render(" │ ")+rs)
	}
	return out
}

// splitPair is one synchronized row in the split-diff view. left/right
// is the raw line text; changed = both sides present but differ;
// blank-left / blank-right denote insert / delete with the other side
// showing an empty placeholder.
type splitPair struct {
	left    string
	right   string
	changed bool
}

// alignSplitDiff walks both line slices using the go-udiff Edit list as
// a guide. Lines outside of any edit are paired 1:1 (context). Inside
// an edit, we line up insertions on the right and deletions on the
// left; if both happen at the same edit boundary we mark them as
// "changed" so the renderer can colour both sides.
func alignSplitDiff(leftLines, rightLines []string, edits []udiff.Edit) []splitPair {
	if len(edits) == 0 {
		// No diff — pair lines 1:1.
		pairs := make([]splitPair, 0, len(leftLines))
		for i := range leftLines {
			r := ""
			if i < len(rightLines) {
				r = rightLines[i]
			}
			pairs = append(pairs, splitPair{left: leftLines[i], right: r})
		}
		return pairs
	}
	// Convert byte-offset edits into line-index edits by walking left
	// text and recording each edit's start/end line.
	leftRaw := strings.Join(leftLines, "\n")
	li := buildLineIndex(leftRaw)
	type lineEdit struct {
		startLine int
		endLine   int
		newText   string
	}
	lineEdits := make([]lineEdit, 0, len(edits))
	for _, e := range edits {
		lineEdits = append(lineEdits, lineEdit{
			startLine: li.lineAt(e.Start),
			endLine:   li.lineAt(e.End),
			newText:   e.New,
		})
	}
	pairs := make([]splitPair, 0, len(leftLines)+len(rightLines))
	li2 := 0
	for _, le := range lineEdits {
		for ; li2 < le.startLine && li2 < len(leftLines); li2++ {
			pairs = append(pairs, splitPair{left: leftLines[li2], right: leftLines[li2]})
		}
		oldChunk := leftLines[le.startLine:min(le.endLine, len(leftLines))]
		newChunk := strings.Split(strings.TrimRight(le.newText, "\n"), "\n")
		if le.newText == "" {
			newChunk = nil
		}
		// Pair as many as we can; the longer side gets blanks on the
		// other column. Same-index pairs are marked changed because the
		// edit replaced one with the other.
		max := len(oldChunk)
		if len(newChunk) > max {
			max = len(newChunk)
		}
		for i := 0; i < max; i++ {
			var ls, rs string
			if i < len(oldChunk) {
				ls = oldChunk[i]
			}
			if i < len(newChunk) {
				rs = newChunk[i]
			}
			pairs = append(pairs, splitPair{left: ls, right: rs, changed: ls != "" && rs != ""})
		}
		li2 = le.endLine
	}
	for ; li2 < len(leftLines); li2++ {
		pairs = append(pairs, splitPair{left: leftLines[li2], right: leftLines[li2]})
	}
	return pairs
}

// lineIndex maps byte offsets in a string to line numbers. Built once
// per diff so the per-edit lookup is O(log n) (binary search).
type lineIndex struct {
	lineStartOffsets []int
}

func buildLineIndex(s string) lineIndex {
	offsets := []int{0}
	for i, c := range s {
		if c == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return lineIndex{lineStartOffsets: offsets}
}

// lineAt returns the 0-based line index containing byte offset `off`.
// Out-of-range offsets clamp to the last line.
func (li lineIndex) lineAt(off int) int {
	lo, hi := 0, len(li.lineStartOffsets)
	for lo < hi {
		mid := (lo + hi) / 2
		if li.lineStartOffsets[mid] <= off {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo > 0 {
		return lo - 1
	}
	return 0
}

func centerNoDiff(width int, t *theme.Theme) string {
	msg := "(identical — no config diff)"
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	pad := (width - lipgloss.Width(msg)) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + dim.Render(msg)
}

// RenderPopup composes the diff panel onto a centred overlay frame —
// border, title, body, bottom-hint legend.
func (m CompareYamlPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFrame())
}

func (m CompareYamlPopupModel) renderFrame() string {
	popupW := m.popupWidth()
	popupH := m.popupHeight()
	borderColor := lipgloss.Color("#9DDAEA")
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(borderColor).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	title := fmt.Sprintf(" \U000f08aa Compare — %s vs %s (%s) ",
		m.leftLabel, m.rightLabel, m.layout.String())
	titleW := lipgloss.Width(title)
	innerW := popupW - 2
	if innerW < 10 {
		innerW = 10
	}
	leadDashCount := 2
	if titleW+leadDashCount+1 > innerW {
		titleW = innerW - leadDashCount - 1
		title = ansiTruncate(title, titleW)
	}
	leadDashes := strings.Repeat("─", leadDashCount)
	trailLen := innerW - leadDashCount - titleW
	if trailLen < 1 {
		trailLen = 1
	}
	top := borderStyle.Render("╭"+leadDashes) +
		titleStyle.Render(title) +
		borderStyle.Render(strings.Repeat("─", trailLen)+"╮")

	body := m.renderBody(innerW, popupH-2)
	vbar := borderStyle.Render("│")
	bodyRows := strings.Split(body, "\n")
	for i, row := range bodyRows {
		visible := lipgloss.Width(row)
		if visible < innerW {
			row = row + strings.Repeat(" ", innerW-visible)
		} else if visible > innerW {
			row = ansiTruncate(row, innerW)
		}
		bodyRows[i] = vbar + row + vbar
	}

	hint := " Space: menu  j/k: scroll  Esc/q: close "
	hintW := lipgloss.Width(hint)
	// Bottom border target width = innerW + 2 (matches top: ╭ + innerW
	// dashes-or-title + ╮). The earlier "╰─" lead consumed 2 chars but
	// the trailing-dash count subtracted 2 from innerW, leaving the
	// row 1 char short — and the ╯ corner slid 1 cell left of the
	// right vertical bar. Drop the leading "─" and size trailDashes =
	// innerW - hintW so the total comes out to innerW + 2.
	trailDashes := innerW - hintW
	if trailDashes < 1 {
		trailDashes = 1
	}
	bot := borderStyle.Render("╰") + hintStyle.Render(hint) +
		borderStyle.Render(strings.Repeat("─", trailDashes)+"╯")

	parts := []string{top}
	parts = append(parts, bodyRows...)
	parts = append(parts, bot)
	frame := strings.Join(parts, "\n")
	if m.menuOpen {
		frame = m.overlayMenu(frame)
	}
	return frame
}

func (m CompareYamlPopupModel) renderBody(width, height int) string {
	rows := make([]string, 0, height)
	end := m.scrollOffset + height
	if end > len(m.contentLines) {
		end = len(m.contentLines)
	}
	for i := m.scrollOffset; i < end; i++ {
		rows = append(rows, m.contentLines[i])
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return strings.Join(rows, "\n")
}

// overlayMenu draws the Space-triggered action menu over the bottom of
// the diff frame. Plain bordered box, cyan accent matching the popup
// frame.
func (m CompareYamlPopupModel) overlayMenu(frame string) string {
	items := m.menuItems()
	if len(items) == 0 {
		return frame
	}
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9DDAEA"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).
		Background(lipgloss.Color("#9DDAEA")).Bold(true)
	rowStyle := lipgloss.NewStyle()
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	hint := " enter: select  esc: cancel "
	hintW := lipgloss.Width(hint)
	// Inner width must accommodate the widest item (+2 padding for the
	// leading/trailing single space inside the row) AND the bottom hint
	// — otherwise the bottom border stretches past the side bars and
	// the corners no longer line up vertically.
	innerW := hintW
	for _, it := range items {
		w := lipgloss.Width(it) + 2 // leading + trailing spaces
		if w > innerW {
			innerW = w
		}
	}

	rows := []string{borderStyle.Render("╭" + strings.Repeat("─", innerW) + "╮")}
	for i, it := range items {
		text := " " + it + strings.Repeat(" ", innerW-1-lipgloss.Width(it))
		if i == m.menuCursor {
			text = cursorStyle.Render(text)
		} else {
			text = rowStyle.Render(text)
		}
		rows = append(rows, borderStyle.Render("│")+text+borderStyle.Render("│"))
	}
	tail := innerW - hintW
	if tail < 0 {
		tail = 0
	}
	rows = append(rows, borderStyle.Render("╰")+hintStyle.Render(hint)+
		borderStyle.Render(strings.Repeat("─", tail)+"╯"))

	menuBlock := strings.Join(rows, "\n")
	// Compose the menu on top of the diff frame using the same overlay
	// engine the top-level popup stack uses. lipgloss.Place was an
	// earlier attempt — it broke up the border across the popup because
	// per-line width measurement on multi-line bordered content gets
	// confused by ANSI styling.
	return overlay.Composite(menuBlock, frame, overlay.Center, overlay.Center, 0, 0)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
