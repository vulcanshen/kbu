package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// BreadcrumbPopupModel lists the Links-tab drill chain and lets the user
// jump back to any ancestor level via j/k + Enter. Opened by `i` on the
// Links tab at depth > 1; closed by Esc / q / i.
//
// On Enter the model emits LinkJumpMsg{Level: 1-indexed level} so
// AppModel can call detail.JumpToDrillLevel — popping intermediate
// frames in one step.
type BreadcrumbPopupModel struct {
	animator PopupAnimator
	chain    []k8s.RefTarget
	cursor   int
	screenW  int
	theme    *theme.Theme
}

func NewBreadcrumbPopupModel(t *theme.Theme) BreadcrumbPopupModel {
	return BreadcrumbPopupModel{
		theme:    t,
		animator: NewPopupAnimator("breadcrumb", lipgloss.Color("#cba6f7")),
	}
}

// Open shows the popup with `chain` as the displayed levels (root first,
// current top last). Cursor lands on the current level so Enter is a
// no-op by default — user must move first to commit a jump.
func (m *BreadcrumbPopupModel) Open(chain []k8s.RefTarget) tea.Cmd {
	m.chain = append([]k8s.RefTarget(nil), chain...)
	if len(m.chain) > 0 {
		m.cursor = len(m.chain) - 1
	} else {
		m.cursor = 0
	}
	return m.animator.Open()
}

func (m *BreadcrumbPopupModel) Close() tea.Cmd { return m.animator.Close() }

func (m *BreadcrumbPopupModel) SetSize(w, _ int) { m.screenW = w }

func (m BreadcrumbPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m BreadcrumbPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *BreadcrumbPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m BreadcrumbPopupModel) Update(msg tea.Msg) (BreadcrumbPopupModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.chain)-1 {
				m.cursor++
			}
			return m, nil
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "g":
			m.cursor = 0
			return m, nil
		case "G":
			m.cursor = len(m.chain) - 1
			return m, nil
		case "enter":
			level := m.cursor + 1
			closeCmd := m.animator.Close()
			jumpCmd := func() tea.Msg { return LinkJumpMsg{Level: level} }
			return m, tea.Batch(closeCmd, jumpCmd)
		case "esc", "q", "i":
			return m, m.animator.Close()
		}
	}
	return m, nil
}

func (m BreadcrumbPopupModel) View() string { return "" }

func (m BreadcrumbPopupModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m BreadcrumbPopupModel) renderFullPopup() string {
	bc := lipgloss.Color("#cba6f7")
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)
	levelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).Background(bc).Bold(true)
	currentMarkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Pending)).Bold(true)

	title := "󰍒 Breadcrumb"
	hint := " j/k: move  Enter: jump  Esc/q/i: close "

	maxInnerW := 80
	if m.screenW > 0 {
		maxInnerW = m.screenW * 70 / 100
		if maxInnerW < 40 {
			maxInnerW = 40
		}
	}

	rows := make([]string, 0, len(m.chain))
	for i, ref := range m.chain {
		levelTag := fmt.Sprintf("%d.", i+1)
		label := refDisplay(ref)
		marker := "  "
		if i == len(m.chain)-1 {
			marker = currentMarkStyle.Render("● ")
		}
		row := " " + levelStyle.Render(levelTag) + " " + marker + label
		if i == m.cursor {
			row = cursorStyle.Render(" " + levelTag + " " + stripStyles(marker) + label + " ")
		}
		rows = append(rows, row)
	}

	innerW := lipgloss.Width(title) + 4
	for _, r := range rows {
		if w := lipgloss.Width(r) + 2; w > innerW {
			innerW = w
		}
	}
	if w := len(hint) + 4; w > innerW {
		innerW = w
	}
	if innerW > maxInnerW {
		innerW = maxInnerW
	}

	dashesAfter := innerW - 1 - lipgloss.Width(title)
	if dashesAfter < 0 {
		dashesAfter = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─") + tStyle.Render(title) + bStyle.Render(strings.Repeat("─", dashesAfter)+"╮") + "\n")

	left := bStyle.Render("│")
	right := bStyle.Render("│")
	rows = append([]string{""}, rows...)
	rows = append(rows, "")
	for _, line := range rows {
		lw := lipgloss.Width(line)
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		b.WriteString(left + line + pad + right + "\n")
	}

	bottomDashes := innerW - len(hint) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+"╯"))

	return b.String()
}

// refDisplay formats a RefTarget for the breadcrumb row. Cluster-scoped
// refs (empty namespace) drop the ns/ prefix.
func refDisplay(ref k8s.RefTarget) string {
	kind := string(ref.Type)
	if def := k8s.DefaultRegistry.Get(ref.Type); def != nil {
		kind = strings.TrimSuffix(def.DisplayName, "s")
	}
	if ref.Namespace == "" {
		return fmt.Sprintf("%s/%s", kind, ref.Name)
	}
	return fmt.Sprintf("%s/%s in %s", kind, ref.Name, ref.Namespace)
}

// stripStyles is a placeholder for "the marker rendered as plain text"
// when we need to measure its width inside a cursor-highlighted row.
// Currently the marker is always two cells wide regardless of style, so
// we just return two spaces.
func stripStyles(_ string) string { return "  " }
