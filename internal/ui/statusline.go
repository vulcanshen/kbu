package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/kbu/internal/theme"
)

// Status line is now strictly one row. The previous dynamic 1–2 row layout
// made vertical math messy and the bottom of the screen jitter when the
// hint set changed.
const statusLineRows = 1

type StatusLineModel struct {
	activePanel Panel
	drillDown   bool
	// dragMode: the pinned-kind drag is on (a mode, tdp K11) — the
	// footer lists its keys instead of the panel entry keys.
	dragMode bool
	width    int
	theme    *theme.Theme
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

func (m *StatusLineModel) SetDragMode(on bool) {
	m.dragMode = on
}

func (m *StatusLineModel) SetWidth(width int) {
	m.width = width
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
func (m StatusLineModel) hints() []keyHint {
	if m.dragMode {
		// tdp K11, M1: in a mode the footer still shows ?, then the
		// mode's own keys. Space does nothing in a mode, so it isn't
		// listed. The mode names itself on panel 1's frame (K11: Drag top
		// right, the frame Yellow), not here: the footer holds keys only
		// (M5).
		return []keyHint{
			{"?", "keys"},
			{"j/k", "move"},
			{"Enter", "drop"},
			{"Esc", "cancel"},
		}
	}
	return []keyHint{
		{"?", "help"},
		{"Esc", "back"},
		{"Space", "menu"},
		{"Enter", "commit/into"},
		{"Tab/1–3", "panels"}, // tdp D1
		{"Alt-t", "Alterm"},
		{">", "settings"},
	}
}

// layoutLine packs the hints into a single row, key:description one space
// apart (tdp M5). If the total width exceeds the terminal, trailing
// entries are dropped whole (D1) — they're still in the key reference.
// The key keeps the theme's footer colour; the colon and description are
// Overlay0 (D2).
func (m StatusLineModel) layoutLine() string {
	hints := m.hints()
	if m.width > 0 {
		hints = fitHints(hints, m.width-2) // a space at either end
	}
	if len(hints) == 0 {
		return " "
	}
	colours := brightHint()
	colours.key = lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.StatusLine.Foreground)).
		Bold(true)
	return " " + renderHints(hints, colours)
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
		return barStyle.Render(padDisp(line, m.width))
	}

	lineW := dispWidth(line)
	maxLen := m.width - lineW - 4
	if maxLen < 10 {
		// Not enough room for the notice — drop it (the App Log popup
		// still has the full text).
		return barStyle.Render(padDisp(line, m.width))
	}
	text := noticeText
	if dispWidth(text) > maxLen {
		text = dispClip(text, maxLen-1) + "…"
	}
	leftPart := barStyle.Render(padDisp(line, lineW+2))
	noticePart := lipgloss.NewStyle().
		Foreground(lipgloss.Color(noticeColor)).
		Render(padDisp(" "+text, m.width-lineW-2))
	return leftPart + noticePart
}
