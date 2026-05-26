package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/km8/internal/theme"
)

// Status line is now strictly one row. The previous dynamic 1–2 row layout
// made vertical math messy and the bottom of the screen jitter when the
// hint set changed.
const statusLineRows = 1

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

// hints returns the keys surfaced on the status line. v1.5.x mental model:
// only the universal navigation keys live here — anything else is reachable
// via Space (per-row context menu, popup self-documents) or `?` (full
// reference). Trigger letters (E/S/D/Y) live in the per-row Space menu, so
// they're not duplicated here.
//
// `/` filter only renders on panel 1/2 — panel 3 has no in-panel search
// (retired in v1.5.0). Hiding it on panel 3 avoids the always-on hint
// misleading users into trying.
func (m StatusLineModel) hints() []hint {
	h := []hint{
		{"?", "help"},
		{"q", "quit"},
		{"N", "ns"},
		{"C", "ctx"},
		{"space", "menu"},
		{"enter", "into"},
		{"Alt-t", "KM8erm"},
	}
	if m.activePanel != DetailPanel {
		h = append(h, hint{"/", "filter"})
	}
	return h
}

func (m StatusLineModel) renderedHints() []string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.StatusLine.Foreground)).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6c7086"))

	var out []string
	for _, h := range m.hints() {
		out = append(out, keyStyle.Render(h.key)+" "+descStyle.Render(h.desc))
	}
	return out
}

// layoutLine packs all hints into a single row. If the total width exceeds
// the terminal, trailing hints are dropped (silent truncation — they're
// still in the help popup).
func (m StatusLineModel) layoutLine() string {
	hints := m.renderedHints()
	if len(hints) == 0 {
		return " "
	}

	var b strings.Builder
	b.WriteString(" ")
	current := 1
	for i, h := range hints {
		sep := "  "
		sepW := 2
		if i == 0 {
			sep = ""
			sepW = 0
		}
		hw := lipgloss.Width(h)
		if m.width > 0 && current+sepW+hw+1 > m.width {
			break
		}
		b.WriteString(sep)
		b.WriteString(h)
		current += sepW + hw
	}
	return b.String()
}

// LineCount returns the fixed status line height. Kept as a method so
// callers using m.statusLine.LineCount() in vertical math still work.
func (m StatusLineModel) LineCount() int { return statusLineRows }

func (m StatusLineModel) View() string {
	return m.ViewWithError(0, "")
}

func (m StatusLineModel) ViewWithError(unreadErrors int, lastError string) string {
	return m.ViewWithNotice(unreadErrors, lastError, "")
}

func (m StatusLineModel) ViewWithNotice(unreadErrors int, lastError, lastSuccess string) string {
	line := m.layoutLine()
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

	if noticeText == "" || m.width <= 0 {
		return barStyle.Width(m.width).Render(line)
	}

	lineW := lipgloss.Width(line)
	maxLen := m.width - lineW - 4
	if maxLen < 10 {
		// Not enough room for the notice — drop it (the App Log popup
		// still has the full text).
		return barStyle.Width(m.width).Render(line)
	}
	text := noticeText
	if lipgloss.Width(text) > maxLen {
		text = ansi.Truncate(text, maxLen-1, "") + "…"
	}
	leftPart := barStyle.Width(lineW + 2).Render(line)
	noticePart := lipgloss.NewStyle().
		Foreground(lipgloss.Color(noticeColor)).
		Width(m.width - lineW - 2).
		Render(" " + text)
	return leftPart + noticePart
}
