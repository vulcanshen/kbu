package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// SidebarCategory represents a collapsible group of resource types.
type SidebarCategory struct {
	Label    string
	Expanded bool
	Items    []SidebarResource
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
	scrollOffset int
	theme        *theme.Theme
	pendingG     bool
	selected     k8s.ResourceType
}

// NewSidebarModel creates a new sidebar with default categories and resources.
func NewSidebarModel(t *theme.Theme) SidebarModel {
	categories := []SidebarCategory{
		{
			Label:    "Cluster",
			Expanded: true,
			Items: []SidebarResource{
				{Label: "Namespaces", ResourceType: k8s.ResourceNamespaces},
				{Label: "Nodes", ResourceType: k8s.ResourceNodes},
			},
		},
		{
			Label:    "Workloads",
			Expanded: true,
			Items: []SidebarResource{
				{Label: "Pods", ResourceType: k8s.ResourcePods},
				{Label: "Deployments", ResourceType: k8s.ResourceDeployments},
				{Label: "DaemonSets", ResourceType: k8s.ResourceDaemonSets},
				{Label: "StatefulSets", ResourceType: k8s.ResourceStatefulSets},
				{Label: "Jobs", ResourceType: k8s.ResourceJobs},
				{Label: "CronJobs", ResourceType: k8s.ResourceCronJobs},
			},
		},
		{
			Label:    "Network",
			Expanded: true,
			Items: []SidebarResource{
				{Label: "Services", ResourceType: k8s.ResourceServices},
				{Label: "Ingresses", ResourceType: k8s.ResourceIngresses},
			},
		},
		{
			Label:    "Config",
			Expanded: true,
			Items: []SidebarResource{
				{Label: "ConfigMaps", ResourceType: k8s.ResourceConfigMaps},
				{Label: "Secrets", ResourceType: k8s.ResourceSecrets},
			},
		},
	}

	standalone := []SidebarResource{
		{Label: "Events", ResourceType: k8s.ResourceEvents},
	}

	return SidebarModel{
		categories: categories,
		standalone: standalone,
		cursor:     0,
		focused:    false,
		selected:   k8s.ResourcePods,
		theme:      t,
	}
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
	visible := m.visibleItems()
	if len(visible) == 0 {
		return m, nil
	}

	// Handle pending g state for gg combo.
	if m.pendingG {
		m.pendingG = false
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'g' {
			m.cursor = 0
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
			return m.moveDown(visible), nil
		case 'k':
			return m.moveUp(visible), nil
		case 'l':
			return m.activate(visible)
		case 'h':
			return m.collapseOrParent(visible), nil
		case 'g':
			m.pendingG = true
			return m, nil
		case 'G':
			m.cursor = len(visible) - 1
			m.ensureCursorVisible()
			return m, nil
		}

	case tea.KeyDown:
		return m.moveDown(visible), nil
	case tea.KeyUp:
		return m.moveUp(visible), nil
	case tea.KeyEnter:
		return m.activate(visible)
	}

	return m, nil
}

func (m SidebarModel) handleMouse(msg tea.MouseMsg) (SidebarModel, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case tea.MouseWheelDown:
		visible := m.visibleItems()
		maxOffset := len(visible) - m.height
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.scrollOffset < maxOffset {
			m.scrollOffset++
		}
	}
	return m, nil
}

func (m SidebarModel) moveDown(visible []visibleItem) SidebarModel {
	if m.cursor < len(visible)-1 {
		m.cursor++
		m.ensureCursorVisible()
	}
	return m
}

func (m SidebarModel) moveUp(visible []visibleItem) SidebarModel {
	if m.cursor > 0 {
		m.cursor--
		m.ensureCursorVisible()
	}
	return m
}

func (m *SidebarModel) ensureCursorVisible() {
	if m.height <= 0 {
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+m.height {
		m.scrollOffset = m.cursor - m.height + 1
	}
}

func (m SidebarModel) activate(visible []visibleItem) (SidebarModel, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(visible) {
		return m, nil
	}
	item := visible[m.cursor]
	if item.isCategory {
		// Toggle expand/collapse.
		m.categories[item.categoryIndex].Expanded = !m.categories[item.categoryIndex].Expanded
		// After collapsing, clamp cursor to visible range.
		newVisible := m.visibleItems()
		if m.cursor >= len(newVisible) {
			m.cursor = len(newVisible) - 1
		}
		return m, nil
	}

	// Resource item — select it and send message.
	m.selected = item.resourceType
	return m, func() tea.Msg {
		return ResourceSelectedMsg{Type: item.resourceType}
	}
}

func (m SidebarModel) collapseOrParent(visible []visibleItem) SidebarModel {
	if m.cursor < 0 || m.cursor >= len(visible) {
		return m
	}
	item := visible[m.cursor]
	if item.isCategory {
		// Collapse this category.
		if m.categories[item.categoryIndex].Expanded {
			m.categories[item.categoryIndex].Expanded = false
		}
		return m
	}

	// Resource item — collapse parent category and move cursor to it.
	if item.categoryIndex >= 0 {
		m.categories[item.categoryIndex].Expanded = false
		// Move cursor to the category header.
		newVisible := m.visibleItems()
		for i, v := range newVisible {
			if v.isCategory && v.categoryIndex == item.categoryIndex {
				m.cursor = i
				break
			}
		}
	}
	return m
}

// View implements tea.Model.
func (m SidebarModel) View() string {
	visible := m.visibleItems()
	if len(visible) == 0 {
		return ""
	}

	baseStyle := m.theme.SidebarStyle()
	selectedStyle := m.theme.SidebarSelectedStyle()
	categoryStyle := m.theme.SidebarCategoryStyle()

	// Compute the content width (inside border).
	contentWidth := m.width - 1 // 1 for right border
	if contentWidth < 1 {
		contentWidth = 1
	}

	var lines []string
	start := m.scrollOffset
	end := start + m.height
	if end > len(visible) {
		end = len(visible)
	}

	for i := start; i < end; i++ {
		item := visible[i]
		isCursor := i == m.cursor

		var line string
		if item.isCategory {
			arrow := "▼"
			if !m.categories[item.categoryIndex].Expanded {
				arrow = "▶"
			}
			label := fmt.Sprintf("%s %s", arrow, item.label)
			if isCursor && m.focused {
				line = selectedStyle.Width(contentWidth).Render(label)
			} else if isCursor {
				// Unfocused but cursor — dimmer highlight.
				dimStyle := baseStyle.Bold(true).Width(contentWidth)
				line = dimStyle.Render(label)
			} else {
				line = categoryStyle.Width(contentWidth).Render(label)
			}
		} else {
			label := "  " + item.label
			if isCursor && m.focused {
				line = selectedStyle.Width(contentWidth).Render(label)
			} else if isCursor {
				dimStyle := baseStyle.Bold(true).Width(contentWidth)
				line = dimStyle.Render(label)
			} else if item.resourceType == m.selected {
				// Currently selected resource gets subtle highlight.
				selStyle := baseStyle.Bold(true).Width(contentWidth)
				line = selStyle.Render(label)
			} else {
				line = baseStyle.Width(contentWidth).Render(label)
			}
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	borderColor := m.theme.Detail.BorderColor
	if m.focused {
		borderColor = m.theme.Sidebar.CategoryFg
	}

	borderStyle := lipgloss.NewStyle().
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Height(m.height).
		Width(contentWidth)

	return borderStyle.Render(content)
}

// visibleItems computes the flat list of visible items based on expanded state.
func (m SidebarModel) visibleItems() []visibleItem {
	var items []visibleItem
	for ci, cat := range m.categories {
		items = append(items, visibleItem{
			isCategory:    true,
			categoryIndex: ci,
			resourceIndex: -1,
			label:         cat.Label,
		})
		if cat.Expanded {
			for ri, res := range cat.Items {
				items = append(items, visibleItem{
					isCategory:    false,
					categoryIndex: ci,
					resourceIndex: ri,
					label:         res.Label,
					resourceType:  res.ResourceType,
				})
			}
		}
	}
	// Standalone items.
	for _, res := range m.standalone {
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
