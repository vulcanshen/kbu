package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/theme"
)

// SidebarHelpPopupModel is the read-only cheatsheet opened by Space on
// panel 1. Each row of the sidebar is itself a navigation target, so a
// per-row action menu would be empty — instead this popup surfaces what
// the user can DO on the sidebar (move, drill, search, lock). Mental
// model: same "Space = surface what's possible here" affordance as the
// per-row menu on panel 2, but informational rather than actionable.
type SidebarHelpPopupModel struct {
	animator PopupAnimator
	screenW  int
	theme    *theme.Theme
}

type sidebarHelpRow struct {
	keys string // "j/k", "Enter", "/", ...
	hint string // one-line description
}

// sidebarHelpRows is fixed content — the keys that drive the sidebar
// don't change at runtime. Order matches the typical user flow:
// browse → focus into → search → lock → clear.
// "󰘍 " indent under "/" matches the drill-into glyph used in the
// Relatives tab so users associate the same arrow with the same idea
// ("this is a child of the previous row"). Without indent the search-
// mode Enter looked like a duplicate of the normal-mode Enter.
var sidebarHelpRows = []sidebarHelpRow{
	{"j/k", "move cursor (also ↓/↑)"},
	{"Enter", "into — focus the selected resource into panel 2"},
	{"/", "search by name; type to filter"},
	{"󰘍 Enter", "while searching: lock the filter and exit search mode"},
	{"󰘍 Esc", "clear search / exit search mode"},
	{"N", "switch namespace (global)"},
	{"C", "switch context (global)"},
}

func NewSidebarHelpPopupModel(t *theme.Theme) SidebarHelpPopupModel {
	return SidebarHelpPopupModel{
		theme:    t,
		animator: NewPopupAnimator("sidebarhelp", lipgloss.Color("#cba6f7")),
	}
}

func (m *SidebarHelpPopupModel) Open() tea.Cmd       { return m.animator.Open() }
func (m *SidebarHelpPopupModel) Close() tea.Cmd      { return m.animator.Close() }
func (m *SidebarHelpPopupModel) SetSize(w, _ int)    { m.screenW = w }
func (m SidebarHelpPopupModel) IsActive() bool       { return m.animator.IsActive() }
func (m SidebarHelpPopupModel) IsInteractive() bool  { return m.animator.IsInteractive() }

func (m *SidebarHelpPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// Update only handles close keys — there's nothing to commit. Mirror
// the panel2 menu's close set (Esc / q / Space) for consistency.
func (m SidebarHelpPopupModel) Update(msg tea.Msg) (SidebarHelpPopupModel, tea.Cmd) {
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

func (m SidebarHelpPopupModel) View() string { return "" }

// wrapText breaks s into lines no wider than w cells, splitting on spaces.
// A single word longer than w is left intact rather than hard-cut (it'll
// poke past the border, which is preferable to mangling an identifier).
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

func (m SidebarHelpPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m SidebarHelpPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#cba6f7")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))

	title := " 󰥋 km8 — what can I do here?"
	bottomHint := " Esc/q/Space: close "

	// Layout: " <key:keyColW>  <hint…>" where the hint can wrap onto
	// continuation lines that leave the key column empty. innerW is
	// chosen aggressively so the popup fits in narrow terminals; hint
	// wrapping handles whatever doesn't fit on one line.
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
			// Cap on very wide terminals — long hints read better wrapped
			// than stretched across 200 cells.
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

	// Hint area: innerW minus leading space, key column, 2-space gap,
	// and trailing space before the border.
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
	for _, r := range sidebarHelpRows {
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
