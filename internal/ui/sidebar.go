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

			item := visible[m.cursor]
			m.selected = item.resourceType
			return m, func() tea.Msg {
				return ResourceSelectedMsg{Type: item.resourceType}
			}
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
			newVisible := m.visibleItems()
			if m.cursor >= len(newVisible) && len(newVisible) > 0 {
				m.cursor = m.firstResourceIndex(newVisible)
			}
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
				visible := m.visibleItems()
		if m.cursor >= len(visible) && len(visible) > 0 {
			m.cursor = m.firstResourceIndex(visible)
		}
		return m, nil
	case msg.Type == tea.KeyEnter:
		m.searching = false

		return m, nil
	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.resetCursorToFirstMatch()
		}
		return m, nil
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		visible := m.visibleItems()
		return m.moveDown(visible)
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
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

		m.selected = visible[m.cursor].resourceType
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Type: visible[next].resourceType}
		}
	}
	// No more resource items below — stay put.
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

		m.selected = visible[m.cursor].resourceType
		return m, func() tea.Msg {
			return ResourceSelectedMsg{Type: visible[prev].resourceType}
		}
	}
	// No more resource items above — stay put.
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

	var lines []string
	for i := 0; i < len(visible); i++ {
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
