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
		{"j/k", "move cursor (also ↓/↑)"},
		{"Enter", "into — focus the selected resource into panel 2"},
		{"/", "search by name; type to filter"},
		{drillArrow + " Enter", "while searching: lock the filter and exit search mode"},
		{drillArrow + " Esc", "clear search / exit search mode"},
		{"N", "switch namespace (global)"},
		{"C", "switch context (global)"},
	}
	return title, rows
}

func logsHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Logs — what can I do here?"
	rows := []hintRow{
		{"j/k", "scroll one line (also ↓/↑)"},
		{"u/d", "scroll half a page"},
		{"gg", "jump to top — pauses auto-follow"},
		{"G", "jump to live tail — resumes auto-follow"},
		{"y", "copy entire log buffer to clipboard"},
		{"z", "toggle full-screen panel"},
	}
	return title, rows
}

func eventsHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Events — what can I do here?"
	rows := []hintRow{
		{"j/k", "scroll one line (also ↓/↑)"},
		{"u/d", "scroll half a page"},
		{"gg/G", "jump to top / bottom"},
		{"y", "copy events to clipboard"},
		{"z", "toggle full-screen panel"},
	}
	return title, rows
}

func panel2EmptyHintContent() (string, []hintRow) {
	title := " " + titleIcon + " No items here — try"
	rows := []hintRow{
		{"N", "switch to a different namespace"},
		{"/", "current filter might be hiding rows"},
		{".", "toggle helm-managed visibility — items may be hidden"},
		{"C", "switch to a different context"},
	}
	return title, rows
}

func relativesDrillHintContent() (string, []hintRow) {
	title := " " + titleIcon + " Relatives — what can I do here?"
	rows := []hintRow{
		{"j/k", "move cursor over related refs"},
		{"Enter", "drill into the cursor's resource — chain extends"},
		{"Y", "open the cursor row's YAML in a popup"},
		{"Esc", "pop back one drill level"},
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
	bottomHint := " Esc/q/Space: close "

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
