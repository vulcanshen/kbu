package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

// ContextPickerModel is an overlay that lets the user switch kubeconfig contexts.
type ContextPickerModel struct {
	contexts    []string
	current     string
	cursor      int
	animator    PopupAnimator
	theme       *theme.Theme
	searching   bool
	searchQuery string
}

// NewContextPickerModel creates a new context picker.
func NewContextPickerModel(t *theme.Theme) ContextPickerModel {
	return ContextPickerModel{
		theme:    t,
		animator: NewPopupAnimator("context", lipgloss.Color("#74c7ec")),
	}
}

// Open populates the picker with available contexts and sets the cursor to
// the currently active context.
func (m *ContextPickerModel) Open(contexts []string, current string) tea.Cmd {
	m.contexts = contexts
	m.current = current
	m.cursor = 0
	m.searching = false
	m.searchQuery = ""
	for i, c := range contexts {
		if c == current {
			m.cursor = i
			break
		}
	}
	return m.animator.Open()
}

func (m ContextPickerModel) filtered() []string {
	if m.searchQuery == "" {
		return m.contexts
	}
	q := strings.ToLower(m.searchQuery)
	var out []string
	for _, c := range m.contexts {
		if strings.Contains(strings.ToLower(c), q) {
			out = append(out, c)
		}
	}
	return out
}

// Close hides the picker.
func (m *ContextPickerModel) Close() tea.Cmd {
	return m.animator.Close()
}

// IsActive returns whether the picker overlay is shown (including animations).
func (m *ContextPickerModel) IsActive() bool      { return m.animator.IsActive() }
func (m *ContextPickerModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *ContextPickerModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// Update handles keyboard input for the context picker.
func (m ContextPickerModel) Update(msg tea.Msg) (ContextPickerModel, tea.Cmd) {
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
	items := m.filtered()
	switch keyMsg.String() {
	case "/":
		m.searching = true
		m.searchQuery = ""
		m.cursor = 0
		return m, nil
	case "j", "down":
		if m.cursor < len(items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		return m.selectCurrent(items)
	case "esc", "c", "C", " ":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.cursor = 0
			return m, nil
		}
		return m, m.animator.Close()
	}
	return m, nil
}

func (m ContextPickerModel) handleSearchKey(msg tea.KeyMsg) (ContextPickerModel, tea.Cmd) {
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
		if m.cursor < len(items)-1 {
			m.cursor++
		}
		return m, nil
	case msg.Type == tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
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

func (m ContextPickerModel) selectCurrent(items []string) (ContextPickerModel, tea.Cmd) {
	if len(items) == 0 || m.cursor >= len(items) {
		return m, nil
	}
	selected := items[m.cursor]
	closeCmd := m.animator.Close()
	return m, tea.Batch(closeCmd, func() tea.Msg {
		return ContextChangedMsg{Context: selected}
	})
}

// View renders the context picker (no-op; rendering via RenderPopup + overlay).
func (m ContextPickerModel) View() string {
	return ""
}

// RenderPopup returns the context picker box for use with overlay.Composite.
func (m ContextPickerModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m ContextPickerModel) renderFullPopup() string {
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	selectedStyle := m.theme.SidebarSelectedStyle()
	normalStyle := m.theme.SidebarStyle()

	boxWidth := 54
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
	if len(items) == 0 {
		lines = append(lines, normalStyle.Width(innerW).Render(" (no matches)"))
	} else {
		for i := start; i < end; i++ {
			marker := "  "
			if items[i] == m.current {
				marker = "* "
			}
			label := marker + items[i]
			if i == m.cursor {
				lines = append(lines, selectedStyle.Width(innerW).Render(label))
			} else {
				lines = append(lines, normalStyle.Width(innerW).Render(label))
			}
		}
	}
	body := strings.Join(lines, "\n")

	title := " Contexts"
	dashesAfter := innerW - 1 - lipgloss.Width(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")

	bodyLines := []string{""}
	if m.searching || m.searchQuery != "" {
		bodyLines = append(bodyLines, strings.Split(renderSearchBox(m.searchQuery, m.searching, innerW, m.theme), "\n")...)
	}
	bodyLines = append(bodyLines, strings.Split(body, "\n")...)
	bodyLines = append(bodyLines, "")

	for _, line := range bodyLines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(leftBorder + line + pad + rightBorder + "\n")
	}

	hint := " Enter: select  /: search  Space: cancel "
	bottomDashes := innerW - lipgloss.Width(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}
