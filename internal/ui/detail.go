package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// logLine holds an unrendered log entry. Wrapping is deferred to render time
// so log output reflows correctly when the panel is resized.
//
// pod is empty for single-pod log streams (the user is on a Pod detail view —
// the pod identity is implicit). For aggregate streams (Deployment / workload
// kinds), pod carries the source pod name so the render path can emit a
// three-segment `<pod-hash>│<container>│<text>` prefix.
type logLine struct {
	pod       string
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
	refetching   bool // true while fetchResourceDetail is in-flight; drives spinner
	spinnerFrame int

	// Links tab state: entries are the logical rows (drillable + info +
	// section headers); linkCursor is the index of the currently-selected
	// entry within the *current level*. Cursor only lands on selectable
	// entries (sections skipped).
	linkEntries    []linkEntry
	linkCursor     int
	linkCursorLine int // display-line index of cursor row; -1 when none

	// drillStack is the chain of resources the user has drilled into via
	// the Links tab (level 2+). Empty = at level 1 (root = the
	// table-selected resource, whose data is m.detail). When non-empty,
	// rebuildLinkEntries reads from drillStack[top].detail instead of
	// m.detail. rootCursor preserves m.linkCursor at level 1 so it can be
	// restored when popping back to root.
	drillStack []drillFrame
	rootCursor int
}

// drillFrame represents one level on the Links-tab drill chain. ref is the
// (kind, ns, name) identity used for cycle detection; item carries the
// fetched resource (UID + Raw for YAML); detail is the per-level link
// payload; cursor is the link cursor remembered for this level when the
// user drilled deeper, so back-navigation puts the cursor right where it
// was.
type drillFrame struct {
	ref    k8s.RefTarget
	item   k8s.ResourceItem
	detail k8s.ResourceDetail
	cursor int
}

type detailSpinnerTickMsg struct{}

var detailSpinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// BeginRefetch marks the panel as refetching and returns a Cmd that drives
// the spinner animation. AppModel calls this whenever it dispatches
// fetchResourceDetail; SetDetail clears the flag when the new data arrives.
func (m *DetailModel) BeginRefetch() tea.Cmd {
	if m.refetching {
		return nil // already ticking
	}
	m.refetching = true
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return detailSpinnerTickMsg{}
	})
}

// IsRefetching reports whether the spinner should be shown.
func (m DetailModel) IsRefetching() bool { return m.refetching }

// SpinnerSuffix returns " <frame>" while refetching, or "" otherwise. Embed in
// the panel border title so the user has a visible "loading" affordance.
func (m DetailModel) SpinnerSuffix() string {
	if !m.refetching {
		return ""
	}
	return " " + string(detailSpinnerFrames[m.spinnerFrame%len(detailSpinnerFrames)])
}

// advanceSpinner moves to the next frame and returns the next tick command,
// or nil when refetching has finished.
func (m *DetailModel) advanceSpinner() tea.Cmd {
	if !m.refetching {
		return nil
	}
	m.spinnerFrame++
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return detailSpinnerTickMsg{}
	})
}

// IsSearching returns true if the detail panel is in search mode.
func (m DetailModel) IsSearching() bool { return m.searching }

// ClearSearch drops any active detail-panel search filter and exits
// search mode. Used by the Relatives-tab space hotkey so the freshly
// switched-to resource isn't hidden behind a stale filter inherited
// from the previous selection.
func (m *DetailModel) ClearSearch() {
	m.searching = false
	m.searchQuery = ""
	m.scrollOffset = 0
}

// HasActiveFilter returns true if a search filter is active.
func (m DetailModel) HasActiveFilter() bool { return m.searchQuery != "" }

// CurrentLevelYAML returns the YAML for the Links-tab drill level the
// user is currently viewing — at depth 1 that's the table-selected
// resource's YAML, at deeper levels it's the YAML of the resource the
// user has drilled into. Used as the Y-key fallback on the Links tab
// when the cursor isn't on a drillable entry.
func (m DetailModel) CurrentLevelYAML() string {
	if len(m.drillStack) == 0 {
		return m.detail.YAML
	}
	return m.drillStack[len(m.drillStack)-1].detail.YAML
}

// YAMLContent returns the raw YAML for the resource currently shown in the
// detail panel, or "" if no YAML is loaded. Used by the global `Y` key to
// open the YAML popup.
func (m DetailModel) YAMLContent() string { return m.detail.YAML }

// NewDetailModel creates a new detail model with no data and the Links tab
// active. SetResourceType refines the tab list (and reorders for Pod/Deploy).
func NewDetailModel(t *theme.Theme) DetailModel {
	return DetailModel{
		activeTab:   DetailTabInfo,
		tabs:        []string{"Relatives", "Events"},
		theme:       t,
		maxLogLines: 1000,
		followTail:  true,
		linkCursor:  -1,
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

	case detailSpinnerTickMsg:
		return m, m.advanceSpinner()

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

	// Links tab uses j/k for cursor navigation (not line scroll) and Enter
	// to drill into the highlighted ref. Other tabs scroll by line — fall
	// through to the standard logic.
	if m.ActiveTabName() == "Relatives" {
		if newModel, handled, cmd := m.handleLinkKey(msg); handled {
			return newModel, cmd
		}
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

// handleLinkKey intercepts the keys with Links-tab-specific semantics:
//   - j/k (or arrow keys): move the cursor between drillable entries,
//     auto-scrolling the viewport so the cursor stays visible.
//   - Enter / l: drill into the highlighted ref (push a frame onto the
//     Links chain — emits LinkPushMsg, AppModel handles cycle check + fetch).
//   - h / Esc: pop one level off the chain. No-op at root level.
//   - b: open the breadcrumb popup so the user can jump back to any
//     ancestor level. No-op at root (nothing to navigate).
//
// Returns handled=false to let the caller fall back to the generic per-line
// scroll handlers for everything else.
func (m DetailModel) handleLinkKey(msg tea.KeyMsg) (DetailModel, bool, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, false, nil
		}
		switch msg.Runes[0] {
		case 'j':
			m.linkCursor = nextSelectableCursor(m.linkEntries, m.linkCursor, +1)
			m.buildContentLines()
			m = m.scrollLinkCursorIntoView()
			return m, true, nil
		case 'k':
			m.linkCursor = nextSelectableCursor(m.linkEntries, m.linkCursor, -1)
			m.buildContentLines()
			m = m.scrollLinkCursorIntoView()
			return m, true, nil
		case 'l':
			return m.dispatchLinkPush()
		case 'h':
			return m.dispatchLinkPop()
		case 'b':
			if m.Depth() <= 1 {
				return m, true, nil
			}
			return m, true, func() tea.Msg { return LinkBreadcrumbMsg{} }
		}
	case tea.KeyDown:
		m.linkCursor = nextSelectableCursor(m.linkEntries, m.linkCursor, +1)
		m.buildContentLines()
		m = m.scrollLinkCursorIntoView()
		return m, true, nil
	case tea.KeyUp:
		m.linkCursor = nextSelectableCursor(m.linkEntries, m.linkCursor, -1)
		m.buildContentLines()
		m = m.scrollLinkCursorIntoView()
		return m, true, nil
	case tea.KeyEnter:
		return m.dispatchLinkPush()
	case tea.KeyEscape:
		// Esc only handled here when we're drilled in; at root, fall
		// through so the generic Esc handler (search-clear) can run.
		if m.Depth() > 1 {
			return m.dispatchLinkPop()
		}
	}
	return m, false, nil
}

// dispatchLinkPush emits LinkPushMsg for the cursor-pointed entry. If
// the cursor isn't on a drillable row, it's a no-op (handled=true so the
// caller doesn't double-process the key).
func (m DetailModel) dispatchLinkPush() (DetailModel, bool, tea.Cmd) {
	ref := m.SelectedLinkRef()
	if ref == nil {
		return m, true, nil
	}
	target := *ref
	return m, true, func() tea.Msg { return LinkPushMsg{Ref: target} }
}

// dispatchLinkPop pops one level off the chain. No-op at root.
func (m DetailModel) dispatchLinkPop() (DetailModel, bool, tea.Cmd) {
	if m.Depth() <= 1 {
		return m, true, nil
	}
	m.PopDrillFrame()
	return m, true, nil
}

// scrollLinkCursorIntoView nudges scrollOffset so the cursor row is
// inside the visible viewport. Mirrors the standard "follow cursor" behavior
// of any selectable list — without it, j/k can move the cursor past the
// bottom of the panel and the user has to manually scroll to see it.
func (m DetailModel) scrollLinkCursorIntoView() DetailModel {
	if m.linkCursorLine < 0 {
		return m
	}
	h := m.contentHeight()
	if h <= 0 {
		return m
	}
	if m.linkCursorLine < m.scrollOffset {
		m.scrollOffset = m.linkCursorLine
	} else if m.linkCursorLine >= m.scrollOffset+h {
		m.scrollOffset = m.linkCursorLine - h + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	return m
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

// SetFocused sets whether the detail panel is focused. Rebuilds the
// pre-rendered Links content so the cursor row picks the focused vs
// unfocused style — without this, the panel would keep its previous
// highlight color until the next data refresh.
func (m *DetailModel) SetFocused(focused bool) {
	if m.focused == focused {
		return
	}
	m.focused = focused
	if m.ActiveTabName() == "Relatives" {
		m.buildContentLines()
	}
}

// CopyableContent returns the current tab's content as plain text (no ANSI
// codes), respecting the active search filter. Used by the global `y` key
// to copy the visible panel content to the clipboard. For raw YAML, the
// user opens the `Y` popup and copies from there.
func (m DetailModel) CopyableContent() string {
	if !m.hasData {
		return ""
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
//
// Does NOT touch the Links drill chain — background watcher refreshes
// keep dispatching detail fetches for the still-selected root row, and a
// stale-arriving ResourceDetailMsg would otherwise wipe the user's in-
// flight drill state and snap them back to level 1. The row-change path
// (RowSelectedMsg) calls ResetDrillStack() explicitly before dispatch;
// namespace/context switches go through ClearDetail() which resets too.
func (m *DetailModel) SetDetail(detail k8s.ResourceDetail, events []k8s.EventItem) {
	m.detail = detail
	m.events = events
	m.hasData = true
	m.scrollOffset = 0
	m.refetching = false // fresh data arrived — stop the spinner
	m.buildContentLines()
}

// PushDrillFrame appends a level to the Links drill chain — used after a
// successful drill fetch (l/Enter on a drillable entry). Saves the
// outgoing level's cursor so back-navigation restores it.
func (m *DetailModel) PushDrillFrame(ref k8s.RefTarget, item k8s.ResourceItem, detail k8s.ResourceDetail) {
	if len(m.drillStack) == 0 {
		m.rootCursor = m.linkCursor
	} else {
		m.drillStack[len(m.drillStack)-1].cursor = m.linkCursor
	}
	m.drillStack = append(m.drillStack, drillFrame{
		ref:    ref,
		item:   item,
		detail: detail,
		cursor: -1,
	})
	m.linkCursor = -1
	m.scrollOffset = 0
	m.refetching = false
	m.buildContentLines()
}

// PopDrillFrame removes the top of the drill chain — used by h/Esc on a
// deeper level. No-op at level 1. Restores the linkCursor to whatever it
// was on the level we're returning to.
func (m *DetailModel) PopDrillFrame() {
	if len(m.drillStack) == 0 {
		return
	}
	m.drillStack = m.drillStack[:len(m.drillStack)-1]
	if len(m.drillStack) == 0 {
		m.linkCursor = m.rootCursor
	} else {
		m.linkCursor = m.drillStack[len(m.drillStack)-1].cursor
	}
	m.scrollOffset = 0
	m.buildContentLines()
}

// JumpToDrillLevel pops frames so the chain is `level` deep. level=1
// returns to root; level=Depth() is a no-op. Used by the breadcrumb popup
// when the user picks an ancestor level. Out-of-range levels are ignored.
func (m *DetailModel) JumpToDrillLevel(level int) {
	if level < 1 || level > m.Depth() {
		return
	}
	for m.Depth() > level {
		m.PopDrillFrame()
	}
}

// ResetDrillStack returns the panel to level 1 without changing m.detail
// — used when the user moves to a different table row, which the chain
// no longer applies to.
func (m *DetailModel) ResetDrillStack() {
	if len(m.drillStack) == 0 {
		return
	}
	m.drillStack = nil
	m.linkCursor = m.rootCursor
	m.scrollOffset = 0
	m.buildContentLines()
}

// TabTitle returns the tab bar string for embedding in the panel border.
// Adds a `(N)` suffix to the Links tab when the user has drilled deeper
// — N is the 1-indexed level (1=root, 2=first drill, ...). The
// breadcrumb popup (`i` key) is the only way to see the full chain;
// this number is the at-a-glance hint that you're not at root.
func (m DetailModel) TabTitle() string {
	var parts []string
	for i, tab := range m.tabs {
		label := m.tabLabel(tab)
		if DetailTab(i) == m.activeTab {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, " "+label+" ")
		}
	}
	return strings.Join(parts, "─")
}

// ActiveTabTitle returns the active tab name with a state marker suffix when
// applicable — currently used to surface follow-tail state on the Logs tab
// and the drill level on the Links tab. Embed this in Panel 3's border title
// (which scopes to the active tab only), rather than the full TabTitle bar
// on Panel 2.
func (m DetailModel) ActiveTabTitle() string {
	name := m.ActiveTabName()
	if name == "Logs" && m.followTail {
		return name + " ▼"
	}
	return m.tabLabel(name)
}

// tabLabel returns the per-tab label as it should appear in the tab bar,
// including the drill-level suffix for the Links tab. The chain glyph
// matches the per-row drill arrow + the breadcrumb middle markers so
// the three surfaces speak the same vocabulary — "you've gone N levels
// down this chain."
func (m DetailModel) tabLabel(name string) string {
	if name == "Relatives" && m.Depth() > 1 {
		return fmt.Sprintf("Relatives %s%d", linksDrillArrow, m.Depth())
	}
	return name
}

// BorderTopRightHint returns a short string to render at the top-right
// of panel 3's border, or "" when no hint applies. Currently used to
// surface the breadcrumb key when the user is in a drill chain on the
// Links tab — discoverable affordance for "press b to see where you've
// been". The chosen format keeps the hotkey in brackets so the user
// can pattern-match it against `b` in the help screen.
func (m DetailModel) BorderTopRightHint() string {
	if m.ActiveTabName() == "Relatives" && m.Depth() > 1 {
		return "[b]readcrumbs"
	}
	return ""
}

// ClearDetail clears the detail data and tears down the Links drill chain.
// Used by namespace/context switches, which invalidate every drilled-into
// resource (different cluster scope, no guarantee the chain still exists).
func (m *DetailModel) ClearDetail() {
	m.detail = k8s.ResourceDetail{}
	m.events = nil
	m.hasData = false
	m.scrollOffset = 0
	m.contentLines = nil
	m.logLines = nil
	m.drillStack = nil
	m.rootCursor = -1
	m.linkCursor = -1
}

// SetResourceType sets the current resource type and adjusts available tabs.
//
// Tab order convention — Relatives is always first when present, so the
// space-hotkey jump-to-this-resource lands on the same tab the user came
// from (no visual whiplash). Logs follows because users on a Pod/Deployment
// almost always want logs once they've oriented themselves.
//
//   - Pods / Deployments: Relatives → Logs → Events
//   - Events:             Relatives alone
//   - !linksApplicable:   Events only (Namespace — Relatives tab dropped)
//   - everything else:    Relatives → Events
//
// Pod gets the structured Owner/Node/SA/Volumes Relatives; other kinds
// use the generic labels + sections fallback so the panel never renders
// empty.
func (m *DetailModel) SetResourceType(rt k8s.ResourceType) {
	m.resourceType = rt
	switch {
	case rt == k8s.ResourcePods, rt == k8s.ResourceDeployments:
		m.tabs = []string{"Relatives", "Logs", "Events"}
	case rt == k8s.ResourceEvents:
		m.tabs = []string{"Relatives"}
	case !linksApplicable(rt):
		m.tabs = []string{"Events"}
	default:
		m.tabs = []string{"Relatives", "Events"}
	}
	m.activeTab = 0
	m.scrollOffset = 0
	m.linkCursor = -1
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
// Pass pod = "" for single-pod streams. For aggregate streams (Deployment
// and other workload kinds), pass the source pod name so the render path can
// label each line with its origin.
func (m *DetailModel) AppendLogLine(pod, container, text string) {
	m.logLines = append(m.logLines, logLine{pod: pod, container: container, text: text})
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
	case "Relatives":
		m.rebuildLinkEntries()
		lines, _, cursorLine := renderLinkEntries(m.linkEntries, m.linkCursor, m.width, m.theme, linksPlaceholderEmpty, m.focused)
		m.contentLines = lines
		m.linkCursorLine = cursorLine
	case "Logs":
		m.contentLines = m.buildLogLines()
	case "Events":
		m.contentLines = m.buildEventLines()
	}
}

// RootRef returns the RefTarget identifying the root (table-selected)
// resource — used by AppModel's cycle-check on drill push.
func (m DetailModel) RootRef() k8s.RefTarget {
	return k8s.RefTarget{
		Type:      m.resourceType,
		Name:      m.detail.Name,
		Namespace: m.detail.Namespace,
	}
}

// DrillChain returns the full identity chain from root to current top
// — root first, top last. Used by cycle detection and the breadcrumb
// popup. At depth 1 it returns just the root.
func (m DetailModel) DrillChain() []k8s.RefTarget {
	out := make([]k8s.RefTarget, 0, len(m.drillStack)+1)
	out = append(out, m.RootRef())
	for _, f := range m.drillStack {
		out = append(out, f.ref)
	}
	return out
}

// CurrentLevelItem returns the ResourceItem of the level currently
// displayed on the Links tab. At root (depth 1) the zero value is
// returned — the caller (AppModel) substitutes the table-selected item.
func (m DetailModel) CurrentLevelItem() k8s.ResourceItem {
	return m.currentLevelItem()
}

// CurrentLevelRef returns the (kind, ns, name) identity of the resource
// the user is currently viewing on the Links tab. At root it's the
// table-selected resource (same as RootRef); at deeper levels it's the
// drilled-into resource. Used by the "space — jump to this resource"
// flow so the caller doesn't have to assemble the ref from CurrentLevelKind
// + CurrentLevelItem manually.
func (m DetailModel) CurrentLevelRef() k8s.RefTarget {
	if len(m.drillStack) == 0 {
		return m.RootRef()
	}
	return m.drillStack[len(m.drillStack)-1].ref
}

// Depth returns the current Links-tab drill level. Level 1 = root
// (table-selected resource); level 2+ = drilled in. Always >= 1 when the
// panel has data.
func (m DetailModel) Depth() int { return 1 + len(m.drillStack) }

// currentLevelDetail returns the ResourceDetail for the level the user is
// currently viewing on the Links tab.
func (m DetailModel) currentLevelDetail() k8s.ResourceDetail {
	if len(m.drillStack) == 0 {
		return m.detail
	}
	return m.drillStack[len(m.drillStack)-1].detail
}

// currentLevelKind returns the ResourceType for the level the user is
// currently viewing on the Links tab. At level 1 this is m.resourceType;
// at deeper levels it's the drilled-into resource's kind.
func (m DetailModel) currentLevelKind() k8s.ResourceType {
	if len(m.drillStack) == 0 {
		return m.resourceType
	}
	return m.drillStack[len(m.drillStack)-1].ref.Type
}

// currentLevelItem returns the ResourceItem of the current level — used
// for Y key fallback when the cursor isn't on a drillable entry. Returns
// zero ResourceItem at level 1 (caller falls back to the table-selected
// item from AppModel).
func (m DetailModel) currentLevelItem() k8s.ResourceItem {
	if len(m.drillStack) == 0 {
		return k8s.ResourceItem{}
	}
	return m.drillStack[len(m.drillStack)-1].item
}

// rebuildLinkEntries refreshes m.linkEntries based on the current
// resource type + detail data, and re-clamps the cursor to the first
// selectable entry when out of bounds.
func (m *DetailModel) rebuildLinkEntries() {
	if !m.hasData {
		m.linkEntries = nil
		m.linkCursor = -1
		return
	}
	kind := m.currentLevelKind()
	detail := m.currentLevelDetail()
	switch kind {
	case k8s.ResourcePods:
		m.linkEntries = buildPodLinkEntries(detail)
	case k8s.ResourceServices:
		m.linkEntries = buildServiceLinkEntries(detail)
	default:
		m.linkEntries = buildGenericLinkEntries(detail)
	}
	if m.linkCursor < 0 || m.linkCursor >= len(m.linkEntries) ||
		(m.linkCursor < len(m.linkEntries) && !m.linkEntries[m.linkCursor].isSelectable()) {
		m.linkCursor = firstSelectableCursor(m.linkEntries)
	}
}

// SelectedLinkRef returns the drill ref under the Links cursor, or
// nil if the cursor is on an info-only row (or the tab has no entries).
func (m DetailModel) SelectedLinkRef() *k8s.RefTarget {
	if m.ActiveTabName() != "Relatives" {
		return nil
	}
	if m.linkCursor < 0 || m.linkCursor >= len(m.linkEntries) {
		return nil
	}
	return m.linkEntries[m.linkCursor].ref
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

// containerLogPalette is the set of foreground colors assigned to container
// log prefixes. 8 entries chosen for visual distinguishability on dark
// terminal backgrounds (Catppuccin-aligned).
var containerLogPalette = []lipgloss.Color{
	"#f38ba8", // red
	"#fab387", // peach
	"#f9e2af", // yellow
	"#a6e3a1", // green
	"#94e2d5", // teal
	"#89b4fa", // blue
	"#cba6f7", // mauve
	"#f5c2e7", // pink
}

// containerLogColor returns a stable per-container color via a tiny FNV-ish
// hash. Same container name always maps to the same palette entry across the
// session, so users can visually associate a color with a container.
func containerLogColor(name string) lipgloss.Color {
	return fnvPaletteColor(name)
}

// podLogColor mirrors containerLogColor for the pod dimension in aggregate
// log streams. Same palette so the two colors blend visually but are derived
// from different identifiers — two pods running the same container name get
// distinct pod-color stripes.
func podLogColor(name string) lipgloss.Color {
	return fnvPaletteColor(name)
}

func fnvPaletteColor(name string) lipgloss.Color {
	h := uint32(2166136261)
	for _, b := range []byte(name) {
		h = (h ^ uint32(b)) * 16777619
	}
	return containerLogPalette[int(h)%len(containerLogPalette)]
}

// podHashTag truncates a pod name to its trailing identifier, the random
// suffix that K8s appends to ReplicaSet pods (`nginx-7f9c4d-abc12` → `abc12`).
// Falls back to last 6 chars when the name has no dash.
func podHashTag(name string) string {
	const want = 5
	if idx := strings.LastIndex(name, "-"); idx >= 0 && idx < len(name)-1 {
		tail := name[idx+1:]
		if len(tail) > want {
			return tail[len(tail)-want:]
		}
		return tail
	}
	if len(name) > want {
		return name[len(name)-want:]
	}
	return name
}

func (m DetailModel) buildLogLines() []string {
	if !supportsLogs(m.resourceType) {
		return []string{"  " + m.theme.DetailValueStyle().Render("Logs not available for this resource type")}
	}
	if len(m.logLines) == 0 {
		return []string{"  " + m.theme.DetailValueStyle().Render("Waiting for logs...")}
	}
	var lines []string
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	for _, ll := range m.logLines {
		// Build plain + styled prefixes side by side. Plain is for wrap-width
		// math (avoid counting ANSI escapes); styled is what we actually emit.
		//
		// Aggregate prefix uses `<container>@<pod-hash> │ <text>` —
		// container-first because that's what users care about during
		// debugging ("which container is this from?"); the `@<pod-hash>`
		// disambiguates between pods running the same container. `│`
		// separates the prefix block from the log line itself.
		var plainPrefix, styledPrefix string
		if ll.pod != "" {
			tag := podHashTag(ll.pod)
			podStyle := lipgloss.NewStyle().Foreground(podLogColor(ll.pod)).Bold(true)
			ctrStyle := lipgloss.NewStyle().Foreground(containerLogColor(ll.container)).Bold(true)
			plainPrefix = "  " + ll.container + "@" + tag + " │ "
			styledPrefix = "  " + ctrStyle.Render(ll.container) + dimStyle.Render("@") + podStyle.Render(tag) + " │ "
		} else {
			ctrStyle := lipgloss.NewStyle().Foreground(containerLogColor(ll.container)).Bold(true)
			plainPrefix = "  " + ll.container + " │ "
			styledPrefix = "  " + ctrStyle.Render(ll.container) + " │ "
		}
		textW := m.width - len(plainPrefix)
		wrapped := wrapPlain(ll.text, textW)
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		lines = append(lines, styledPrefix+wrapped[0])
		if len(wrapped) > 1 {
			// Visual-width math: prefix has 2 leading spaces + identifier(s) +
			// " │ " (3 cols). Continuation needs same total visual width up
			// to the rightmost " │ " so text columns align.
			prefixCols := lipgloss.Width(plainPrefix)
			contIndent := strings.Repeat(" ", prefixCols-3) + " │ "
			for _, w := range wrapped[1:] {
				lines = append(lines, contIndent+w)
			}
		}
	}
	return lines
}

// supportsLogs reports whether a resource type has a Logs tab in its detail
// panel. Pods stream single-pod; Deployments stream aggregate from the
// current-generation ReplicaSet pods. Other workload kinds (StatefulSet,
// DaemonSet, Job) follow the same aggregate pattern but are out of scope
// for this iteration.
func supportsLogs(rt k8s.ResourceType) bool {
	return rt == k8s.ResourcePods || rt == k8s.ResourceDeployments
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
