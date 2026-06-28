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

// hints returns the keys surfaced on the status line. v1.7+ mental model:
// only the universal cross-panel gestures live here — `?` for the full
// reference, `Esc` / `Space` / `Enter` / `Tab` as the four core gestures
// (per the popup-design mindset memo), plus the global Alterm toggle.
//
// Everything panel-specific (N namespace, C context, / filter, trigger
// letters Y/E/S/D, sort hotkeys, ...) lives in the statusbar labels
// (`[C]ontext:` / `[N]amespace:`) or the per-row Space menus / popups
// that self-document — duplicating them here was noisy.
func (m StatusLineModel) hints() []hint {
	return []hint{
		{"?", "help"},
		{"Esc", "exit"},
		{"Space", "menu"},
		{"Enter", "commit/into"},
		{"Tab", "cycle panel"},
		{"Alt-t", "Alterm"},
		{">", "settings"},
	}
}

func (m StatusLineModel) renderedHints() []string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.StatusLine.Foreground)).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7f849c"))

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
	return m.ViewWithNotice(unreadErrors, 0, lastError, "", "")
}

// ViewWithNotice renders the status line with an optional right-side
// notice. Precedence: error (red lastError) > warn (peach lastWarn) >
// success (green lastSuccess) > nothing. The warn slot mirrors the
// status bar's peach badge so the two surfaces read consistently.
func (m StatusLineModel) ViewWithNotice(unreadErrors, unreadWarns int, lastError, lastWarn, lastSuccess string) string {
	line := m.layoutLine()
	barStyle := m.theme.StatusLineStyle().Padding(0, 0)

	noticeText := ""
	noticeColor := ""
	switch {
	case unreadErrors > 0 && lastError != "":
		noticeText = lastError
		noticeColor = m.theme.Status.Error
	case unreadWarns > 0 && lastWarn != "":
		noticeText = lastWarn
		noticeColor = toastWarnColor
	case lastSuccess != "":
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
