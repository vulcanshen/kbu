package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/theme"
)

// SettingsPopupModel is the global app-settings popup opened by the
// `>` (shift+.) hotkey. Currently it carries a single row (Mouse on/off),
// but the structure is designed to grow — each row is a SettingsItem
// with a label and a coloured value badge, cursor + Enter on a row
// emits SettingsToggleMsg with that row's Key so AppModel can apply
// the toggle and persist.
//
// AppModel owns the actual config mutation. The popup is a pure
// presenter: it doesn't read or write config directly, just renders
// whatever items it was opened with and emits messages on commit.
// After a toggle, AppModel calls SetItems with the new state and the
// popup re-renders in place — same animator-doesn't-restart trick the
// listPicker uses for chained pickers.
type SettingsPopupModel struct {
	animator    PopupAnimator
	items       []SettingsItem
	cursor      int
	screenW     int
	theme       *theme.Theme
	layer       int
	borderColor lipgloss.Color
}

// SettingsItem is one row in the popup. Key identifies the row in
// SettingsToggleMsg; Label is the human-readable left-side text;
// ValueText is the right-side badge ("ON" / "OFF" — caller supplies
// the string so future non-boolean settings reuse the shape); ValueOn
// drives the badge colour (lavender when true — "user-set persistent
// state" mindset color; grey when false — inactive).
type SettingsItem struct {
	Key       string
	Label     string
	ValueText string
	ValueOn   bool
}

// SettingsToggleMsg is emitted when the user commits a row (Enter on
// cursor). Key matches the row's SettingsItem.Key. AppModel switches
// on Key to perform the actual toggle + persistence.
type SettingsToggleMsg struct {
	Key string
}

func NewSettingsPopupModel(t *theme.Theme) SettingsPopupModel {
	bc := theme.PopupLayerColor(1)
	return SettingsPopupModel{
		theme:       t,
		animator:    NewPopupAnimator("settingspopup", bc),
		borderColor: bc,
		layer:       1,
	}
}

// SetLayer stamps nesting depth + derives border / animator color.
func (m *SettingsPopupModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	m.animator.Color = m.borderColor
}

// Open shows the popup with the given items. Cursor resets to 0.
// Subsequent Open calls (e.g. AppModel re-opening after a toggle to
// refresh values) detect "already open" and swap content in place
// without re-running the open animation.
func (m *SettingsPopupModel) Open(items []SettingsItem) tea.Cmd {
	m.items = items
	m.cursor = 0
	if m.animator.State == PopupOpen {
		return nil
	}
	return m.animator.Open()
}

// SetItems updates items without touching cursor — used after a
// toggle to refresh the displayed value badge while keeping the
// user's place in the list. No-op when popup isn't open.
func (m *SettingsPopupModel) SetItems(items []SettingsItem) {
	if !m.animator.IsActive() {
		return
	}
	m.items = items
	if m.cursor >= len(items) {
		m.cursor = 0
	}
}

func (m *SettingsPopupModel) Close() tea.Cmd     { return m.animator.Close() }
func (m *SettingsPopupModel) SetSize(w, _ int)   { m.screenW = w }
func (m SettingsPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m SettingsPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *SettingsPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m SettingsPopupModel) Update(msg tea.Msg) (SettingsPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "j", "down":
		if len(m.items) > 0 {
			m.cursor = (m.cursor + 1) % len(m.items)
		}
		return m, nil
	case "k", "up":
		if len(m.items) > 0 {
			m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
		}
		return m, nil
	case "g":
		m.cursor = 0
		return m, nil
	case "G":
		m.cursor = len(m.items) - 1
		return m, nil
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.items) {
			return m, nil
		}
		key := m.items[m.cursor].Key
		return m, func() tea.Msg { return SettingsToggleMsg{Key: key} }
	case "esc", " ", ">":
		return m, m.animator.Close()
	}
	return m, nil
}

// HandleMouse routes a click against the popup's currently rendered
// frame. Left-click on a row commits that row's toggle (same as
// keyboard Enter); right-click on a row closes the popup (mirror of
// Esc). Clicks outside the popup or on padding / borders are silent
// no-ops — don't dismiss accidentally, don't fire bogus toggles.
//
// screenW/screenH are the terminal dimensions — needed to derive
// where the centered popup actually sits since the popup itself
// doesn't track its own absolute position.
func (m SettingsPopupModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (SettingsPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	row := popupRowAt(m.renderFullPopup(), msg, screenW, screenH, 2, len(m.items))
	if row < 0 {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		m.cursor = row
		key := m.items[row].Key
		return m, func() tea.Msg { return SettingsToggleMsg{Key: key} }
	case tea.MouseButtonRight:
		return m, m.animator.Close()
	}
	return m, nil
}

func (m SettingsPopupModel) View() string { return "" }

func (m SettingsPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m SettingsPopupModel) renderFullPopup() string {
	bc := m.borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)
	onStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe")).Bold(true)  // lavender — user-set persistent state
	offStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c")).Bold(true) // catppuccin overlay1 — inactive

	//  is the Nerd Font cog glyph (U+F013) — the canonical "settings"
	// icon shared across most TUI / GUI apps. Leading + trailing space
	// keep it from butting against the border corner.
	title := "  Settings "
	bottomHint := " j/k: move  Enter: toggle  Esc: close "

	// Width: pick widest of title / bottom hint / rows; clamp to 85% screen.
	maxInnerW := 50
	if m.screenW > 0 {
		maxInnerW = m.screenW * 85 / 100
		if maxInnerW < 36 {
			maxInnerW = 36
		}
	}
	innerW := lipgloss.Width(title) + 4
	if w := lipgloss.Width(bottomHint) + 4; w > innerW {
		innerW = w
	}
	for _, it := range m.items {
		// Row shape: "  Label   ...padding...   ValueText  "
		// Need at least: 1 + 2 + label + 4 + value + 2 = label+value+9.
		w := 1 + 2 + lipgloss.Width(it.Label) + 4 + lipgloss.Width(it.ValueText) + 2
		if w > innerW {
			innerW = w
		}
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	// Cursor row collapses ON/OFF color distinction to the same dark
	// text on the popup-layer accent background — the "ON"/"OFF" word
	// itself carries the state signal, color stays uniform so the
	// row reads as a single reverse-of-title chip (same pattern as
	// other popups' cursor row).
	var rows []string
	for i, it := range m.items {
		isCursor := i == m.cursor
		labelW := lipgloss.Width(it.Label)
		valueW := lipgloss.Width(it.ValueText)
		// "  Label   ...   ValueText  " — left/right pad to innerW.
		gap := innerW - 3 - labelW - valueW - 2 // 3 = " " + "  " left, 2 = "  " right
		if gap < 2 {
			gap = 2
		}
		if isCursor {
			cursorLine := " " + "  " + it.Label + strings.Repeat(" ", gap) + it.ValueText + "  "
			rows = append(rows, cursorStyle.Render(cursorLine))
			continue
		}
		valueStyled := offStyle.Render(it.ValueText)
		if it.ValueOn {
			valueStyled = onStyle.Render(it.ValueText)
		}
		rows = append(rows, " "+"  "+it.Label+strings.Repeat(" ", gap)+valueStyled+"  ")
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
