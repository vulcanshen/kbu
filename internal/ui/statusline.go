package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

const maxStatusLineRows = 2

type StatusLineModel struct {
	activePanel Panel
	drillDown   bool
	width       int
	theme       *theme.Theme
}

func NewStatusLineModel(t *theme.Theme) StatusLineModel {
	return StatusLineModel{
		activePanel: SidebarPanel,
		theme:       t,
	}
}

func (m *StatusLineModel) SetActivePanel(p Panel) {
	m.activePanel = p
}

func (m *StatusLineModel) SetDrillDown(d bool) {
	m.drillDown = d
}

func (m *StatusLineModel) SetWidth(width int) {
	m.width = width
}

type hint struct {
	key  string
	desc string
}

func (m StatusLineModel) hints() []hint {
	nav := []hint{
		{"j/k", "scroll"},
		{"u/d", "page"},
		{"gg/G", "top/bot"},
	}
	switch m.activePanel {
	case SidebarPanel:
		return append(nav,
			hint{"n", "ns"},
			hint{"c", "ctx"},
			hint{"e", "edit"},
		)
	case TablePanel:
		if m.drillDown {
			return append(nav,
				hint{"/", "search"},
				hint{"h/l", "tab"},
				hint{"s", "shell"},
				hint{"esc", "back"},
			)
		}
		return append(nav,
			hint{"/", "search"},
			hint{"h/l", "tab"},
			hint{"s", "shell"},
			hint{"e", "edit"},
			hint{"D", "delete"},
			hint{"+/-", "expand"},
		)
	case DetailPanel:
		return append(nav,
			hint{"/", "search"},
			hint{"h/l", "tab"},
			hint{"+/-", "expand"},
		)
	}
	return nil
}

type renderedHint struct {
	s string
	w int
}

func (m StatusLineModel) renderedHints() []renderedHint {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.StatusLine.Foreground)).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6c7086"))

	var out []renderedHint
	for _, h := range m.hints() {
		s := keyStyle.Render(h.key) + " " + descStyle.Render(h.desc)
		out = append(out, renderedHint{s: s, w: lipgloss.Width(s)})
	}
	return out
}

// layoutLines packs hints into up to maxStatusLineRows lines, dropping the
// trailing hints that do not fit.
func (m StatusLineModel) layoutLines() []string {
	hints := m.renderedHints()
	if m.width <= 0 {
		var b strings.Builder
		b.WriteString(" ")
		for i, h := range hints {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(h.s)
		}
		return []string{b.String()}
	}

	var lines []string
	var current strings.Builder
	currentW := 1
	current.WriteString(" ")

	for _, h := range hints {
		sep := "  "
		sepW := 2
		if currentW == 1 {
			sep = ""
			sepW = 0
		}
		if currentW+sepW+h.w+1 > m.width {
			if currentW > 1 {
				lines = append(lines, current.String())
				if len(lines) >= maxStatusLineRows {
					return lines
				}
				current.Reset()
				current.WriteString(" ")
				currentW = 1
				sep, sepW = "", 0
			}
			if currentW+sepW+h.w+1 > m.width {
				continue
			}
		}
		current.WriteString(sep)
		current.WriteString(h.s)
		currentW += sepW + h.w
	}
	if currentW > 1 {
		lines = append(lines, current.String())
	}
	if len(lines) == 0 {
		lines = append(lines, " ")
	}
	return lines
}

// LineCount returns how many rows the status line will occupy.
func (m StatusLineModel) LineCount() int {
	return len(m.layoutLines())
}

func (m StatusLineModel) View() string {
	return m.ViewWithError(0, "")
}

func (m StatusLineModel) ViewWithError(unreadErrors int, lastError string) string {
	return m.ViewWithNotice(unreadErrors, lastError, "")
}

func (m StatusLineModel) ViewWithNotice(unreadErrors int, lastError, lastSuccess string) string {
	lines := m.layoutLines()
	barStyle := m.theme.StatusLineStyle().Padding(0, 0)

	// Error takes priority over success.
	noticeText := ""
	noticeColor := ""
	if unreadErrors > 0 && lastError != "" {
		noticeText = lastError
		noticeColor = m.theme.Status.Error
	} else if lastSuccess != "" {
		noticeText = lastSuccess
		noticeColor = m.theme.Status.Running
	}

	styled := make([]string, 0, len(lines))
	for i, line := range lines {
		isLast := i == len(lines)-1
		if isLast && noticeText != "" {
			lineW := lipgloss.Width(line)
			maxLen := m.width - lineW - 4
			if maxLen > 10 {
				text := noticeText
				if lipgloss.Width(text) > maxLen {
					text = text[:maxLen-1] + "…"
				}
				leftPart := barStyle.Width(lineW + 2).Render(line)
				noticePart := lipgloss.NewStyle().
					Foreground(lipgloss.Color(noticeColor)).
					Width(m.width - lineW - 2).
					Render(" " + text)
				styled = append(styled, leftPart+noticePart)
				continue
			}
		}
		styled = append(styled, barStyle.Width(m.width).Render(line))
	}
	return strings.Join(styled, "\n")
}
