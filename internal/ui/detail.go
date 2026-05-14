package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// DetailTab identifies which tab is active in the detail panel.
type DetailTab int

const (
	DetailTabInfo DetailTab = iota
	DetailTabEvents
	DetailTabLogs
)

// DetailModel is the Bubble Tea model for the detail panel.
type DetailModel struct {
	activeTab    DetailTab
	tabs         []string
	detail       k8s.ResourceDetail
	events       []k8s.EventItem
	scrollOffset int
	contentLines []string // pre-rendered content lines for current tab
	focused      bool
	width        int
	height       int
	theme        *theme.Theme
	hasData      bool
	pendingG     bool
	logLines     []string
	maxLogLines  int
	resourceType k8s.ResourceType
	searching    bool
	searchQuery  string
}

// IsSearching returns true if the detail panel is in search mode.
func (m DetailModel) IsSearching() bool { return m.searching }

// HasActiveFilter returns true if a search filter is active.
func (m DetailModel) HasActiveFilter() bool { return m.searchQuery != "" }

// NewDetailModel creates a new detail model with no data and the Detail tab active.
func NewDetailModel(t *theme.Theme) DetailModel {
	return DetailModel{
		activeTab:   DetailTabInfo,
		tabs:        []string{"Detail", "Events"},
		theme:       t,
		maxLogLines: 1000,
	}
}

// Init implements tea.Model.
func (m DetailModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ResourceDetailMsg:
		m.SetDetail(msg.Detail, msg.Events)
		return m, nil

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		return m.handleKey(msg)

	case tea.MouseMsg:
		if !m.focused {
			return m, nil
		}
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m DetailModel) handleKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	if m.searching {
		return m.handleSearchKey(msg)
	}

	if m.pendingG {
		m.pendingG = false
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'g' {
			m.scrollOffset = 0
			return m, nil
		}
		// Not a second g — fall through to normal handling.
	}

	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, nil
		}
		switch msg.Runes[0] {
		case 'j':
			m = m.scrollDown()
		case 'k':
			m = m.scrollUp()
		case 'g':
			m.pendingG = true
		case 'G':
			m = m.scrollToBottom()
		case ']':
			m = m.nextTab()
		case '[':
			m = m.prevTab()
		case '/':
			m.searching = true
			m.searchQuery = ""
			return m, nil
		}

	case tea.KeyDown:
		m = m.scrollDown()
	case tea.KeyUp:
		m = m.scrollUp()
	case tea.KeyEscape:
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.scrollOffset = 0
			return m, nil
		}
	}

	return m, nil
}

func (m DetailModel) handleSearchKey(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.searching = false
		m.searchQuery = ""
		m.scrollOffset = 0
		return m, nil
	case msg.Type == tea.KeyEnter:
		m.searching = false
		return m, nil
	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.scrollOffset = 0
		}
		return m, nil
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		m = m.scrollDown()
		return m, nil
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		m = m.scrollUp()
		return m, nil
	case msg.Type == tea.KeyRunes:
		for _, r := range msg.Runes {
			m.searchQuery += string(r)
		}
		m.scrollOffset = 0
		return m, nil
	}
	return m, nil
}

func (m DetailModel) filteredContentLines() []string {
	if m.searchQuery == "" {
		return m.contentLines
	}
	query := strings.ToLower(m.searchQuery)
	var filtered []string
	for _, line := range m.contentLines {
		if strings.Contains(strings.ToLower(line), query) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func (m DetailModel) handleMouse(msg tea.MouseMsg) (DetailModel, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		m = m.scrollUp()
	case tea.MouseWheelDown:
		m = m.scrollDown()
	}
	return m, nil
}

func (m DetailModel) scrollDown() DetailModel {
	maxOffset := m.maxScrollOffset()
	if m.scrollOffset < maxOffset {
		m.scrollOffset++
	}
	return m
}

func (m DetailModel) scrollUp() DetailModel {
	if m.scrollOffset > 0 {
		m.scrollOffset--
	}
	return m
}

func (m DetailModel) scrollToBottom() DetailModel {
	m.scrollOffset = m.maxScrollOffset()
	return m
}

func (m DetailModel) maxScrollOffset() int {
	contentHeight := m.contentHeight()
	max := len(m.contentLines) - contentHeight
	if max < 0 {
		return 0
	}
	return max
}

// contentHeight returns the number of lines available for content.
func (m DetailModel) contentHeight() int {
	if m.height < 0 {
		return 0
	}
	return m.height
}

func (m DetailModel) switchToTab(tab DetailTab) DetailModel {
	if m.activeTab != tab {
		m.activeTab = tab
		m.scrollOffset = 0
		m.buildContentLines()
	}
	return m
}

func (m DetailModel) nextTab() DetailModel {
	next := (int(m.activeTab) + 1) % len(m.tabs)
	return m.switchToTab(DetailTab(next))
}

func (m DetailModel) prevTab() DetailModel {
	prev := (int(m.activeTab) - 1 + len(m.tabs)) % len(m.tabs)
	return m.switchToTab(DetailTab(prev))
}

// View implements tea.Model.
func (m DetailModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	var b strings.Builder

	contentHeight := m.contentHeight()
	if m.searching || m.searchQuery != "" {
		contentHeight -= 3
	}
	if contentHeight <= 0 {
		return ""
	}

	if m.searching || m.searchQuery != "" {
		b.WriteString(renderSearchBox(m.searchQuery, m.searching, m.width, m.theme))
		b.WriteString("\n")
	}

	if !m.hasData {
		b.WriteString(m.theme.DetailValueStyle().Render("  No resource selected"))
		return b.String()
	}

	displayLines := m.filteredContentLines()

	end := m.scrollOffset + contentHeight
	if end > len(displayLines) {
		end = len(displayLines)
	}

	var lines []string
	for i := m.scrollOffset; i < end; i++ {
		lines = append(lines, displayLines[i])
	}
	b.WriteString(strings.Join(lines, "\n"))

	return b.String()
}

// SetSize sets the dimensions of the detail panel.
func (m *DetailModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether the detail panel is focused.
func (m *DetailModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetDetail updates the detail data and rebuilds content lines.
func (m *DetailModel) SetDetail(detail k8s.ResourceDetail, events []k8s.EventItem) {
	m.detail = detail
	m.events = events
	m.hasData = true
	m.scrollOffset = 0
	m.buildContentLines()
}

// TabTitle returns the tab bar string for embedding in the panel border.
func (m DetailModel) TabTitle() string {
	var parts []string
	for i, tab := range m.tabs {
		if DetailTab(i) == m.activeTab {
			parts = append(parts, "["+tab+"]")
		} else {
			parts = append(parts, " "+tab+" ")
		}
	}
	return strings.Join(parts, "─")
}

// ClearDetail clears the detail data.
func (m *DetailModel) ClearDetail() {
	m.detail = k8s.ResourceDetail{}
	m.events = nil
	m.hasData = false
	m.scrollOffset = 0
	m.contentLines = nil
	m.logLines = nil
}

// SetResourceType sets the current resource type and adjusts available tabs.
func (m *DetailModel) SetResourceType(rt k8s.ResourceType) {
	m.resourceType = rt
	switch rt {
	case k8s.ResourcePods:
		m.tabs = []string{"Detail", "Logs", "Events"}
	case k8s.ResourceEvents:
		m.tabs = []string{"Detail"}
	default:
		m.tabs = []string{"Detail", "Events"}
	}
	m.activeTab = 0
	m.scrollOffset = 0
	m.buildContentLines()
}

// NextTab switches to the next tab.
func (m DetailModel) NextTab() DetailModel {
	return m.nextTab()
}

// PrevTab switches to the previous tab.
func (m DetailModel) PrevTab() DetailModel {
	return m.prevTab()
}

// ActiveTabName returns the name of the currently active tab.
func (m DetailModel) ActiveTabName() string {
	if int(m.activeTab) < len(m.tabs) {
		return m.tabs[m.activeTab]
	}
	return "Detail"
}

// AppendLogLine appends a formatted log line to the log buffer.
// If the buffer exceeds maxLogLines, the oldest lines are trimmed.
// If the Logs tab is active, content lines are rebuilt.
func (m *DetailModel) AppendLogLine(container, text string) {
	formatted := "  " + container + " │ " + text
	m.logLines = append(m.logLines, formatted)
	if len(m.logLines) > m.maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-m.maxLogLines:]
	}
	if m.ActiveTabName() == "Logs" {
		m.buildContentLines()
	}
}

// buildContentLines rebuilds the pre-rendered content lines for the current tab.
func (m *DetailModel) buildContentLines() {
	switch m.ActiveTabName() {
	case "Detail":
		m.contentLines = m.buildInfoLines()
	case "Logs":
		m.contentLines = m.buildLogLines()
	case "Events":
		m.contentLines = m.buildEventLines()
	}
}

func (m DetailModel) buildInfoLines() []string {
	if !m.hasData {
		return nil
	}

	labelStyle := m.theme.DetailLabelStyle()
	valueStyle := m.theme.DetailValueStyle()

	var lines []string

	// Standard fields with aligned label column.
	addField := func(label, value string) {
		l := labelStyle.Render(fmt.Sprintf("%-14s", label))
		v := valueStyle.Render(value)
		lines = append(lines, "  "+l+v)
	}

	if m.detail.Name != "" {
		addField("Name:", m.detail.Name)
	}
	if m.detail.Namespace != "" {
		addField("Namespace:", m.detail.Namespace)
	}
	if m.detail.Kind != "" {
		addField("Kind:", m.detail.Kind)
	}
	if m.detail.UID != "" {
		addField("UID:", m.detail.UID)
	}
	if m.detail.CreatedAt != "" {
		addField("Created:", m.detail.CreatedAt)
	}

	// Labels section.
	if len(m.detail.Labels) > 0 {
		keys := sortedKeys(m.detail.Labels)
		if len(keys) <= 3 {
			// Inline: Labels:  key=val, key=val
			var pairs []string
			for _, k := range keys {
				pairs = append(pairs, k+"="+m.detail.Labels[k])
			}
			l := labelStyle.Render(fmt.Sprintf("%-14s", "Labels:"))
			v := valueStyle.Render(strings.Join(pairs, ", "))
			lines = append(lines, "  "+l+v)
		} else {
			lines = append(lines, "")
			lines = append(lines, "  "+labelStyle.Render("Labels:"))
			for _, k := range keys {
				v := m.detail.Labels[k]
				lines = append(lines, "    "+valueStyle.Render(k+"="+v))
			}
		}
	}

	// Annotations section.
	if len(m.detail.Annotations) > 0 {
		keys := sortedKeys(m.detail.Annotations)
		if len(keys) <= 3 {
			// Inline: Annotations:  key=val, key=val
			var pairs []string
			for _, k := range keys {
				pairs = append(pairs, k+"="+m.detail.Annotations[k])
			}
			l := labelStyle.Render(fmt.Sprintf("%-14s", "Annotations:"))
			v := valueStyle.Render(strings.Join(pairs, ", "))
			lines = append(lines, "  "+l+v)
		} else {
			lines = append(lines, "")
			lines = append(lines, "  "+labelStyle.Render("Annotations:"))
			for _, k := range keys {
				v := m.detail.Annotations[k]
				lines = append(lines, "    "+valueStyle.Render(k+"="+v))
			}
		}
	}

	// Extra fields.
	if len(m.detail.Fields) > 0 {
		lines = append(lines, "")
		for _, f := range m.detail.Fields {
			addField(f.Label+":", f.Value)
		}
	}

	// Containers section.
	if len(m.detail.Containers) > 0 {
		lines = append(lines, "")
		lines = append(lines, "  "+labelStyle.Render("Containers:"))
		for _, c := range m.detail.Containers {
			prefix := "  "
			if c.Init {
				prefix = "  (init) "
			}
			nameStr := labelStyle.Render(prefix + c.Name)
			lines = append(lines, "  "+nameStr)

			addContainerField := func(label, value string) {
				if value != "" {
					l := labelStyle.Render(fmt.Sprintf("      %-10s", label))
					v := valueStyle.Render(value)
					lines = append(lines, "  "+l+v)
				}
			}
			addContainerField("Image:", c.Image)
			addContainerField("State:", c.State)
			readyStr := "false"
			if c.Ready {
				readyStr = "true"
			}
			addContainerField("Ready:", readyStr)
			if c.Restarts > 0 {
				addContainerField("Restarts:", fmt.Sprintf("%d", c.Restarts))
			}
			if c.Ports != "" {
				addContainerField("Ports:", c.Ports)
			}
		}
	}

	return lines
}

func (m DetailModel) buildEventLines() []string {
	if !m.hasData || len(m.events) == 0 {
		return []string{"  " + m.theme.DetailValueStyle().Render("No events")}
	}

	// Compute column widths.
	typeW := len("TYPE")
	reasonW := len("REASON")
	objectW := len("OBJECT")
	messageW := len("MESSAGE")
	ageW := len("AGE")

	for _, e := range m.events {
		if len(e.Type) > typeW {
			typeW = len(e.Type)
		}
		if len(e.Reason) > reasonW {
			reasonW = len(e.Reason)
		}
		if len(e.Object) > objectW {
			objectW = len(e.Object)
		}
		if len(e.Message) > messageW {
			messageW = len(e.Message)
		}
		if len(e.Age) > ageW {
			ageW = len(e.Age)
		}
	}

	// Cap message width to avoid overly wide lines.
	maxMsgW := m.width - typeW - reasonW - objectW - ageW - 12 // 4 gaps of 2 + leading spaces
	if maxMsgW < 10 {
		maxMsgW = 10
	}
	if messageW > maxMsgW {
		messageW = maxMsgW
	}

	labelStyle := m.theme.DetailLabelStyle()
	valueStyle := m.theme.DetailValueStyle()

	formatRow := func(t, r, o, msg, age string) string {
		// Truncate message if needed.
		if len(msg) > messageW {
			msg = msg[:messageW-1] + "…"
		}
		return fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s",
			typeW, t, reasonW, r, objectW, o, messageW, msg, ageW, age)
	}

	var lines []string
	header := formatRow("TYPE", "REASON", "OBJECT", "MESSAGE", "AGE")
	lines = append(lines, labelStyle.Render(header))

	for _, e := range m.events {
		row := formatRow(e.Type, e.Reason, e.Object, e.Message, e.Age)
		lines = append(lines, valueStyle.Render(row))
	}

	return lines
}

func (m DetailModel) buildLogLines() []string {
	if m.resourceType != k8s.ResourcePods {
		return []string{"  " + m.theme.DetailValueStyle().Render("Logs not available for this resource type")}
	}
	if len(m.logLines) == 0 {
		return []string{"  " + m.theme.DetailValueStyle().Render("Waiting for logs...")}
	}
	return m.logLines
}

// sortedKeys returns the keys of a map sorted alphabetically.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
