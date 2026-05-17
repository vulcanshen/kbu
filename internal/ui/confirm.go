package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type ConfirmAction int

const (
	ConfirmShellExec ConfirmAction = iota
	ConfirmDelete
)

type ConfirmModel struct {
	animator  PopupAnimator
	action    ConfirmAction
	message   string
	detail    string
	screenW   int
	theme     *theme.Theme
	onConfirm tea.Cmd
}

func (m *ConfirmModel) SetSize(w, h int) {
	m.screenW = w
}

func NewConfirmModel(t *theme.Theme) ConfirmModel {
	return ConfirmModel{
		theme:    t,
		animator: NewPopupAnimator("confirm", lipgloss.Color("#74c7ec")),
	}
}

func (m *ConfirmModel) Show(action ConfirmAction, message, detail string, onConfirm tea.Cmd) tea.Cmd {
	m.action = action
	m.message = message
	m.detail = detail
	m.onConfirm = onConfirm
	return m.animator.Open()
}

func (m *ConfirmModel) Close() tea.Cmd {
	m.onConfirm = nil
	return m.animator.Close()
}

func (m ConfirmModel) IsActive() bool      { return m.animator.IsActive() }
func (m ConfirmModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *ConfirmModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y":
			cmd := m.onConfirm
			m.onConfirm = nil
			closeCmd := m.animator.Close()
			return m, tea.Batch(cmd, closeCmd)
		case "esc", "n", "q":
			m.onConfirm = nil
			return m, m.animator.Close()
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	return ""
}

func (m ConfirmModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m ConfirmModel) renderFullPopup() string {
	bc := lipgloss.Color("#74c7ec")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	msgStyle := lipgloss.NewStyle().Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Pending))

	title := "Confirm"
	hint := " Enter/y: confirm  Esc/n: cancel "

	// Cap inner width at 70% of screen (or 80 chars if no screen size).
	maxInnerW := 80
	if m.screenW > 0 {
		maxInnerW = m.screenW * 70 / 100
		if maxInnerW < 40 {
			maxInnerW = 40
		}
	}

	// Start from content; reserve 2 chars for left/right inner padding.
	innerW := 40
	for _, s := range []string{m.message, m.detail} {
		if w := lipgloss.Width(s) + 2; w > innerW {
			innerW = w
		}
	}
	if w := len(title) + 4; w > innerW {
		innerW = w
	}
	if w := len(hint) + 4; w > innerW {
		innerW = w
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	contentW := innerW - 2 // leading + trailing padding
	var lines []string
	for _, l := range wrapWords(m.message, contentW) {
		lines = append(lines, msgStyle.Render(" "+l))
	}
	if m.detail != "" {
		lines = append(lines, "")
		for _, l := range wrapWords(m.detail, contentW) {
			lines = append(lines, detailStyle.Render(" "+l))
		}
	}
	body := strings.Join(lines, "\n")

	dashesAfter := innerW - 1 - len(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	bodyLines := append([]string{""}, strings.Split(body, "\n")...)
	bodyLines = append(bodyLines, "")
	for _, line := range bodyLines {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(leftBorder + line + pad + rightBorder + "\n")
	}

	bottomDashes := innerW - len(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}
