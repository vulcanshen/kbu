package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/kbu/internal/theme"
)

// ListPickerModel is the generic "pick one from a list" popup, used
// wherever a flow needs the user to choose between named options. The
// sort flow runs two instances: the column picker and, stacked on it,
// the direction picker — each step of a multi-step flow is its own
// popup with its own height (tdp F1, F4). pickerID tags the step so the
// app-side switch knows which one is committing.
type ListPickerModel struct {
	animator PopupAnimator
	pickerID string
	title    string
	items    []ListPickerItem
	cursor   int
	screenW  int
	theme    *theme.Theme

	layer       int
	borderColor lipgloss.Color
}

// ListPickerItem is one row in the picker.
//
//   - Key:       emitted in ListPickerActionMsg on commit; app.go
//     switches on it.
//   - Label:     visible row text (left side); for Header items the
//     label IS the region heading (rendered dim grey).
//   - Hint:      right-side dim text (optional). Suppresses cleanly
//     when empty so plain pickers don't render a phantom
//     gap.
//   - Badge:     small marker shown after Label (e.g. "↑" / "↓" for
//     the current sort column, "current" for the current
//     direction). Optional.
//   - Separator: when true the row is a non-selectable visual divider;
//     j/k/g/G + mouse clicks skip past it, Enter never
//     commits it. Other fields are ignored.
//   - Header:    when true the row is a non-selectable region label
//     rendered in dim grey above the items it introduces.
//     Same navigation skip rules as Separator. Used by
//     pickers that mix multiple operation kinds (e.g. sort
//     column picker: "fields" above the column list,
//     "all" above the Reset shortcut).
//   - Disabled:  the row exists but can't run right now (tdp M6) —
//     dimmed; the cursor can rest on it, Enter and clicks do nothing.
type ListPickerItem struct {
	Key       string
	Label     string
	Hint      string
	Badge     string
	Separator bool
	Header    bool
	Disabled  bool
}

// ListPickerActionMsg is emitted when the user commits a row (Enter
// or single-letter hotkey if Key is one character). PickerID is the
// tag passed to Open — app.go switches on it to route the commit to
// the right handler (column step vs direction step).
type ListPickerActionMsg struct {
	PickerID string
	Key      string
}

// ListPickerCancelMsg is emitted on Esc (or a right-click). PickerID tags
// the cancelled step so app.go can drop any in-flight flow state
// (e.g. the cached column from the column step when direction is
// cancelled).
type ListPickerCancelMsg struct {
	PickerID string
}

func NewListPickerModel(t *theme.Theme) ListPickerModel {
	return newListPickerModel(t, "listpicker")
}

// NewSortDirPickerModel is the sort flow's second step, stacked on the
// column picker.
func NewSortDirPickerModel(t *theme.Theme) ListPickerModel {
	return newListPickerModel(t, "listpicker_dir")
}

func newListPickerModel(t *theme.Theme, target string) ListPickerModel {
	bc := theme.PopupLayerColor(1)
	return ListPickerModel{
		theme:       t,
		animator:    NewPopupAnimator(target, bc),
		borderColor: bc,
		layer:       1,
	}
}

// SetLayer stamps nesting depth + derives border / animator color.
func (m *ListPickerModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	m.animator.Color = m.borderColor
}

// Open shows the picker with the given title + items. The cursor starts
// on the first item with Badge == "current" (so the user sees where they
// are now), or on the first selectable row.
func (m *ListPickerModel) Open(pickerID, title string, items []ListPickerItem) tea.Cmd {
	m.pickerID = pickerID
	m.title = title
	m.items = items
	m.cursor = m.firstSelectable()
	for i, it := range items {
		if !it.Separator && !it.Header && it.Badge == "current" {
			m.cursor = i
			break
		}
	}
	return m.animator.Open()
}

// SetItems refreshes the rows of an open picker in place — badges after a
// sort tier lands, a row dimmed or lit — keeping the cursor where the
// user left it. The row count stays what it was at Open (tdp F7).
func (m *ListPickerModel) SetItems(items []ListPickerItem) {
	m.items = items
	if m.cursor >= len(items) {
		m.cursor = m.firstSelectable()
	}
}

// firstSelectable returns the index of the first non-separator item,
// or 0 if every item is a separator (shouldn't happen but guards
// against pathological input).
func (m ListPickerModel) firstSelectable() int {
	for i, it := range m.items {
		if !it.Separator && !it.Header {
			return i
		}
	}
	return 0
}

// lastSelectable returns the index of the last non-separator item,
// or len(items)-1 as a fallback.
func (m ListPickerModel) lastSelectable() int {
	for i := len(m.items) - 1; i >= 0; i-- {
		if !m.items[i].Separator && !m.items[i].Header {
			return i
		}
	}
	if len(m.items) == 0 {
		return 0
	}
	return len(m.items) - 1
}

// nextSelectable cycles cursor forward to the next non-separator
// item. Returns the current cursor when no selectable rows exist.
func (m ListPickerModel) nextSelectable(from int) int {
	n := len(m.items)
	if n == 0 {
		return from
	}
	for step := 1; step <= n; step++ {
		idx := (from + step) % n
		if !m.items[idx].Separator && !m.items[idx].Header {
			return idx
		}
	}
	return from
}

// prevSelectable cycles cursor backward to the previous
// non-separator item.
func (m ListPickerModel) prevSelectable(from int) int {
	n := len(m.items)
	if n == 0 {
		return from
	}
	for step := 1; step <= n; step++ {
		idx := (from - step + n) % n
		if !m.items[idx].Separator && !m.items[idx].Header {
			return idx
		}
	}
	return from
}

func (m *ListPickerModel) Close() tea.Cmd     { return m.animator.Close() }
func (m *ListPickerModel) SetSize(w, _ int)   { m.screenW = w }
func (m ListPickerModel) IsActive() bool      { return m.animator.IsActive() }
func (m ListPickerModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *ListPickerModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m ListPickerModel) Update(msg tea.Msg) (ListPickerModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "j", "down":
		m.cursor = m.nextSelectable(m.cursor)
		return m, nil
	case "k", "up":
		m.cursor = m.prevSelectable(m.cursor)
		return m, nil
	case "g":
		m.cursor = m.firstSelectable()
		return m, nil
	case "G":
		m.cursor = m.lastSelectable()
		return m, nil
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.items) {
			return m, nil
		}
		if it := m.items[m.cursor]; it.Separator || it.Header || it.Disabled {
			return m, nil
		}
		return m, m.commit(m.items[m.cursor].Key)
	case "esc":
		// Cancel — emit a tagged msg so app.go can drop in-flight
		// flow state, then run the close animation. Order: close
		// cmd comes FIRST so the popup starts closing immediately;
		// the cancel msg is observed by app.go and tears down any
		// cached step state (e.g. chosen column from a prior step).
		pickerID := m.pickerID
		closeCmd := m.animator.Close()
		cancelCmd := func() tea.Msg { return ListPickerCancelMsg{PickerID: pickerID} }
		return m, tea.Batch(closeCmd, cancelCmd)
	}
	return m, nil
}

// HandleMouse routes clicks against the rendered picker. Left-click
// on an item commits that row (cursor moves to it + the same msg
// keyboard Enter would have fired). Right-click closes via the
// cancel path, so chained flows (sort: column → direction) drop
// their cached state the same way as Esc.
func (m ListPickerModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (ListPickerModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	row := popupRowAt(m.renderFullPopup(), msg, screenW, screenH, 2, len(m.items))
	if row < 0 {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		if it := m.items[row]; it.Separator || it.Header || it.Disabled {
			return m, nil
		}
		m.cursor = row
		return m, m.commit(m.items[row].Key)
	case tea.MouseButtonRight:
		pickerID := m.pickerID
		closeCmd := m.animator.Close()
		cancelCmd := func() tea.Msg { return ListPickerCancelMsg{PickerID: pickerID} }
		return m, tea.Batch(closeCmd, cancelCmd)
	}
	return m, nil
}

// commit emits the action msg WITHOUT running the close animation.
// app.go decides what the step does next: open the next step over it,
// refresh it in place, or close it.
func (m *ListPickerModel) commit(key string) tea.Cmd {
	pickerID := m.pickerID
	return func() tea.Msg {
		return ListPickerActionMsg{PickerID: pickerID, Key: key}
	}
}

func (m ListPickerModel) View() string { return "" }

func (m ListPickerModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m ListPickerModel) renderFullPopup() string {
	bc := m.borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Overlay0))
	dimCursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c")).Background(lipgloss.Color("#45475a"))

	title := " " + m.title + " "
	bottomHint := " j/k: move  Enter: pick  Esc: cancel "

	// tdp F7: one width for every popup, whatever it shows.
	innerW := popupInnerWidth(m.screenW)

	const gutter = "  "
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	var rows []string
	for i, it := range m.items {
		if it.Header {
			// Region label — dim grey, indented by gutter so it
			// hangs over the items below. No background, no border —
			// reads as section heading, not a clickable row.
			rows = append(rows, " "+gutter+headerStyle.Render(it.Label))
			continue
		}
		if it.Separator {
			// Same purple horizontal rule the hintpopup uses to split
			// its action region from the cheatsheet below — keeps
			// every kbu popup's internal divider visually consistent.
			rows = append(rows, bStyle.Render(strings.Repeat("─", innerW)))
			continue
		}
		isCursor := i == m.cursor
		label := it.Label
		// Badge sits inline right after the label, separated by a
		// single space so a cursor-highlighted row's reverse
		// background includes the badge.
		if it.Badge != "" {
			label = label + " " + it.Badge
		}
		bodyLeft := " " + gutter + label
		var hintPart string
		if it.Hint != "" {
			hintPart = "    " + it.Hint
		}
		bodyPlain := bodyLeft + hintPart
		padW := innerW - 1 - lipgloss.Width(bodyPlain)
		if padW < 0 {
			padW = 0
		}
		pad := strings.Repeat(" ", padW)
		if isCursor && it.Disabled {
			rows = append(rows, dimCursorStyle.Render(bodyPlain+pad))
			continue
		}
		if isCursor {
			rows = append(rows, cursorStyle.Render(bodyPlain+pad))
			continue
		}
		if it.Disabled {
			rows = append(rows, dimStyle.Render(bodyPlain)+pad)
			continue
		}
		// Non-cursor: keep label plain, dim hint, accent badge.
		styledLabel := " " + gutter + it.Label
		if it.Badge != "" {
			styledLabel += " " + badgeStyle.Render(it.Badge)
		}
		styledLine := styledLabel
		if it.Hint != "" {
			styledLine += "    " + hintStyle.Render(it.Hint)
		}
		rows = append(rows, styledLine+pad)
	}

	dashesAfter := innerW - 1 - lipgloss.Width(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")
	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right + "\n"
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
