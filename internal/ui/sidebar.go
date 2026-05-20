package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// SidebarCategory represents a visual group of resource types.
// Categories are non-interactive headers — they cannot be selected or collapsed.
type SidebarCategory struct {
	Label    string
	Items    []SidebarResource
	Expanded bool
}

// SidebarResource represents a single resource type entry in the sidebar.
type SidebarResource struct {
	Label        string
	ResourceType k8s.ResourceType
}

// visibleItem represents a single item in the flattened visible list.
type visibleItem struct {
	isCategory    bool
	categoryIndex int // index into categories slice (-1 for standalone)
	resourceIndex int // index into category's Items slice (-1 for category header)
	label         string
	resourceType  k8s.ResourceType
}

// SidebarModel is the Bubble Tea model for the sidebar panel.
type SidebarModel struct {
	categories   []SidebarCategory
	standalone   []SidebarResource
	cursor       int
	scrollOffset int
	focused      bool
	width        int
	height       int
	theme        *theme.Theme
	pendingG     bool
	selected     k8s.ResourceType
	searching    bool
	searchQuery  string
}

// IsSearching returns true if the sidebar is in search mode.
func (m SidebarModel) IsSearching() bool { return m.searching }

// HasActiveFilter returns true if a search filter is active.
func (m SidebarModel) HasActiveFilter() bool { return m.searchQuery != "" }

// NewSidebarModel creates a new sidebar with categories built from the registry.
func NewSidebarModel(t *theme.Theme) SidebarModel {
	return newSidebarFromRegistry(t, k8s.DefaultRegistry)
}

func newSidebarFromRegistry(t *theme.Theme, reg *k8s.Registry) SidebarModel {
	catGroups := reg.SidebarCategories()
	categories := make([]SidebarCategory, len(catGroups))
	for i, cg := range catGroups {
		items := make([]SidebarResource, len(cg.Resources))
		for j, def := range cg.Resources {
			items[j] = SidebarResource{
				Label:        def.DisplayName,
				ResourceType: def.Type,
			}
		}
		categories[i] = SidebarCategory{
			Label: cg.Label,
			Items: items,
		}
	}

	m := SidebarModel{
		categories: categories,
		focused:    false,
		selected:   k8s.ResourcePods,
		theme:      t,
	}

	visible := m.visibleItems()
	for i, item := range visible {
		if !item.isCategory && item.resourceType == k8s.ResourcePods {
			m.cursor = i
			break
		}
	}

	return m
}

// CopyableContent returns the visible sidebar tree as plain text (respecting
// the active search filter). Categories are flush-left, resources indented
// with two spaces. Used by the global `y` key.
func (m SidebarModel) CopyableContent() string {
	items := m.visibleItems()
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		if it.isCategory {
			lines = append(lines, it.label)
		} else {
			lines = append(lines, "  "+it.label)
		}
	}
	return strings.Join(lines, "\n")
}

// RefreshCategories rebuilds sidebar categories from the registry.
func (m *SidebarModel) RefreshCategories(reg *k8s.Registry) {
	catGroups := reg.SidebarCategories()
	categories := make([]SidebarCategory, len(catGroups))
	for i, cg := range catGroups {
		items := make([]SidebarResource, len(cg.Resources))
		for j, def := range cg.Resources {
			items[j] = SidebarResource{
				Label:        def.DisplayName,
				ResourceType: def.Type,
			}
		}
		categories[i] = SidebarCategory{
			Label: cg.Label,
			Items: items,
		}
	}
	m.categories = categories
	m.standalone = nil
}

// Init implements tea.Model.
func (m SidebarModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

func (m SidebarModel) handleKey(msg tea.KeyMsg) (SidebarModel, tea.Cmd) {
	if m.searching {
		return m.handleSearchKey(msg)
	}

	visible := m.visibleItems()
	if len(visible) == 0 {
		return m, nil
	}

	if m.pendingG {
		m.pendingG = false
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'g' {
			// gg — jump to first resource item.
			m.cursor = m.firstResourceIndex(visible)
			m.ensureCursorVisible()
			item := visible[m.cursor]
			m.selected = item.resourceType
			return m, func() tea.Msg {
				return ResourceSelectedMsg{Type: item.resourceType}
			}
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
			return m.moveDown(visible)
		case 'k':
			return m.moveUp(visible)
		case 'l':
			return m.activateResource(visible)
		case 'g':
			m.pendingG = true
			return m, nil
		case 'G':
			m.cursor = m.lastResourceIndex(visible)
			m.ensureCursorVisible()
			item := visible[m.cursor]
			m.selected = item.resourceType
			return m, func() tea.Msg {
				return ResourceSelectedMsg{Type: item.resourceType}
			}
		case 'd':
			return m.pageDown(visible)
		case 'u':
			return m.pageUp(visible)
		case '/':
			m.searching = true
			m.searchQuery = ""
			return m, nil
		}

	case tea.KeyDown:
		return m.moveDown(visible)
	case tea.KeyUp:
		return m.moveUp(visible)
	case tea.KeyEnter:
		return m.activateResource(visible)
	case tea.KeyEscape:
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.restoreCursorToSelected()
			return m, nil
		}
	}

	return m, nil
}

func (m SidebarModel) handleSearchKey(msg tea.KeyMsg) (SidebarModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.searching = false
		m.searchQuery = ""
		m.restoreCursorToSelected()
		return m, nil
	case msg.Type == tea.KeyEnter:
		m.searching = false
		visible := m.visibleItems()
		return m.activateResource(visible)
	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.resetCursorToFirstMatch()
		}
		return m, nil
	case msg.Type == tea.KeyDown:
		visible := m.visibleItems()
		return m.moveDown(visible)
	case msg.Type == tea.KeyUp:
		visible := m.visibleItems()
		return m.moveUp(visible)
	case msg.Type == tea.KeyRunes:
		for _, r := range msg.Runes {
			m.searchQuery += string(r)
		}
		m.resetCursorToFirstMatch()
		return m, nil
	}
	return m, nil
}

// restoreCursorToSelected finds m.selected in the current visible list and
// moves the cursor to it. Falls back to the first resource if not found.
func (m *SidebarModel) restoreCursorToSelected() {
	visible := m.visibleItems()
	for i, item := range visible {
		if !item.isCategory && item.resourceType == m.selected {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
	if len(visible) > 0 {
		m.cursor = m.firstResourceIndex(visible)
		m.ensureCursorVisible()
	}
}

func (m *SidebarModel) resetCursorToFirstMatch() {
	visible := m.visibleItems()
	for i, item := range visible {
		if !item.isCategory {
			m.cursor = i

			m.selected = item.resourceType
			return
		}
	}
}

func (m SidebarModel) handleMouse(msg tea.MouseMsg) (SidebarModel, tea.Cmd) {
	return m, nil
}

// moveDown moves the cursor to the next resource item, skipping categories.
func (m SidebarModel) moveDown(visible []visibleItem) (SidebarModel, tea.Cmd) {
	next := m.cursor + 1
	for next < len(visible) && visible[next].isCategory {
		next++
	}
	if next < len(visible) {
		m.cursor = next
		m.ensureCursorVisible()
		m.selected = visible[m.cursor].resourceType
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Type: visible[next].resourceType}
		}
	}
	return m, nil
}

// moveUp moves the cursor to the previous resource item, skipping categories.
func (m SidebarModel) moveUp(visible []visibleItem) (SidebarModel, tea.Cmd) {
	prev := m.cursor - 1
	for prev >= 0 && visible[prev].isCategory {
		prev--
	}
	if prev >= 0 {
		m.cursor = prev
		m.ensureCursorVisible()
		m.selected = visible[m.cursor].resourceType
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Type: visible[prev].resourceType}
		}
	}
	return m, nil
}

func (m SidebarModel) pageDown(visible []visibleItem) (SidebarModel, tea.Cmd) {
	half := m.viewportHeight() / 2
	if half < 1 {
		half = 1
	}
	target := m.cursor + half
	for target < len(visible) && visible[target].isCategory {
		target++
	}
	if target >= len(visible) {
		target = m.lastResourceIndex(visible)
	}
	if target != m.cursor {
		m.cursor = target
		m.ensureCursorVisible()
		m.selected = visible[m.cursor].resourceType
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Type: visible[m.cursor].resourceType}
		}
	}
	return m, nil
}

func (m SidebarModel) pageUp(visible []visibleItem) (SidebarModel, tea.Cmd) {
	half := m.viewportHeight() / 2
	if half < 1 {
		half = 1
	}
	target := m.cursor - half
	for target >= 0 && visible[target].isCategory {
		target--
	}
	if target < 0 {
		target = m.firstResourceIndex(visible)
	}
	if target != m.cursor {
		m.cursor = target
		m.ensureCursorVisible()
		m.selected = visible[m.cursor].resourceType
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Type: visible[m.cursor].resourceType}
		}
	}
	return m, nil
}

// activateResource selects the resource under cursor and emits ResourceSelectedMsg.
func (m SidebarModel) activateResource(visible []visibleItem) (SidebarModel, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(visible) {
		return m, nil
	}
	item := visible[m.cursor]
	if item.isCategory {
		// Categories are non-interactive — do nothing.
		return m, nil
	}
	m.selected = item.resourceType
	return m, func() tea.Msg {
		return ResourceSelectedMsg{Type: item.resourceType}
	}
}

// firstResourceIndex returns the index of the first non-category item.
func (m SidebarModel) firstResourceIndex(visible []visibleItem) int {
	for i, item := range visible {
		if !item.isCategory {
			return i
		}
	}
	return 0
}

// lastResourceIndex returns the index of the last non-category item.
func (m SidebarModel) lastResourceIndex(visible []visibleItem) int {
	for i := len(visible) - 1; i >= 0; i-- {
		if !visible[i].isCategory {
			return i
		}
	}
	return len(visible) - 1
}

// View implements tea.Model.
func (m SidebarModel) View() string {
	visible := m.visibleItems()
	if len(visible) == 0 {
		if m.searching || m.searchQuery != "" {
			return renderSearchBox(m.searchQuery, m.searching, m.width, m.theme)
		}
		return ""
	}

	baseStyle := m.theme.SidebarStyle()
	selectedStyle := m.theme.SidebarSelectedStyle()
	categoryStyle := m.theme.SidebarCategoryStyle()

	viewH := m.viewportHeight()
	end := m.scrollOffset + viewH
	if end > len(visible) {
		end = len(visible)
	}

	var lines []string
	for i := m.scrollOffset; i < end; i++ {
		item := visible[i]
		isCursor := i == m.cursor

		var line string
		if item.isCategory {
			line = categoryStyle.Width(m.width).Render(item.label)
		} else {
			label := "  " + item.label
			if isCursor && m.focused {
				line = selectedStyle.Width(m.width).Render(label)
			} else if isCursor {
				dimStyle := baseStyle.Bold(true).Width(m.width)
				line = dimStyle.Render(label)
			} else if item.resourceType == m.selected {
				selStyle := baseStyle.Bold(true).Width(m.width)
				line = selStyle.Render(label)
			} else {
				line = baseStyle.Width(m.width).Render(label)
			}
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	if m.searching || m.searchQuery != "" {
		return renderSearchBox(m.searchQuery, m.searching, m.width, m.theme) + "\n" + content
	}
	return content
}

// visibleItems computes the flat list of visible items, filtered by search query.
func (m SidebarModel) visibleItems() []visibleItem {
	query := strings.ToLower(m.searchQuery)
	var items []visibleItem
	for ci, cat := range m.categories {
		var children []visibleItem
		for ri, res := range cat.Items {
			if query != "" && !strings.Contains(strings.ToLower(res.Label), query) {
				continue
			}
			children = append(children, visibleItem{
				isCategory:    false,
				categoryIndex: ci,
				resourceIndex: ri,
				label:         res.Label,
				resourceType:  res.ResourceType,
			})
		}
		if len(children) > 0 || query == "" {
			items = append(items, visibleItem{
				isCategory:    true,
				categoryIndex: ci,
				resourceIndex: -1,
				label:         cat.Label,
			})
			items = append(items, children...)
		}
	}
	for _, res := range m.standalone {
		if query != "" && !strings.Contains(strings.ToLower(res.Label), query) {
			continue
		}
		items = append(items, visibleItem{
			isCategory:    false,
			categoryIndex: -1,
			resourceIndex: -1,
			label:         res.Label,
			resourceType:  res.ResourceType,
		})
	}
	return items
}

func (m *SidebarModel) ensureCursorVisible() {
	viewH := m.viewportHeight()
	if viewH <= 0 {
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
		if m.scrollOffset > 0 {
			visible := m.visibleItems()
			if m.scrollOffset-1 >= 0 && visible[m.scrollOffset-1].isCategory {
				m.scrollOffset--
			}
		}
	}
	if m.cursor >= m.scrollOffset+viewH {
		m.scrollOffset = m.cursor - viewH + 1
	}
}

func (m SidebarModel) viewportHeight() int {
	h := m.height
	if m.searching || m.searchQuery != "" {
		h -= 3
	}
	if h < 1 {
		h = 1
	}
	return h
}

// SetSize sets the dimensions of the sidebar panel.
func (m *SidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether the sidebar is focused.
func (m *SidebarModel) SetFocused(focused bool) {
	m.focused = focused
}

// Selected returns the currently selected resource type.
func (m SidebarModel) Selected() k8s.ResourceType {
	return m.selected
}

// ScrollInfo returns the current cursor position among resources (non-category items).
func (m SidebarModel) ScrollInfo() *ScrollInfo {
	visible := m.visibleItems()
	var resourceIdx, totalResources int
	for i, item := range visible {
		if item.isCategory {
			continue
		}
		totalResources++
		if i == m.cursor {
			resourceIdx = totalResources
		}
	}
	if totalResources == 0 {
		return nil
	}
	return &ScrollInfo{Position: resourceIdx, Total: totalResources}
}
