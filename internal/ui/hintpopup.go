package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/theme"
)

// HintPopupModel is the "what can I do here?" popup. Used wherever a
// panel / tab might want to surface its keybindings. Content (title +
// rows) is passed in at Open() so one instance serves multiple call
// sites (sidebar, Logs tab, Events tab, Relatives-at-depth-1).
//
// By default the popup is READ-ONLY — Update only handles close keys.
// If OpenWithActions is used instead of Open, the popup grows an
// interactive top region: a single-column action list. Cursor + Enter
// commits an action, or the action's single-letter hotkey commits
// directly. Used by the panel-1 sidebar Space menu so the same popup
// can carry the cheatsheet AND the contextual Pin / Unpin action.
type HintPopupModel struct {
	animator PopupAnimator
	screenW  int
	theme    *theme.Theme

	// Captured at Open — re-render uses these.
	title   string
	rows    []hintRow
	actions []hintAction
	cursor  int // index into actions; ignored when len(actions) == 0
}

type hintRow struct {
	keys string // "j/k", "Enter", "/", ...
	hint string // one-line description
}

// hintAction is one interactive entry rendered in the top region of
// the popup. Key is the single-letter hotkey (bracketed in label via
// bracketHotkey, dispatched directly on press). Action is the opaque
// identifier emitted via HintActionMsg so the caller routes commits.
type hintAction struct {
	label  string // "Pin Pods" / "Unpin Pods" / ...
	key    string // single-letter hotkey, e.g. "P"
	action string // commit identifier passed back in HintActionMsg
}

// HintActionMsg is emitted when the user commits one of the popup's
// actions (via cursor + Enter or direct hotkey letter). Action is the
// string the action was registered with at OpenWithActions time.
type HintActionMsg struct {
	Action string
}

func NewHintPopupModel(t *theme.Theme) HintPopupModel {
	return HintPopupModel{
		theme:    t,
		animator: NewPopupAnimator("hintpopup", lipgloss.Color("#cba6f7")),
	}
}

// Open shows the popup with the given title + rows. No interactive
// actions — popup is read-only, every key closes or is ignored.
func (m *HintPopupModel) Open(title string, rows []hintRow) tea.Cmd {
	m.title = title
	m.rows = rows
	m.actions = nil
	m.cursor = 0
	return m.animator.Open()
}

// OpenWithActions opens the popup with both an interactive top region
// (actions) and the read-only cheatsheet below. Cursor starts at index
// 0 of actions. j/k navigate, Enter commits, single-letter hotkey
// commits directly. Empty actions slice degrades to the same behaviour
// as Open.
func (m *HintPopupModel) OpenWithActions(title string, actions []hintAction, rows []hintRow) tea.Cmd {
	m.title = title
	m.rows = rows
	m.actions = actions
	m.cursor = 0
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

// Update handles close keys + (when actions present) cursor navigation
// and commit. Mirror the panel2 menu's close set (Esc / q / Space).
// Enter closes only when there are no actions — with actions, Enter
// commits the cursor item.
func (m HintPopupModel) Update(msg tea.Msg) (HintPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	key := keyMsg.String()
	switch key {
	case "esc", "q", " ":
		return m, m.animator.Close()
	}
	if len(m.actions) == 0 {
		// Read-only popup — Enter also closes for the friendly
		// "press anything to dismiss" feel.
		if key == "enter" {
			return m, m.animator.Close()
		}
		return m, nil
	}
	switch key {
	case "j", "down":
		if m.cursor < len(m.actions)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.actions) {
			return m, nil
		}
		return m, m.commitAction(m.actions[m.cursor].action)
	default:
		// Direct hotkey trigger — must match an action's registered
		// key (case-sensitive). Unknown keys fall through to no-op so
		// stray presses don't close the popup.
		for _, a := range m.actions {
			if a.key != "" && key == a.key {
				return m, m.commitAction(a.action)
			}
		}
	}
	return m, nil
}

// commitAction returns the Cmd batch that closes the popup AND emits
// the action message back to AppModel. Bundled so the trigger paths
// (Enter on cursor / direct hotkey) cannot diverge — every action
// commit closes the popup.
func (m *HintPopupModel) commitAction(action string) tea.Cmd {
	closeCmd := m.animator.Close()
	actionCmd := func() tea.Msg { return HintActionMsg{Action: action} }
	return tea.Batch(closeCmd, actionCmd)
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
		{keys: "P", hint: "toggle pinned — pin / unpin the cursor's resource kind"},
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

	// Build the action rows (top, interactive) if any. Cursor row gets
	// reverse-video; non-cursor rows render in the popup's accent
	// colour, dimmed, with the hotkey letter bracketed via
	// bracketHotkey (same visual convention as panel-2 Y/E/S/D).
	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1e1e2e")).
		Background(bc).Bold(true)
	actionStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	var actionRows []string
	for i, a := range m.actions {
		bracketed := bracketHotkey(a.label, a.key)
		body := " " + bracketed
		plainW := lipgloss.Width(body)
		pad := ""
		if plainW < innerW {
			pad = strings.Repeat(" ", innerW-plainW)
		}
		row := body + pad
		if i == m.cursor {
			row = cursorStyle.Render(row)
		} else {
			row = actionStyle.Render(row)
		}
		actionRows = append(actionRows, row)
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")
	b.WriteString(padRow)
	for _, line := range actionRows {
		b.WriteString(left + line + right + "\n")
	}
	if len(actionRows) > 0 {
		// Visual separator between the action region and the read-only
		// cheatsheet below — same dim-grey horizontal rule the popup
		// border uses, inset by one column on each side so it reads
		// as an internal divider, not a re-doubled border.
		sep := bStyle.Render(strings.Repeat("─", innerW))
		b.WriteString(left + sep + right + "\n")
	}
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
