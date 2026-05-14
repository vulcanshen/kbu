package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// Column defines a table column with a title and minimum width.
type Column struct {
	Title    string
	MinWidth int
}

// TableModel is the Bubble Tea model for the resource table panel.
type TableModel struct {
	columns      []Column
	rows         [][]string
	cursor       int
	scrollOffset int
	focused      bool
	width        int
	height       int
	theme        *theme.Theme
	pendingG     bool
	resourceType k8s.ResourceType
}

// ColumnsForResource returns the column definitions for a given resource type.
func ColumnsForResource(rt k8s.ResourceType) []Column {
	switch rt {
	case k8s.ResourceNamespaces:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Status", MinWidth: 10},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceNodes:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Status", MinWidth: 10},
			{Title: "Roles", MinWidth: 12},
			{Title: "Version", MinWidth: 10},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourcePods:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Ready", MinWidth: 7},
			{Title: "Status", MinWidth: 10},
			{Title: "Restarts", MinWidth: 10},
			{Title: "Age", MinWidth: 8},
			{Title: "Node", MinWidth: 15},
		}
	case k8s.ResourceDeployments:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Ready", MinWidth: 7},
			{Title: "Up-to-date", MinWidth: 12},
			{Title: "Available", MinWidth: 10},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceDaemonSets:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Desired", MinWidth: 8},
			{Title: "Current", MinWidth: 8},
			{Title: "Ready", MinWidth: 7},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceStatefulSets:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Ready", MinWidth: 7},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceJobs:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Completions", MinWidth: 12},
			{Title: "Duration", MinWidth: 10},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceCronJobs:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Schedule", MinWidth: 15},
			{Title: "Suspend", MinWidth: 8},
			{Title: "Active", MinWidth: 7},
			{Title: "Last Schedule", MinWidth: 15},
		}
	case k8s.ResourceServices:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Type", MinWidth: 12},
			{Title: "Cluster-IP", MinWidth: 15},
			{Title: "External-IP", MinWidth: 15},
			{Title: "Ports", MinWidth: 15},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceIngresses:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Class", MinWidth: 10},
			{Title: "Hosts", MinWidth: 20},
			{Title: "Ports", MinWidth: 10},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceConfigMaps:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Data", MinWidth: 6},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceSecrets:
		return []Column{
			{Title: "Name", MinWidth: 20},
			{Title: "Type", MinWidth: 20},
			{Title: "Data", MinWidth: 6},
			{Title: "Age", MinWidth: 8},
		}
	case k8s.ResourceEvents:
		return []Column{
			{Title: "Type", MinWidth: 8},
			{Title: "Reason", MinWidth: 15},
			{Title: "Object", MinWidth: 20},
			{Title: "Message", MinWidth: 30},
			{Title: "Age", MinWidth: 8},
		}
	default:
		return []Column{
			{Title: "Name", MinWidth: 20},
		}
	}
}

// NewTableModel creates a new table model initialized with Pods columns.
func NewTableModel(t *theme.Theme) TableModel {
	return TableModel{
		columns:      ColumnsForResource(k8s.ResourcePods),
		rows:         nil,
		cursor:       0,
		scrollOffset: 0,
		focused:      false,
		width:        80,
		height:       20,
		theme:        t,
		pendingG:     false,
		resourceType: k8s.ResourcePods,
	}
}

// Init implements tea.Model.
func (m TableModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m TableModel) Update(msg tea.Msg) (TableModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ResourceSelectedMsg:
		m.resourceType = msg.Type
		m.columns = ColumnsForResource(msg.Type)
		m.rows = nil
		m.cursor = 0
		m.scrollOffset = 0
		m.pendingG = false
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

func (m TableModel) handleKey(msg tea.KeyMsg) (TableModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		m.pendingG = false
		if len(m.rows) > 0 && m.cursor < len(m.rows)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}

	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		m.pendingG = false
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "G":
		m.pendingG = false
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
			m.ensureCursorVisible()
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "g":
		if m.pendingG {
			m.cursor = 0
			m.scrollOffset = 0
			m.pendingG = false
		} else {
			m.pendingG = true
		}

	default:
		m.pendingG = false
	}

	return m, nil
}

func (m TableModel) handleMouse(msg tea.MouseMsg) (TableModel, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	case tea.MouseWheelDown:
		maxOffset := len(m.rows) - m.visibleRows()
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.scrollOffset < maxOffset {
			m.scrollOffset++
		}
	}
	return m, nil
}

func (m *TableModel) ensureCursorVisible() {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

// View implements tea.Model.
func (m TableModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	colWidths := m.calcColumnWidths()

	var b strings.Builder

	// Render header — bright when focused, dim when not
	headerStyle := m.theme.TableHeaderStyle()
	if !m.focused {
		headerStyle = m.theme.TableRowStyle().Bold(true)
	}
	header := m.renderRow(colWidths, m.columnTitles(), headerStyle)
	b.WriteString(header)
	b.WriteString("\n")

	// Render visible rows
	visible := m.visibleRows()
	if visible <= 0 {
		return b.String()
	}

	end := m.scrollOffset + visible
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.scrollOffset; i < end; i++ {
		var style lipgloss.Style
		if i == m.cursor && m.focused {
			style = m.theme.TableSelectedRowStyle()
		} else if i == m.cursor {
			style = m.theme.TableRowStyle().Bold(true)
		} else if i%2 == 0 {
			style = m.theme.TableAlternatingRowStyle()
		} else {
			style = m.theme.TableRowStyle()
		}
		row := m.renderRow(colWidths, m.rows[i], style)
		b.WriteString(row)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Show scroll indicator if there are rows outside viewport
	if len(m.rows) > visible {
		remaining := len(m.rows) - end
		if remaining > 0 {
			b.WriteString("\n")
			indicator := fmt.Sprintf(" ↓ %d more rows", remaining)
			b.WriteString(m.theme.TableRowStyle().Render(indicator))
		}
	}

	return b.String()
}

func (m TableModel) columnTitles() []string {
	titles := make([]string, len(m.columns))
	for i, col := range m.columns {
		titles[i] = col.Title
	}
	return titles
}

func (m TableModel) calcColumnWidths() []int {
	if len(m.columns) == 0 {
		return nil
	}

	totalMin := 0
	for _, col := range m.columns {
		totalMin += col.MinWidth
	}

	widths := make([]int, len(m.columns))
	available := m.width

	if totalMin == 0 {
		// Equal distribution
		each := available / len(m.columns)
		for i := range widths {
			widths[i] = each
		}
		return widths
	}

	// Distribute proportionally based on MinWidth
	for i, col := range m.columns {
		widths[i] = col.MinWidth * available / totalMin
	}

	// Distribute remaining pixels to avoid rounding loss
	used := 0
	for _, w := range widths {
		used += w
	}
	remainder := available - used
	for i := 0; i < remainder && i < len(widths); i++ {
		widths[i]++
	}

	return widths
}

func (m TableModel) renderRow(colWidths []int, values []string, style lipgloss.Style) string {
	var parts []string
	for i, w := range colWidths {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		// Pad or truncate
		if len(val) > w {
			if w > 1 {
				val = val[:w-1] + "…"
			} else if w > 0 {
				val = val[:w]
			} else {
				val = ""
			}
		} else {
			val = val + strings.Repeat(" ", w-len(val))
		}
		parts = append(parts, val)
	}

	line := strings.Join(parts, " ")

	// Ensure the line fills the full width
	lineLen := lipgloss.Width(line)
	if lineLen < m.width {
		line = line + strings.Repeat(" ", m.width-lineLen)
	}

	return style.Render(line)
}

// SetSize sets the table dimensions.
func (m *TableModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether the table is focused.
func (m *TableModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetRows sets the table rows.
func (m *TableModel) SetRows(rows [][]string) {
	m.rows = rows
	if m.cursor >= len(m.rows) && len(m.rows) > 0 {
		m.cursor = len(m.rows) - 1
	}
	if len(m.rows) == 0 {
		m.cursor = 0
		m.scrollOffset = 0
	}
}

// SelectedRow returns the current cursor position.
func (m TableModel) SelectedRow() int {
	return m.cursor
}

// visibleRows returns how many rows fit in the viewport (height minus header).
func (m TableModel) visibleRows() int {
	v := m.height - 1 // 1 line for header
	if v < 0 {
		return 0
	}
	return v
}
