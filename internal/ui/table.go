package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

// Column is an alias for k8s.Column for backward compatibility within the UI package.
type Column = k8s.Column

// TableModel is the Bubble Tea model for the resource table panel.
type TableModel struct {
	columns         []Column
	rows            [][]string
	allRows         [][]string
	filteredIndices []int
	cursor          int
	scrollOffset    int
	focused         bool
	width           int
	height          int
	theme           *theme.Theme
	pendingG        bool
	resourceType    k8s.ResourceType
	searching       bool
	searchQuery     string
}

// ColumnsForResource returns the column definitions for a given resource type.
func ColumnsForResource(rt k8s.ResourceType) []Column {
	return k8s.DefaultRegistry.ColumnsFor(rt)
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
		m.allRows = nil
		m.filteredIndices = nil
		m.cursor = 0
		m.scrollOffset = 0
		m.pendingG = false
		m.searching = false
		m.searchQuery = ""
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
	if m.searching {
		return m.handleSearchKey(msg)
	}

	switch {
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		m.pendingG = false
		if len(m.rows) > 0 && m.cursor < len(m.rows)-1 {
			m.cursor++
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		m.pendingG = false
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "G":
		m.pendingG = false
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "g":
		if m.pendingG {
			m.cursor = 0
			m.scrollOffset = 0
			m.pendingG = false
			return m, m.emitCursorChanged()
		} else {
			m.pendingG = true
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "d":
		m.pendingG = false
		if len(m.rows) > 0 {
			half := m.visibleRows() / 2
			if half < 1 {
				half = 1
			}
			m.cursor += half
			if m.cursor >= len(m.rows) {
				m.cursor = len(m.rows) - 1
			}
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "u":
		m.pendingG = false
		if len(m.rows) > 0 {
			half := m.visibleRows() / 2
			if half < 1 {
				half = 1
			}
			m.cursor -= half
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "/":
		m.pendingG = false
		m.searching = true
		m.searchQuery = ""
		m.filterRows()
		return m, nil

	case msg.Type == tea.KeyEnter:
		m.pendingG = false
		if len(m.rows) > 0 {
			return m, func() tea.Msg {
				return RowSelectedMsg{Index: m.OriginalIndex(m.cursor)}
			}
		}

	case msg.Type == tea.KeyEscape:
		if m.searchQuery != "" {
			m.pendingG = false
			orig := m.OriginalIndex(m.cursor)
			m.searchQuery = ""
			m.filterRows()
			m.cursor = orig
			m.ensureCursorVisible()
			return m, nil
		}

	default:
		m.pendingG = false
	}

	return m, nil
}

func (m TableModel) handleSearchKey(msg tea.KeyMsg) (TableModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.searching = false
		orig := m.OriginalIndex(m.cursor)
		m.searchQuery = ""
		m.filterRows()
		m.cursor = orig
		m.ensureCursorVisible()
		return m, nil

	case msg.Type == tea.KeyEnter:
		m.searching = false
		return m, nil

	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.filterRows()
		}
		return m, nil

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		if len(m.rows) > 0 && m.cursor < len(m.rows)-1 {
			m.cursor++
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyRunes:
		for _, r := range msg.Runes {
			m.searchQuery += string(r)
		}
		m.filterRows()
		return m, nil
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

	if m.searching || m.searchQuery != "" {
		b.WriteString(renderSearchBox(m.searchQuery, m.searching, m.width, m.theme))
		b.WriteString("\n")
	}

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

func (m TableModel) emitCursorChanged() tea.Cmd {
	idx := m.OriginalIndex(m.cursor)
	return func() tea.Msg {
		return RowSelectedMsg{Index: idx}
	}
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
	separators := len(m.columns) - 1
	available := m.width - separators

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

// SetColumns replaces the column definitions and resets the table state.
func (m *TableModel) SetColumns(cols []Column) {
	m.columns = cols
	m.rows = nil
	m.allRows = nil
	m.filteredIndices = nil
	m.cursor = 0
	m.scrollOffset = 0
	m.searching = false
	m.searchQuery = ""
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

// SetRows sets the table rows, storing in allRows and applying the current filter.
func (m *TableModel) SetRows(rows [][]string) {
	m.allRows = rows
	m.filterRows()
}

// filterRows filters allRows by searchQuery and updates rows and filteredIndices.
func (m *TableModel) filterRows() {
	if m.searchQuery == "" {
		m.rows = m.allRows
		m.filteredIndices = nil
	} else {
		query := strings.ToLower(m.searchQuery)
		m.rows = nil
		m.filteredIndices = nil
		for i, row := range m.allRows {
			for _, col := range row {
				if strings.Contains(strings.ToLower(col), query) {
					m.rows = append(m.rows, row)
					m.filteredIndices = append(m.filteredIndices, i)
					break
				}
			}
		}
	}

	if m.cursor >= len(m.rows) && len(m.rows) > 0 {
		m.cursor = len(m.rows) - 1
	}
	if len(m.rows) == 0 {
		m.cursor = 0
		m.scrollOffset = 0
	}
}

// OriginalIndex returns the original index into allRows for the given display index.
// HasActiveFilter returns true if a search filter is active.
func (m TableModel) HasActiveFilter() bool { return m.searchQuery != "" }

// When no filter is active, the display index equals the original index.
func (m TableModel) OriginalIndex(displayIdx int) int {
	if m.filteredIndices == nil {
		return displayIdx
	}
	if displayIdx >= 0 && displayIdx < len(m.filteredIndices) {
		return m.filteredIndices[displayIdx]
	}
	return displayIdx
}

// SelectedRow returns the original index of the currently selected row.
func (m TableModel) SelectedRow() int {
	return m.OriginalIndex(m.cursor)
}

// IsSearching returns whether the table is in search mode.
func (m TableModel) IsSearching() bool {
	return m.searching
}

// SearchQuery returns the current search query.
func (m TableModel) SearchQuery() string {
	return m.searchQuery
}

// ScrollInfo returns the current cursor position and total row count.
func (m TableModel) ScrollInfo() *ScrollInfo {
	if len(m.rows) == 0 {
		return nil
	}
	return &ScrollInfo{Position: m.cursor + 1, Total: len(m.rows)}
}

// visibleRows returns how many rows fit in the viewport (height minus header and search bar).
func (m TableModel) visibleRows() int {
	v := m.height - 1 // 1 line for header
	if m.searching || m.searchQuery != "" {
		v -= 3 // 3 lines for search box (border + input + border)
	}
	if v < 0 {
		return 0
	}
	return v
}
