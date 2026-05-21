package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// logLine holds an unrendered log entry. Wrapping is deferred to render time
// so log output reflows correctly when the panel is resized.
type logLine struct {
	container string
	text      string
}

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
	logLines     []logLine
	maxLogLines  int
	resourceType k8s.ResourceType
	searching    bool
	searchQuery  string
	followTail   bool // Logs tab: stick to bottom on new lines until user scrolls up
}

// IsSearching returns true if the detail panel is in search mode.
func (m DetailModel) IsSearching() bool { return m.searching }

// HasActiveFilter returns true if a search filter is active.
func (m DetailModel) HasActiveFilter() bool { return m.searchQuery != "" }

// NewDetailModel creates a new detail model with no data and the Detail tab active.
func NewDetailModel(t *theme.Theme) DetailModel {
	return DetailModel{
		activeTab:   DetailTabInfo,
		tabs:        []string{"YAML", "Events"},
		theme:       t,
		maxLogLines: 1000,
		followTail:  true,
	}
}

// FollowTail reports whether the Logs tab auto-scrolls to the bottom on new lines.
func (m DetailModel) FollowTail() bool { return m.followTail }

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
			m = m.disableFollowIfLogs()
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
		case 'd':
			half := m.contentHeight() / 2
			if half < 1 {
				half = 1
			}
			m.scrollOffset += half
			if m.scrollOffset > m.maxScrollOffset() {
				m.scrollOffset = m.maxScrollOffset()
			}
		case 'u':
			half := m.contentHeight() / 2
			if half < 1 {
				half = 1
			}
			m.scrollOffset -= half
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			m = m.disableFollowIfLogs()
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
	case msg.Type == tea.KeyDown:
		m = m.scrollDown()
		return m, nil
	case msg.Type == tea.KeyUp:
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
	return m.disableFollowIfLogs()
}

func (m DetailModel) scrollToBottom() DetailModel {
	m.scrollOffset = m.maxScrollOffset()
	if m.ActiveTabName() == "Logs" {
		m.followTail = true
	}
	return m
}

// disableFollowIfLogs turns off follow-tail when the user manually scrolls up
// inside the Logs tab. Outside of Logs it is a no-op.
func (m DetailModel) disableFollowIfLogs() DetailModel {
	if m.ActiveTabName() == "Logs" {
		m.followTail = false
	}
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
		if m.ActiveTabName() == "Logs" {
			m.followTail = true
			m.scrollOffset = m.maxScrollOffset()
		}
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

// SetSize sets the dimensions of the detail panel. When the width changes the
// content lines are rebuilt so wrap points reflow to the new width — this is
// what makes panel expand (= / -) feel correct.
func (m *DetailModel) SetSize(width, height int) {
	widthChanged := width != m.width
	m.width = width
	m.height = height
	if widthChanged && m.hasData {
		m.buildContentLines()
		if m.ActiveTabName() == "Logs" && m.followTail {
			m.scrollOffset = m.maxScrollOffset()
		}
	}
}

// SetFocused sets whether the detail panel is focused.
func (m *DetailModel) SetFocused(focused bool) {
	m.focused = focused
}

// CopyableContent returns the current tab's content as plain text (no ANSI
// codes), respecting the active search filter. Used by the global `y` key
// to copy the visible panel content to the clipboard.
//
// On the Detail tab, when YAML is available and no search filter is active,
// the raw unwrapped YAML is returned so it stays valid for paste-back use.
func (m DetailModel) CopyableContent() string {
	if !m.hasData {
		return ""
	}
	if m.ActiveTabName() == "YAML" && m.detail.YAML != "" && m.searchQuery == "" {
		return strings.TrimRight(m.detail.YAML, "\n")
	}
	lines := m.filteredContentLines()
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = strings.TrimRight(ansi.Strip(l), " ")
	}
	return strings.Join(plain, "\n")
}

// ScrollInfo returns scroll position for the detail panel.
func (m DetailModel) ScrollInfo() *ScrollInfo {
	lines := m.filteredContentLines()
	if len(lines) == 0 {
		return nil
	}
	pos := m.scrollOffset + 1
	if pos > len(lines) {
		pos = len(lines)
	}
	return &ScrollInfo{Position: pos, Total: len(lines)}
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

// ActiveTabTitle returns the active tab name with a state marker suffix when
// applicable — currently used to surface follow-tail state on the Logs tab.
// Embed this in Panel 3's border title (which scopes to the active tab only),
// rather than the full TabTitle bar on Panel 2.
func (m DetailModel) ActiveTabTitle() string {
	name := m.ActiveTabName()
	if name == "Logs" && m.followTail {
		return name + " ▼"
	}
	return name
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
	def := k8s.DefaultRegistry.Get(rt)
	if def != nil && def.HasLogs {
		m.tabs = []string{"YAML", "Logs", "Events"}
	} else if rt == k8s.ResourceEvents {
		m.tabs = []string{"YAML"}
	} else {
		m.tabs = []string{"YAML", "Events"}
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
	return "YAML"
}

// AppendLogLine appends a formatted log line to the log buffer.
// If the buffer exceeds maxLogLines, the oldest lines are trimmed.
// If the Logs tab is active, content lines are rebuilt.
//
// Raw (container, text) pairs are stored; wrapping happens at render time so
// log output reflows on resize.
func (m *DetailModel) AppendLogLine(container, text string) {
	m.logLines = append(m.logLines, logLine{container: container, text: text})
	if len(m.logLines) > m.maxLogLines {
		m.logLines = m.logLines[len(m.logLines)-m.maxLogLines:]
	}
	if m.ActiveTabName() == "Logs" {
		m.buildContentLines()
		if m.followTail {
			m.scrollOffset = m.maxScrollOffset()
		}
	}
}

// buildContentLines rebuilds the pre-rendered content lines for the current tab.
func (m *DetailModel) buildContentLines() {
	switch m.ActiveTabName() {
	case "YAML":
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
	if m.detail.YAML != "" {
		return m.buildYAMLLines()
	}

	labelStyle := m.theme.DetailLabelStyle()
	valueStyle := m.theme.DetailValueStyle()

	const labelW = 12
	const indent = "  "
	valueAvail := m.width - len(indent) - labelW

	var lines []string

	addField := func(label, value string) {
		labelCol := labelStyle.Render(fmt.Sprintf("%-*s", labelW, label))
		valueLines := wrapPlain(value, valueAvail)
		if len(valueLines) == 0 {
			valueLines = []string{""}
		}
		lines = append(lines, indent+labelCol+valueStyle.Render(valueLines[0]))
		contIndent := indent + strings.Repeat(" ", labelW)
		for _, v := range valueLines[1:] {
			lines = append(lines, contIndent+valueStyle.Render(v))
		}
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
			var pairs []string
			for _, k := range keys {
				pairs = append(pairs, k+"="+m.detail.Labels[k])
			}
			addField("Labels:", strings.Join(pairs, ", "))
		} else {
			lines = append(lines, indent+labelStyle.Render("Labels:"))
			subAvail := m.width - 4
			subIndent := "    "
			contIndent := "      "
			for _, k := range keys {
				v := m.detail.Labels[k]
				wrapped := wrapPlain(k+"="+v, subAvail)
				if len(wrapped) == 0 {
					continue
				}
				lines = append(lines, subIndent+valueStyle.Render(wrapped[0]))
				for _, cont := range wrapped[1:] {
					lines = append(lines, contIndent+valueStyle.Render(cont))
				}
			}
		}
	}

	// Annotations section.
	if len(m.detail.Annotations) > 0 {
		keys := sortedKeys(m.detail.Annotations)
		if len(keys) <= 3 {
			var pairs []string
			for _, k := range keys {
				pairs = append(pairs, k+"="+m.detail.Annotations[k])
			}
			addField("Annotations:", strings.Join(pairs, ", "))
		} else {
			lines = append(lines, indent+labelStyle.Render("Annotations:"))
			subAvail := m.width - 4
			subIndent := "    "
			contIndent := "      "
			for _, k := range keys {
				v := m.detail.Annotations[k]
				wrapped := wrapPlain(k+"="+v, subAvail)
				if len(wrapped) == 0 {
					continue
				}
				lines = append(lines, subIndent+valueStyle.Render(wrapped[0]))
				for _, cont := range wrapped[1:] {
					lines = append(lines, contIndent+valueStyle.Render(cont))
				}
			}
		}
	}

	// Extra fields.
	for _, f := range m.detail.Fields {
		addField(f.Label+":", f.Value)
	}

	// Containers section.
	if len(m.detail.Containers) > 0 {
		lines = append(lines, indent+labelStyle.Render("Containers:"))
		const cFieldW = 8
		cFieldAvail := m.width - 6 - cFieldW
		for _, c := range m.detail.Containers {
			prefix := "  "
			if c.Init {
				prefix = "  (init) "
			}
			lines = append(lines, indent+labelStyle.Render(prefix+c.Name))

			addContainerField := func(label, value string) {
				if value == "" {
					return
				}
				labelCol := labelStyle.Render(fmt.Sprintf("%-*s", cFieldW, label))
				wrapped := wrapPlain(value, cFieldAvail)
				if len(wrapped) == 0 {
					wrapped = []string{""}
				}
				lines = append(lines, "      "+labelCol+valueStyle.Render(wrapped[0]))
				contIndent := "      " + strings.Repeat(" ", cFieldW)
				for _, v := range wrapped[1:] {
					lines = append(lines, contIndent+valueStyle.Render(v))
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

// buildYAMLLines renders the resource's raw YAML with a 2-space left indent,
// wrapping long lines and applying syntax highlighting per line.
func (m DetailModel) buildYAMLLines() []string {
	raw := strings.TrimRight(m.detail.YAML, "\n")
	if raw == "" {
		return nil
	}
	valueStyle := m.theme.DetailValueStyle()
	const indent = "  "
	const contIndent = "    "
	avail := m.width - len(indent)
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if avail <= 0 {
			out = append(out, indent+highlightYAMLLine(line, m.theme))
			continue
		}
		wrapped := wrapPlain(line, avail)
		if len(wrapped) == 0 {
			out = append(out, indent)
			continue
		}
		out = append(out, indent+highlightYAMLLine(wrapped[0], m.theme))
		for _, w := range wrapped[1:] {
			out = append(out, contIndent+valueStyle.Render(w))
		}
	}
	return out
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

	// Cap message width so MESSAGE column wraps within panel.
	maxMsgW := m.width - typeW - reasonW - objectW - ageW - 12 // 4 gaps of 2 + leading 2 + trailing 2
	if maxMsgW < 10 {
		maxMsgW = 10
	}
	if messageW > maxMsgW {
		messageW = maxMsgW
	}

	labelStyle := m.theme.DetailLabelStyle()
	valueStyle := m.theme.DetailValueStyle()

	// Indent for message continuation lines: leading 2 + typeW + 2 + reasonW + 2 + objectW + 2
	msgIndent := strings.Repeat(" ", 2+typeW+2+reasonW+2+objectW+2)

	formatRows := func(t, r, o, msg, age string) []string {
		msgLines := wrapPlain(msg, messageW)
		if len(msgLines) == 0 {
			msgLines = []string{""}
		}
		first := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s",
			typeW, t, reasonW, r, objectW, o, messageW, msgLines[0], ageW, age)
		out := []string{first}
		for _, cont := range msgLines[1:] {
			out = append(out, msgIndent+cont)
		}
		return out
	}

	var lines []string
	for _, row := range formatRows("TYPE", "REASON", "OBJECT", "MESSAGE", "AGE") {
		lines = append(lines, labelStyle.Render(row))
	}

	for _, e := range m.events {
		for _, row := range formatRows(e.Type, e.Reason, e.Object, e.Message, e.Age) {
			lines = append(lines, valueStyle.Render(row))
		}
	}

	return lines
}

// wrapPlain wraps plain (no ANSI) text to the given width, breaking at word
// boundaries when possible. Returns one string per output line. The returned
// lines do not include any indentation — callers prepend continuation indent.
func wrapPlain(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if len(text) <= width {
		return []string{text}
	}
	var out []string
	rest := text
	for len(rest) > width {
		cut := width
		// If the boundary char is a space, break exactly at width — keeps
		// "hello world" intact when width == 11.
		if rest[width] != ' ' {
			if idx := strings.LastIndex(rest[:width], " "); idx > 0 {
				cut = idx
			}
		}
		out = append(out, strings.TrimRight(rest[:cut], " "))
		rest = strings.TrimLeft(rest[cut:], " ")
	}
	if rest != "" {
		out = append(out, rest)
	}
	return out
}

func (m DetailModel) buildLogLines() []string {
	if m.resourceType != k8s.ResourcePods {
		return []string{"  " + m.theme.DetailValueStyle().Render("Logs not available for this resource type")}
	}
	if len(m.logLines) == 0 {
		return []string{"  " + m.theme.DetailValueStyle().Render("Waiting for logs...")}
	}
	var lines []string
	for _, ll := range m.logLines {
		prefix := "  " + ll.container + " │ "
		textW := m.width - len(prefix)
		wrapped := wrapPlain(ll.text, textW)
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		lines = append(lines, prefix+wrapped[0])
		if len(wrapped) > 1 {
			contIndent := "  " + strings.Repeat(" ", len(ll.container)) + " │ "
			for _, w := range wrapped[1:] {
				lines = append(lines, contIndent+w)
			}
		}
	}
	return lines
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
