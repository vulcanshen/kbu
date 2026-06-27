package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/theme"
)

type LogLevel int

const (
	LogInfo LogLevel = iota
	LogWarn
	LogError
	LogSuccess
)

type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
}

func (e LogEntry) FormatPrefix() (string, string) {
	ts := e.Time.Format("15:04:05")
	switch e.Level {
	case LogWarn:
		return ts, "WARN"
	case LogError:
		return ts, "ERR"
	case LogSuccess:
		return ts, "OK"
	default:
		return ts, "INFO"
	}
}

type AppLogModel struct {
	entries        []LogEntry
	maxEntries     int
	animator       PopupAnimator
	scrollOffset   int
	width          int
	height         int
	theme          *theme.Theme
	errorCount     int
	seenErrorCount int
	lastError      string
	lastSuccess    string
	layer          int
	borderColor    lipgloss.Color
}

func NewAppLogModel(t *theme.Theme) AppLogModel {
	bc := theme.PopupLayerColor(1)
	return AppLogModel{
		maxEntries:  1000,
		theme:       t,
		animator:    NewPopupAnimator("applog", bc),
		borderColor: bc,
		layer:       1,
	}
}

// SetLayer stamps the popup's nesting depth and derives + stores the
// layer color. Animator stroke color updates in lockstep so the open/
// close lines match the final border. Call this before Open().
func (m *AppLogModel) SetLayer(layer int) {
	m.layer = layer
	m.borderColor = theme.PopupLayerColor(layer)
	m.animator.Color = m.borderColor
}

const maxEntryChars = 300

func (m *AppLogModel) Add(level LogLevel, msg string) {
	runes := []rune(msg)
	if len(runes) > maxEntryChars {
		msg = string(runes[:maxEntryChars]) + "…"
	}
	m.entries = append(m.entries, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
	})
	if len(m.entries) > m.maxEntries {
		m.entries = m.entries[len(m.entries)-m.maxEntries:]
	}
	if level == LogError || level == LogWarn {
		m.errorCount++
		m.lastError = msg
	} else if level == LogSuccess {
		m.lastSuccess = msg
	}
}

func (m *AppLogModel) Info(msg string)    { m.Add(LogInfo, msg) }
func (m *AppLogModel) Warn(msg string)    { m.Add(LogWarn, msg) }
func (m *AppLogModel) Error(msg string)   { m.Add(LogError, msg) }
func (m *AppLogModel) Success(msg string) { m.Add(LogSuccess, msg) }

func (m *AppLogModel) Toggle() tea.Cmd {
	if m.animator.IsActive() {
		return m.animator.Close()
	}
	m.scrollOffset = 0
	m.seenErrorCount = m.errorCount
	m.lastError = ""
	return m.animator.Open()
}

func (m AppLogModel) IsActive() bool      { return m.animator.IsActive() }
func (m AppLogModel) IsInteractive() bool { return m.animator.IsInteractive() }

func (m *AppLogModel) HandleTick(msg AnimTickMsg) tea.Cmd {
	if msg.Target != m.animator.Target {
		return nil
	}
	return m.animator.Tick()
}

func (m *AppLogModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m AppLogModel) LastSuccessMessage() string { return m.lastSuccess }

func (m AppLogModel) UnreadErrorCount() int {
	return m.errorCount - m.seenErrorCount
}

func (m AppLogModel) LastErrorMessage() string {
	return m.lastError
}

func (m AppLogModel) Update(msg tea.Msg) (AppLogModel, tea.Cmd) {
	if !m.animator.IsInteractive() {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "!", " ":
			return m, m.animator.Close()
		case "j", "down":
			if m.scrollOffset < m.maxScrollOffset() {
				m.scrollOffset++
			}
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "d":
			half := (m.height - 4) / 2
			if half < 1 {
				half = 1
			}
			m.scrollOffset += half
			if m.scrollOffset > m.maxScrollOffset() {
				m.scrollOffset = m.maxScrollOffset()
			}
		case "u":
			half := (m.height - 4) / 2
			if half < 1 {
				half = 1
			}
			m.scrollOffset -= half
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
		case "G":
			m.scrollOffset = m.maxScrollOffset()
		case "g":
			m.scrollOffset = 0
		case "D":
			m.entries = nil
			m.errorCount = 0
			m.seenErrorCount = 0
			m.lastError = ""
			m.scrollOffset = 0
		case "y":
			return m, copyToClipboardCmd(m.PlainText())
		}
	}
	return m, nil
}

// PlainText returns every log entry as a single string, one entry per
// line, newest first to match the on-screen order. Used by y-to-copy
// so the user can paste the full log into an issue / chat.
func (m AppLogModel) PlainText() string {
	if len(m.entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i := len(m.entries) - 1; i >= 0; i-- {
		e := m.entries[i]
		ts, level := e.FormatPrefix()
		b.WriteString(ts)
		b.WriteByte(' ')
		b.WriteString(level)
		b.WriteByte(' ')
		b.WriteString(e.Message)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m AppLogModel) popupHeight() int {
	h := m.height * 60 / 100
	if h < 10 {
		h = 10
	}
	return h
}

func (m AppLogModel) popupWidth() int {
	w := m.width * 70 / 100
	if w < 40 {
		w = 40
	}
	if w > m.width-4 {
		w = m.width - 4
	}
	return w
}

// renderAllLines renders every entry into display lines (newest first).
// scrollOffset is now line-based so this method is the single source of truth.
func (m AppLogModel) renderAllLines() []string {
	if len(m.entries) == 0 {
		return nil
	}
	innerW := m.popupWidth() - 2
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Error))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Pending))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Status.Running))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Detail.ValueFg))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	var all []string
	for i := len(m.entries) - 1; i >= 0; i-- {
		e := m.entries[i]
		ts, level := e.FormatPrefix()
		var levelStyle lipgloss.Style
		switch e.Level {
		case LogError:
			levelStyle = errorStyle
		case LogWarn:
			levelStyle = warnStyle
		case LogSuccess:
			levelStyle = successStyle
		default:
			levelStyle = infoStyle
		}
		prefix := dimStyle.Render(ts) + " " + levelStyle.Width(5).Render(level) + " "
		prefixW := lipgloss.Width(prefix)
		msgW := innerW - prefixW
		if msgW < 4 {
			msgW = 4
		}
		indent := strings.Repeat(" ", prefixW)
		remaining := []rune(e.Message)
		first := true
		for len(remaining) > 0 {
			w, cut := 0, 0
			for cut < len(remaining) {
				rw := lipgloss.Width(string(remaining[cut]))
				if w+rw > msgW {
					break
				}
				w += rw
				cut++
			}
			if cut == 0 {
				cut = 1
			}
			chunk := string(remaining[:cut])
			remaining = remaining[cut:]
			if first {
				all = append(all, prefix+levelStyle.Render(chunk))
				first = false
			} else {
				all = append(all, indent+levelStyle.Render(chunk))
			}
		}
	}
	return all
}

func (m AppLogModel) maxScrollOffset() int {
	contentH := m.popupHeight() - 2
	innerW := m.popupWidth() - 2
	prefixW := len("00:00:00 LEVEL ") // fixed-width prefix approximation
	msgW := innerW - prefixW
	if msgW < 4 {
		msgW = 4
	}
	total := 0
	for _, e := range m.entries {
		r := []rune(e.Message)
		if len(r) == 0 {
			total++
		} else {
			total += (len(r) + msgW - 1) / msgW
		}
	}
	max := total - contentH
	if max < 0 {
		return 0
	}
	return max
}

// HandleMouse routes a click against the app log viewer. Right-
// click inside the popup closes it (mirror of Esc / !).
// Left-click is no-op — log lines aren't selectable. Wheel scroll
// is handled at the AppModel layer (synthesizes u/d).
func (m AppLogModel) HandleMouse(msg tea.MouseMsg, screenW, screenH int) (AppLogModel, tea.Cmd) {
	if !m.animator.IsInteractive() || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if !popupContains(m.RenderPopup(), msg, screenW, screenH) {
		return m, nil
	}
	if msg.Button == tea.MouseButtonRight {
		return m, m.animator.Close()
	}
	return m, nil
}

func (m AppLogModel) View() string {
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		m.RenderPopup())
}

func (m AppLogModel) RenderPopup() string {
	return m.animator.RenderFrame(m.renderFullPopup())
}

func (m AppLogModel) renderFullPopup() string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	boxW := m.popupWidth()
	// Body content area = total height - 2 borders - 2 padRows.
	contentH := m.popupHeight() - 4
	innerW := boxW - 2

	var lines []string

	allLines := m.renderAllLines()
	if len(allLines) == 0 {
		lines = append(lines, dimStyle.Render(" No log entries"))
	} else {
		start := m.scrollOffset
		if start > len(allLines) {
			start = len(allLines)
		}
		end := start + contentH
		if end > len(allLines) {
			end = len(allLines)
		}
		lines = allLines[start:end]
	}

	body := strings.Join(lines, "\n")

	bc := m.borderColor
	bStyle := lipgloss.NewStyle().Foreground(bc)
	tStyle := lipgloss.NewStyle().Foreground(bc).Bold(true)

	title := " App Log"
	dashes := innerW - 1 - lipgloss.Width(title)
	if dashes < 0 {
		dashes = 0
	}

	var b strings.Builder
	b.WriteString(bStyle.Render("╭─"))
	b.WriteString(tStyle.Render(title))
	b.WriteString(bStyle.Render(strings.Repeat("─", dashes) + "╮"))
	b.WriteString("\n")

	left := bStyle.Render("│")
	right := bStyle.Render("│")
	padRow := left + strings.Repeat(" ", innerW) + right + "\n"
	contentLines := strings.Split(body, "\n")
	for len(contentLines) < contentH {
		contentLines = append(contentLines, "")
	}
	b.WriteString(padRow) // top padding row
	for _, line := range contentLines[:contentH] {
		lw := lipgloss.Width(line)
		if lw > innerW {
			line = ansiTruncate(line, innerW)
			lw = lipgloss.Width(line)
		}
		pad := ""
		if lw < innerW {
			pad = strings.Repeat(" ", innerW-lw)
		}
		if line == "" {
			b.WriteString(left + strings.Repeat(" ", innerW) + right)
		} else {
			b.WriteString(left + line + pad + right)
		}
		b.WriteString("\n")
	}
	b.WriteString(padRow) // bottom padding row
	hint := " Space:close j/k u/d y:copy D:clear "
	indicator := ""
	if totalLines := len(allLines); totalLines > 0 {
		indicator = fmt.Sprintf(" %d of %d ", m.scrollOffset+1, totalLines)
	}
	bottomDashes := innerW - len(hint) - len(indicator) - 1
	if bottomDashes < 0 {
		bottomDashes = 0
	}
	b.WriteString(bStyle.Render("╰─") + tStyle.Render(hint) + bStyle.Render(strings.Repeat("─", bottomDashes)+indicator+"╯"))

	return b.String()
}
