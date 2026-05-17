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
	lines := m.layoutLines()
	barStyle := m.theme.StatusLineStyle().Padding(0, 0)

	// Inline error only when status line is a single row (to keep multi-row clean).
	if len(lines) == 1 && unreadErrors > 0 && lastError != "" {
		hints := lines[0]
		errText := lastError
		hintsWidth := lipgloss.Width(hints)
		maxErrLen := m.width - hintsWidth - 4
		if maxErrLen > 10 {
			if len(errText) > maxErrLen {
				errText = errText[:maxErrLen-1] + "…"
			}
			leftPart := barStyle.Width(hintsWidth + 2).Render(hints)
			errPart := lipgloss.NewStyle().
				Foreground(lipgloss.Color(m.theme.Status.Error)).
				Width(m.width - hintsWidth - 2).
				Render(" " + errText)
			return leftPart + errPart
		}
	}

	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		styled = append(styled, barStyle.Width(m.width).Render(line))
	}
	return strings.Join(styled, "\n")
}
