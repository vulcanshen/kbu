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
)

type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
}

func (e LogEntry) String() string {
	ts := e.Time.Format("15:04:05")
	var prefix string
	switch e.Level {
	case LogInfo:
		prefix = "INFO"
	case LogWarn:
		prefix = "WARN"
	case LogError:
		prefix = "ERR "
	}
	return fmt.Sprintf("%s [%s] %s", ts, prefix, e.Message)
}

type AppLogModel struct {
	entries        []LogEntry
	maxEntries     int
	active         bool
	scrollOffset   int
	width          int
	height         int
	theme          *theme.Theme
	errorCount     int
	seenErrorCount int
	lastError      string
}

func NewAppLogModel(t *theme.Theme) AppLogModel {
	return AppLogModel{
		maxEntries: 500,
		theme:      t,
	}
}

func (m *AppLogModel) Add(level LogLevel, msg string) {
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
	}
}

func (m *AppLogModel) Info(msg string)  { m.Add(LogInfo, msg) }
func (m *AppLogModel) Warn(msg string)  { m.Add(LogWarn, msg) }
func (m *AppLogModel) Error(msg string) { m.Add(LogError, msg) }

func (m *AppLogModel) Toggle() {
	m.active = !m.active
	if m.active {
		m.scrollOffset = m.maxScrollOffset()
	}
	m.seenErrorCount = m.errorCount
	m.lastError = ""
}

func (m AppLogModel) IsActive() bool { return m.active }

func (m *AppLogModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m AppLogModel) UnreadErrorCount() int {
	return m.errorCount - m.seenErrorCount
}

func (m AppLogModel) LastErrorMessage() string {
	return m.lastError
}

func (m AppLogModel) Update(msg tea.Msg) (AppLogModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "!", "q":
			m.active = false
		case "j", "down":
			if m.scrollOffset < m.maxScrollOffset() {
				m.scrollOffset++
			}
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "G":
			m.scrollOffset = m.maxScrollOffset()
		case "g":
			m.scrollOffset = 0
		}
	}
	return m, nil
}

func (m AppLogModel) maxScrollOffset() int {
	contentH := m.height - 4
	max := len(m.entries) - contentH
	if max < 0 {
		return 0
	}
	return max
}

func (m AppLogModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Sidebar.CategoryFg)).
		Bold(true)
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Status.Error))
	warnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Status.Pending))
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.Detail.ValueFg))

	boxW := m.width - 4
	contentH := m.height - 4
	if contentH < 1 {
		contentH = 1
	}

	var lines []string
	lines = append(lines, titleStyle.Render(" App Log (! to close, j/k to scroll)"))
	lines = append(lines, "")

	if len(m.entries) == 0 {
		lines = append(lines, infoStyle.Render(" No log entries"))
	} else {
		end := m.scrollOffset + contentH
		if end > len(m.entries) {
			end = len(m.entries)
		}
		start := m.scrollOffset
		if start < 0 {
			start = 0
		}
		for i := start; i < end; i++ {
			e := m.entries[i]
			var s lipgloss.Style
			switch e.Level {
			case LogError:
				s = errorStyle
			case LogWarn:
				s = warnStyle
			default:
				s = infoStyle
			}
			line := e.String()
			if len(line) > boxW-2 {
				line = line[:boxW-3] + "…"
			}
			lines = append(lines, s.Render(" "+line))
		}
	}

	content := strings.Join(lines, "\n")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Sidebar.CategoryFg)).
		Width(boxW).
		Height(m.height - 2).
		Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay)
}
