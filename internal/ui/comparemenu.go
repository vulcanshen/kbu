package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"github.com/vulcanshen/km8/internal/theme"
)

// CompareMenuActionResult signals what the user did with the in-popup
// compare menu on the last Update call.
type CompareMenuActionResult int

const (
	CompareMenuActionNone CompareMenuActionResult = iota
	CompareMenuActionCancel
	CompareMenuActionCommit
)

// CompareMenuPopupModel is the Space-triggered diff-options menu that
// lives inside CompareYamlPopupModel (Switch view / Close). Extracted
// to its own file + own PopupAnimator so it falls under the regular
// popup-audit pattern — see .claude/rules/popup-convention.md. The
// parent computes the item labels (they depend on parent layout
// state), hands them to Open, and reads Cursor() on Commit.
type CompareMenuPopupModel struct {
	animator PopupAnimator
	items    []string
	cursor   int
	theme    *theme.Theme
}

func NewCompareMenuPopupModel(t *theme.Theme) CompareMenuPopupModel {
	return CompareMenuPopupModel{
		animator: NewPopupAnimator("comparepopup_menu", lipgloss.Color(theme.Periwinkle)),
		theme:    t,
	}
}

// Open stamps the current items + begins the open animation.
func (m *CompareMenuPopupModel) Open(items []string) tea.Cmd {
	m.items = items
	m.cursor = 0
	return m.animator.Open()
}

func (m *CompareMenuPopupModel) Close() tea.Cmd     { return m.animator.Close() }
func (m CompareMenuPopupModel) IsActive() bool      { return m.animator.IsActive() }
func (m CompareMenuPopupModel) IsInteractive() bool { return m.animator.IsInteractive() }
func (m CompareMenuPopupModel) Cursor() int         { return m.cursor }

// Reset forces the menu to PopupClosed without playing a close
// animation. Called by the parent's Open() so a previous compare's
// menu state can't leak into the new one.
func (m *CompareMenuPopupModel) Reset() {
	m.animator.State = PopupClosed
	m.animator.Frame = 0
	m.cursor = 0
}

func (m *CompareMenuPopupModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

// Update routes the key. Returns the new model, the action result, and
// a tea.Cmd. The parent inspects the result and reads Cursor() on
// Commit to decide which item was picked.
func (m CompareMenuPopupModel) Update(keyMsg tea.KeyMsg) (CompareMenuPopupModel, CompareMenuActionResult, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, CompareMenuActionNone, nil
	}
	switch keyMsg.String() {
	case "esc", " ":
		return m, CompareMenuActionCancel, m.animator.Close()
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
		return m, CompareMenuActionNone, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, CompareMenuActionNone, nil
	case "enter":
		return m, CompareMenuActionCommit, m.animator.Close()
	}
	return m, CompareMenuActionNone, nil
}

// Render overlays the menu on top of frame and returns the composed
// view. No-op (returns frame unchanged) when the menu is PopupClosed.
func (m CompareMenuPopupModel) Render(frame string) string {
	if !m.animator.IsActive() || len(m.items) == 0 {
		return frame
	}
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Periwinkle))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Periwinkle)).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#1e1e2e")).
		Background(lipgloss.Color(theme.Periwinkle)).Bold(true)
	rowStyle := lipgloss.NewStyle()
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	// Same icon as the parent compare-popup title so the overlay menu
	// reads as "this is the compare popup's menu" — same family, not a
	// new popup with its own identity.
	title := " \U000f08aa Diff "
	hint := " enter: select  esc: cancel "
	hintW := lipgloss.Width(hint)
	titleW := lipgloss.Width(title)
	innerW := hintW
	if w := titleW + 3; w > innerW {
		innerW = w
	}
	for _, it := range m.items {
		w := lipgloss.Width(it) + 2
		if w > innerW {
			innerW = w
		}
	}

	leadDashCount := 2
	trailDashCount := innerW - leadDashCount - titleW
	if trailDashCount < 1 {
		trailDashCount = 1
	}
	top := borderStyle.Render("╭"+strings.Repeat("─", leadDashCount)) +
		titleStyle.Render(title) +
		borderStyle.Render(strings.Repeat("─", trailDashCount)+"╮")
	rows := []string{top}
	for i, it := range m.items {
		text := " " + it + strings.Repeat(" ", innerW-1-lipgloss.Width(it))
		if i == m.cursor {
			text = cursorStyle.Render(text)
		} else {
			text = rowStyle.Render(text)
		}
		rows = append(rows, borderStyle.Render("│")+text+borderStyle.Render("│"))
	}
	tail := innerW - hintW
	if tail < 0 {
		tail = 0
	}
	rows = append(rows, borderStyle.Render("╰")+hintStyle.Render(hint)+
		borderStyle.Render(strings.Repeat("─", tail)+"╯"))

	menuBlock := strings.Join(rows, "\n")
	return overlay.Composite(m.animator.RenderFrame(menuBlock), frame, overlay.Center, overlay.Center, 0, 0)
}
