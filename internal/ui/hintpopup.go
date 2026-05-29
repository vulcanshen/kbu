package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/theme"
)

// HintPopupModel is the read-only "what can I do here?" popup. Used wherever
// a panel / tab has no actionable per-row menu but the user might still want
// to know which keys work. Content (title + rows) is passed in at Open() so
// one instance serves multiple call sites (sidebar, Logs tab, Events tab,
// Relatives-at-depth-1).
type HintPopupModel struct {
	animator PopupAnimator
	screenW  int
	theme    *theme.Theme

	// Captured at Open — re-render uses these.
	title string
	rows  []hintRow
}

type hintRow struct {
	keys string // "j/k", "Enter", "/", ...
	hint string // one-line description
}

func NewHintPopupModel(t *theme.Theme) HintPopupModel {
	return HintPopupModel{
		theme:    t,
		animator: NewPopupAnimator("hintpopup", lipgloss.Color("#cba6f7")),
	}
}

// Open shows the popup with the given title + rows.
func (m *HintPopupModel) Open(title string, rows []hintRow) tea.Cmd {
	m.title = title
	m.rows = rows
	return m.animator.Open()
}

func (m *HintPopupModel) Close() tea.Cmd     { return m.animator.Close() }
func (m *HintPopupModel) SetSize(w, _ int)   { m.screenW = w }
func (m HintPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m HintPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *HintPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// Update only handles close keys — nothing to commit. Mirror the panel2 menu's
// close set (Esc / q / Space) plus Enter as a friendly close-on-confirm.
func (m HintPopupModel) Update(msg tea.Msg) (HintPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc", "q", " ", "enter":
		return m, m.animator.Close()
	}
	return m, nil
}

func (m HintPopupModel) View() string { return "" }

// ── content builders ──────────────────────────────────────────────────────

// drillDownIcon (U+F0FC4) and drillUpIcon (U+F0FC5) are the directionally-
// symmetric glyphs paired with the "Enter" (drill in) and "Esc" (drill out
// / pop level) actions across menus and hint popups. Used by both panel 2's
// per-row context menu and panel 3's Relatives hint to give those two
// actions a consistent visual identity.
const drillDownIcon = "\U000f0fc4"
const drillUpIcon = "\U000f0fc5"

// drillArrow (\U000f060d) is the southeast/right arrow used in the Relatives
// tab. We embed it before sub-mode keys to mark visual nesting — without it,
// search-mode Enter looked like a duplicate of normal-mode Enter.
const drillArrow = "\U000f060d"

// titleIcon is the km8 wheel glyph (\U000f094b) — same across all hint popups
// so the user associates the icon with "this is the cheatsheet for here".
const titleIcon = "\U000f094b"

func sidebarHintContent() (string, []hintRow) {
	title := " " + titleIcon + " km8 — what can I do here?"
	rows := []hintRow{
		{keys: "j/k", hint: "move cursor (also ↓/↑)"},
		{keys: "Enter", hint: "into — focus the selected resource into panel 2"},
		{keys: "/", hint: "search by name; type to filter"},
		{keys: drillArrow + " Enter", hint: "while searching: lock the filter and exit search mode"},
		{keys: drillArrow + " Esc", hint: "clear search / exit search mode"},
		{keys: "N", hint: "switch namespace (global)"},
		{keys: "C", hint: "switch context (global)"},
	}
	return title, rows
}

func logsHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Logs — what can I do here?"
	rows := []hintRow{
		{keys: "j/k", hint: "scroll one line (also ↓/↑)"},
		{keys: "u/d", hint: "scroll half a page"},
		{keys: "gg", hint: "jump to top — pauses auto-follow"},
		{keys: "G", hint: "jump to live tail — resumes auto-follow"},
		{keys: "y", hint: "copy entire log buffer to clipboard"},
		{keys: "z", hint: "toggle full-screen panel"},
	}
	return title, rows
}

func eventsHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Events — what can I do here?"
	rows := []hintRow{
		{keys: "j/k", hint: "scroll one line (also ↓/↑)"},
		{keys: "u/d", hint: "scroll half a page"},
		{keys: "gg/G", hint: "jump to top / bottom"},
		{keys: "y", hint: "copy events to clipboard"},
		{keys: "z", hint: "toggle full-screen panel"},
	}
	return title, rows
}

func conditionsHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Conditions — what can I do here?"
	rows := []hintRow{
		{keys: "j/k", hint: "scroll one line (also ↓/↑)"},
		{keys: "u/d", hint: "scroll half a page"},
		{keys: "gg/G", hint: "jump to top / bottom"},
		{keys: "y", hint: "copy conditions to clipboard"},
		{keys: "z", hint: "toggle full-screen panel"},
	}
	return title, rows
}

func panel2EmptyHintContent() (string, []hintRow) {
	title := " " + titleIcon + " No items here — try"
	rows := []hintRow{
		{keys: "N", hint: "switch to a different namespace"},
		{keys: "/", hint: "current filter might be hiding rows"},
		{keys: ".", hint: "toggle helm-managed visibility — items may be hidden"},
		{keys: "C", hint: "switch to a different context"},
	}
	return title, rows
}

func relativesDrillHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Relatives — what can I do here?"
	// Depth=1 only — no parent in the chain to pop back to, so no Esc row.
	// At depth>1 the Space handler opens the breadcrumb popup instead.
	rows := []hintRow{
		{keys: "j/k", hint: "move cursor over related refs"},
		{keys: "Y", hint: "open the cursor row's YAML in a popup"},
		{keys: "Enter " + drillDownIcon, hint: "drill into the cursor's resource — chain extends"},
	}
	return title, rows
}

// wrapText breaks s into lines no wider than w cells, splitting on spaces.
// A single word longer than w is left intact rather than hard-cut.
func wrapText(s string, w int) []string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	var lines []string
	var cur strings.Builder
	curW := 0
	for _, word := range words {
		wW := lipgloss.Width(word)
		if curW == 0 {
			cur.WriteString(word)
			curW = wW
			continue
		}
		if curW+1+wW <= w {
			cur.WriteByte(' ')
			cur.WriteString(word)
			curW += 1 + wW
			continue
		}
		lines = append(lines, cur.String())
		cur.Reset()
		cur.WriteString(word)
		curW = wW
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func (m HintPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m HintPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#cba6f7")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))

	title := m.title
	bottomHint := " Space: close "

	const keyColW = 10
	innerW := 60
	if m.screenW > 0 {
		w := m.screenW * 75 / 100
		if w < 50 {
			w = 50
		}
		if w < innerW {
			innerW = w
		} else if w > 80 {
			innerW = 80
		} else {
			innerW = w
		}
	}
	if w := lipgloss.Width(title) + 4; w > innerW {
		innerW = w
	}
	if w := lipgloss.Width(bottomHint) + 4; w > innerW {
		innerW = w
	}

	hintW := innerW - 1 - keyColW - 2 - 1
	if hintW < 10 {
		hintW = 10
	}

	keyPad := func(keys string) string {
		n := keyColW - lipgloss.Width(keys)
		if n < 0 {
			n = 0
		}
		return strings.Repeat(" ", n)
	}
	emptyKeyCol := strings.Repeat(" ", keyColW)
	padToInner := func(plainWidth int) string {
		n := innerW - plainWidth
		if n < 0 {
			n = 0
		}
		return strings.Repeat(" ", n)
	}

	var rows []string
	for _, r := range m.rows {
		lines := wrapText(r.hint, hintW)
		for i, ln := range lines {
			var body string
			plainW := 1 + keyColW + 2 + lipgloss.Width(ln)
			if i == 0 {
				body = " " + keyStyle.Render(r.keys) + keyPad(r.keys) + "  " + hintStyle.Render(ln)
			} else {
				body = " " + emptyKeyCol + "  " + hintStyle.Render(ln)
			}
			rows = append(rows, body+padToInner(plainW))
		}
	}

	dashesAfter := innerW - 1 - lipgloss.Width(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right + "\n"

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")
	b.WriteString(padRow)
	for _, line := range rows {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(left + line + pad + right + "\n")
	}
	b.WriteString(padRow)
	bottomDashes := innerW - lipgloss.Width(bottomHint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(bottomHint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))
	return b.String()
}
