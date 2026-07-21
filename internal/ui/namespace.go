package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/kbu/internal/theme"
)

type NamespacePickerModel struct {
	namespaces  []string
	cursor      int
	animator    PopupAnimator
	theme       *theme.Theme
	searching   bool
	searchQuery string

	// loading=true means the popup is open with a placeholder row
	// while fetchNamespaces is still in flight. Update gates out all
	// list-mutating keys in this state (j/k/Enter/search) so the
	// user can't act on an empty list. Flipped to false by
	// SetNamespaces once the real list arrives.
	loading      bool
	spinnerFrame int

	layer       int
	borderColor lipgloss.Color
}

// namespaceSpinnerTickMsg drives the braille-spinner cycle in the
// title slot while loading. Independent of PopupAnimator's
// AnimTickMsg because the spinner runs at its own ~80ms cadence and
// has no opening / expand state machine — it just cycles frames
// until loading=false.
type namespaceSpinnerTickMsg struct{}

// namespaceSpinnerInterval picks 80ms — fast enough to read as
// "alive / working", slow enough not to flicker. 10 frames @ 80ms =
// 800ms full cycle.
const namespaceSpinnerInterval = 80 * time.Millisecond

// namespaceSpinnerFrames is the standard 10-frame braille spinner
// (dots-cycle pattern). Each frame is a single Unicode codepoint in
// the Braille Patterns block (U+2800–U+28FF) — all single-cell wide
// in monospaced terminals, so the title slot's width stays constant
// across frames.
var namespaceSpinnerFrames = []string{
	"⠋", "⠙", "⠹", "⠸", "⠼",
	"⠴", "⠦", "⠧", "⠇", "⠏",
}

func NewNamespacePickerModel(t *theme.Theme) NamespacePickerModel {
	bc := theme.PopupLayerColor(1)
	return NamespacePickerModel{
		theme:       t,
		animator:    NewPopupAnimator("namespace", bc),
		borderColor: bc,
		layer:       1,
	}
}

// SetLayer stamps nesting depth + derives border / animator color.
func (m *NamespacePickerModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	m.animator.Color = m.borderColor
}

// OpenLoading opens the popup IMMEDIATELY in its loading state — no
// API call needed to show the frame. fetchNamespaces is fired in
// parallel by the caller; once NamespaceListMsg arrives,
// SetNamespaces swaps the placeholder for the real list in place
// (no re-animation, no flicker — the animator stays in
// PopupOpen/PopupOpeningExpand).
//
// Pre-existing direct Open(namespaces) call was removed because the
// caller would have had to do the fetch synchronously anyway —
// merging into the async path is the whole point of this change.
func (m *NamespacePickerModel) OpenLoading() tea.Cmd {
	m.namespaces = nil
	m.cursor = 0
	m.searching = false
	m.searchQuery = ""
	m.loading = true
	m.spinnerFrame = 0
	return tea.Batch(m.animator.Open(), m.spinnerTickCmd())
}

func (m NamespacePickerModel) spinnerTickCmd() tea.Cmd {
	return tea.Tick(namespaceSpinnerInterval, func(time.Time) tea.Msg {
		return namespaceSpinnerTickMsg{}
	})
}

// HandleSpinnerTick advances the spinner frame and schedules the next
// tick. Returns nil once loading flips false (SetNamespaces fired),
// terminating the spinner loop naturally without a separate stop msg.
func (m *NamespacePickerModel) HandleSpinnerTick(_ namespaceSpinnerTickMsg) tea.Cmd {
	if !m.loading {
		return nil
	}
	m.spinnerFrame = (m.spinnerFrame + 1) % len(namespaceSpinnerFrames)
	return m.spinnerTickCmd()
}

// SetNamespaces fills in the real list. Safe to call whether or not
// the popup is still open — if the user dismissed before the fetch
// returned, the state update is harmless (next OpenLoading resets
// it). Cursor lands on "All Namespaces" so Enter immediately is a
// sensible default.
func (m *NamespacePickerModel) SetNamespaces(namespaces []string) {
	m.loading = false
	all := []string{"All Namespaces"}
	m.namespaces = append(all, namespaces...)
	m.cursor = 0
}

func (m NamespacePickerModel) filtered() []string {
	if m.searchQuery == "" {
		return m.namespaces
	}
	q := strings.ToLower(m.searchQuery)
	var out []string
	for _, n := range m.namespaces {
		if strings.Contains(strings.ToLower(n), q) {
			out = append(out, n)
		}
	}
	return out
}

func (m *NamespacePickerModel) Close() tea.Cmd {
	return m.animator.Close()
}

func (m *NamespacePickerModel) IsActive() bool      { return m.animator.IsActive() }
func (m *NamespacePickerModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *NamespacePickerModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m NamespacePickerModel) Update(msg tea.Msg) (NamespacePickerModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.loading {
		// Loading state: only the close set responds — j/k/Enter on
		// an empty placeholder would either no-op or fire bogus
		// selections, so we just ignore them until the real list
		// lands.
		switch keyMsg.String() {
		case "esc", "n", "N", " ":
			return m, m.animator.Close()
		}
		return m, nil
	}
	if m.searching {
		return m.handleSearchKey(keyMsg)
	}
	items := m.filtered()
	switch keyMsg.String() {
	case "/":
		m.searching = true
		m.searchQuery = ""
		m.cursor = 0
		return m, nil
	case "j", "down":
		if len(items) > 0 {
			m.cursor = (m.cursor + 1) % len(items)
		}
	case "k", "up":
		if len(items) > 0 {
			m.cursor = (m.cursor - 1 + len(items)) % len(items)
		}
	case "enter":
		return m.selectCurrent(items)
	case "esc", "n", "N", " ":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.cursor = 0
			return m, nil
		}
		return m, m.animator.Close()
	}
	return m, nil
}

func (m NamespacePickerModel) handleSearchKey(msg tea.KeyMsg) (NamespacePickerModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.searching = false
		m.searchQuery = ""
		m.cursor = 0
		return m, nil
	case msg.Type == tea.KeyEnter:
		// Release search focus, keep filter. j/k navigation becomes available;
		// a second Enter then selects.
		m.searching = false
		return m, nil
	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.cursor = 0
		}
		return m, nil
	case msg.Type == tea.KeyDown:
		items := m.filtered()
		if len(items) > 0 {
			m.cursor = (m.cursor + 1) % len(items)
		}
		return m, nil
	case msg.Type == tea.KeyUp:
		items := m.filtered()
		if len(items) > 0 {
			m.cursor = (m.cursor - 1 + len(items)) % len(items)
		}
		return m, nil
	case msg.Type == tea.KeyRunes:
		for _, r := range msg.Runes {
			m.searchQuery += string(r)
		}
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

// HandleMouse routes a click against the picker. Left-click on a
// namespace row selects that namespace (mirror of cursor+Enter).
// Right-click closes the picker. Clicks during the loading state
// only respond to right-click (no rows to act on).
//
// The render shape adapts to whether the user has the search box
// open, which pushes the namespace rows down by 3 lines (search-box
// is itself a 3-line ╭─╮ block inside the popup). Scrolling matters
// too — the picker only renders a 10-item window into m.namespaces
// at a time, so a click on visible row N maps back to
// m.namespaces[start+N] where `start` is the same window-clamp the
// renderer uses.
func (m NamespacePickerModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (NamespacePickerModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if m.loading {
		if msg.Button == tea.MouseButtonRight {
			return m, m.animator.Close()
		}
		return m, nil
	}
	items := m.filtered()
	maxVisible := 10
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}
	numVisible := end - start

	itemsStartLine := 2
	if m.searching || m.searchQuery != "" {
		// renderSearchBox emits 3 lines (top + mid + bottom border).
		itemsStartLine += 3
	}
	row := popupRowAt(m.renderFullPopup(), msg, screenW, screenH, itemsStartLine, numVisible)
	if row < 0 {
		if msg.Button == tea.MouseButtonRight {
			return m, m.animator.Close()
		}
		return m, nil
	}
	realIdx := start + row
	if realIdx < 0 || realIdx >= len(items) {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		m.cursor = realIdx
		return m.selectCurrent(items)
	case tea.MouseButtonRight:
		return m, m.animator.Close()
	}
	return m, nil
}

func (m NamespacePickerModel) selectCurrent(items []string) (NamespacePickerModel, tea.Cmd) {
	if len(items) == 0 || m.cursor >= len(items) {
		return m, nil
	}
	ns := ""
	if items[m.cursor] != "All Namespaces" {
		ns = items[m.cursor]
	}
	closeCmd := m.animator.Close()
	return m, tea.Batch(closeCmd, func() tea.Msg {
		return NamespaceChangedMsg{Namespace: ns}
	})
}

func (m NamespacePickerModel) View() string {
	return ""
}

func (m NamespacePickerModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m NamespacePickerModel) renderFullPopup() string {
	bc := m.borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	selectedStyle := m.theme.SidebarSelectedStyle()
	normalStyle := m.theme.SidebarStyle()

	boxWidth := 44
	innerW := boxWidth - 2

	items := m.filtered()

	maxVisible := 10
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

	var lines []string
	switch {
	case m.loading:
		// Loading state: spinner in title carries the signal; body
		// shows a single empty row so the popup has visible interior
		// instead of collapsing to top + padRows + bottom. When data
		// arrives the empty row gets replaced by items.
		lines = append(lines, normalStyle.Width(innerW).Render(""))
	case len(items) == 0:
		lines = append(lines, normalStyle.Width(innerW).Render(" (no matches)"))
	default:
		for i := start; i < end; i++ {
			label := " " + items[i]
			if i == m.cursor {
				lines = append(lines, selectedStyle.Width(innerW).Render(label))
			} else {
				lines = append(lines, normalStyle.Width(innerW).Render(label))
			}
		}
	}
	body := strings.Join(lines, "\n")

	// Title reserves a fixed-width spinner slot so trailing dashes
	// stay constant across loading↔loaded. Loaded: slot is a single
	// space. Loading: slot carries one frame of the braille spinner.
	// All braille spinner chars are 1-cell wide → lipgloss.Width(title)
	// never changes → no border shake.
	spinner := " "
	if m.loading {
		spinner = namespaceSpinnerFrames[m.spinnerFrame]
	}
	title := " Namespaces " + spinner
	dashesAfter := innerW - 1 - lipgloss.Width(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right + "\n"

	var contentLines []string
	if m.searching || m.searchQuery != "" {
		contentLines = append(contentLines, strings.Split(renderSearchBox(m.searchQuery, m.searching, innerW, m.theme), "\n")...)
	}
	contentLines = append(contentLines, strings.Split(body, "\n")...)

	b.WriteString(padRow) // top padding row
	for _, line := range contentLines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(left + line + pad + right + "\n")
	}
	b.WriteString(padRow) // bottom padding row

	hint := " Enter: select  /: search  Space: cancel "
	bottomDashes := innerW - lipgloss.Width(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}
