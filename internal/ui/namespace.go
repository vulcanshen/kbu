package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type NamespacePickerModel struct {
	namespaces  []string
	cursor      int
	animator    PopupAnimator
	theme       *theme.Theme
	searching   bool
	searchQuery string
}

func NewNamespacePickerModel(t *theme.Theme) NamespacePickerModel {
	return NamespacePickerModel{
		theme:    t,
		animator: NewPopupAnimator("namespace", lipgloss.Color("#74c7ec")),
	}
}

func (m *NamespacePickerModel) Open(namespaces []string) tea.Cmd {
	all := []string{"All Namespaces"}
	m.namespaces = append(all, namespaces...)
	m.cursor = 0
	m.searching = false
	m.searchQuery = ""
	return m.animator.Open()
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
	case "esc", "n":
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
	bc := lipgloss.Color("#74c7ec")
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
	if len(items) == 0 {
		lines = append(lines, normalStyle.Width(innerW).Render(" (no matches)"))
	} else {
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

	title := popupGlyph + " Namespaces"
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

	hint := " Enter: select  /: search  Esc: cancel "
	bottomDashes := innerW - lipgloss.Width(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}
