package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vulcanshen/km8/internal/config"
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

	// Compare-mode lock highlight. lockedRow is the (post-filter) row
	// index that should render with the compare-baseline background.
	// -1 = no lock. AppModel re-resolves the index whenever rows change
	// (filter / watcher / drill-down) and calls SetLockedRow.
	lockedRow int

	// sortChain mirrors the kind's saved sort chain, one entry per
	// sorted column (tier 0 = primary). Drives the panel-2 header:
	// each sorted column renders its title + priority badge
	// "(N)" + direction arrow. Empty chain = no badge anywhere.
	// AppModel calls SetSortIndicators on init, on kind switch, and
	// on every commit so the header stays in lock-step with config.
	sortChain []config.SortConfig
}

// CopyableContent returns the current (filtered) table rows as plain text
// with the header. Columns are space-padded for readability when pasted.
// Used by the global `y` key.
func (m TableModel) CopyableContent() string {
	if len(m.rows) == 0 {
		return ""
	}
	widths := make([]int, len(m.columns))
	for i, c := range m.columns {
		widths[i] = len(c.Title)
	}
	for _, row := range m.rows {
		for i := 0; i < len(widths) && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	formatRow := func(cells []string) string {
		parts := make([]string, len(widths))
		for i := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			if i == len(widths)-1 {
				parts[i] = cell
			} else {
				parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
			}
		}
		return strings.Join(parts, "  ")
	}
	header := make([]string, len(m.columns))
	for i, c := range m.columns {
		header[i] = c.Title
	}
	lines := []string{formatRow(header)}
	for _, row := range m.rows {
		lines = append(lines, formatRow(row))
	}
	return strings.Join(lines, "\n")
}

// ColumnsForResource returns the column definitions for a given resource
// type. Inserts an unlabeled "helm-marker" column right after Name on
// every resource except Helm Releases (where every row is helm by
// definition — the dedicated CHART / REV / STATUS columns carry the
// release context already).
func ColumnsForResource(rt k8s.ResourceType) []Column {
	cols := k8s.DefaultRegistry.ColumnsFor(rt)
	if rt == k8s.ResourceReleases || len(cols) == 0 {
		return cols
	}
	out := make([]Column, 0, len(cols)+1)
	out = append(out, cols[0])
	// Always-present empty column even for non-helm rows, so column
	// alignment stays consistent. Cell value is ASCII (HelmRowMark)
	// to avoid runewidth ambiguity drift across rows.
	out = append(out, Column{Title: "", MinWidth: 2})
	out = append(out, cols[1:]...)
	return out
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
		lockedRow:    -1,
	}
}

// SetCursorAtScreenY moves the cursor to the data row a mouse click
// landed on. screenY is counted from the panel's top border (0 =
// top border, 1+ = panel content rows).
//
// Layout the math walks through, top-down:
//   - line 0:           panel top border (no row, no-op)
//   - lines 1..3:       optional search box (renderSearchBox emits
//     a 3-line bordered box, only when the user is
//     in search mode or has a sticky filter)
//   - next line:        column header (no row, no-op)
//   - subsequent lines: data rows
//
// Returns the same RowSelectedMsg-emitting cmd as keyboard j/k so the
// detail panel re-fetches for the clicked row. Clicks on the border,
// header, search box, or any out-of-range row are silent no-ops.
func (m *TableModel) SetCursorAtScreenY(screenY int) tea.Cmd {
	contentY := screenY - 1 // skip the top border
	if contentY < 0 {
		return nil
	}
	if m.searching || m.searchQuery != "" {
		contentY -= 3 // skip the 3-line search-box header
		if contentY < 0 {
			return nil
		}
	}
	if contentY <= 0 {
		return nil // column header (always present)
	}
	rowIdx := m.scrollOffset + (contentY - 1)
	if rowIdx < 0 || rowIdx >= len(m.rows) {
		return nil
	}
	m.cursor = rowIdx
	return m.emitCursorChanged()
}

// SetLockedRow marks the row at (post-filter) index `idx` as the
// compare-mode baseline. -1 clears the highlight. Caller (AppModel)
// owns the lock identity (by UID) and re-resolves the index whenever
// rows change.
func (m *TableModel) SetLockedRow(idx int) {
	m.lockedRow = idx
}

// SetSortIndicators declares which columns the panel-2 header
// should badge — one entry per tier, index = priority. Empty / nil
// chain clears all badges. The table renders priority "(N)" +
// direction arrow on each matching column header; non-matching
// columns stay plain. Defensive: stale config column names that
// no longer map to any header render harmlessly (no badge).
func (m *TableModel) SetSortIndicators(chain []config.SortConfig) {
	if len(chain) == 0 {
		m.sortChain = nil
		return
	}
	m.sortChain = append(m.sortChain[:0], chain...)
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
		// Enter on a non-drillable table or inside a drill view used to
		// fall back to "focus panel 3". With mouse double-click → Enter
		// synthesis that fallback would have hijacked the user's focus
		// every time they double-clicked a row. Removed: app.go's
		// "case enter" still routes Enter to enterDrillDown when the
		// kind is drillable; for non-drillable kinds Enter is a
		// deliberate no-op now.
		m.pendingG = false

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
		// Search commits — leave search mode, keep the filter active.
		// Previously this also auto-focused panel 3; that focus jump
		// got removed when the broader Enter-as-focus fallback went
		// away (avoids double-click → Enter accidentally shifting
		// focus). j/k inside search already kept detail in sync via
		// emitCursorChanged, so no extra row-load needed here.
		m.searching = false
		return m, nil

	case msg.Type == tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.filterRows()
		}
		return m, nil

	case msg.Type == tea.KeyDown:
		if len(m.rows) > 0 && m.cursor < len(m.rows)-1 {
			m.cursor++
			m.ensureCursorVisible()
			return m, m.emitCursorChanged()
		}

	case msg.Type == tea.KeyUp:
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

	// Render header — bright when focused, drop to overlay0 grey + bold
	// when unfocused so it recedes with the rest of the panel chrome,
	// matching the sidebar's category-header treatment.
	headerStyle := m.theme.TableHeaderStyle()
	if !m.focused {
		headerStyle = m.theme.TableDimRowStyle().Bold(true)
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

	dimRowStyle := m.theme.TableDimRowStyle()
	for i := m.scrollOffset; i < end; i++ {
		var style lipgloss.Style
		isLocked := i == m.lockedRow
		switch {
		case i == m.cursor && m.focused:
			style = m.theme.TableSelectedRowStyle()
		case i == m.cursor:
			// Lavender chip — single strong "remembered position"
			// marker against the dimmed surrounding rows.
			style = m.theme.TableUnfocusedSelectedRowStyle()
		case isLocked:
			// Compare-mode lock background — same #9DDAEA cyan as the
			// status-bar marker so the two signals visually connect.
			// Stays full color even when the table is unfocused
			// because compare is an intentional user state.
			style = lipgloss.NewStyle().
				Background(lipgloss.Color("#9DDAEA")).
				Foreground(lipgloss.Color("#1e1e2e"))
		case !m.focused:
			// Unfocused → flatten alternating-row striping into a
			// single dim color so the cursor chip is the only
			// surviving signal in the panel.
			style = dimRowStyle
		case i%2 == 0:
			style = m.theme.TableAlternatingRowStyle()
		default:
			style = m.theme.TableRowStyle()
		}
		row := m.renderRow(colWidths, m.rows[i], style)
		// Cursor sitting on top of the locked row keeps the reverse-
		// video cursor style — the user's cursor position is the more
		// transient signal (changes every j/k), the lock is fixed and
		// already surfaced in the status-bar marker. Combining the two
		// visual cues on one row reads as visual noise.
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
	// Build a lookup so each header render is O(1): column title →
	// tier index in the sort chain. Multi-tier columns get a
	// priority badge "(N)" + arrow; non-sorted columns stay plain.
	tierByTitle := make(map[string]int, len(m.sortChain))
	for i, c := range m.sortChain {
		if c.Column != "" {
			tierByTitle[c.Column] = i
		}
	}
	for i, col := range m.columns {
		title := col.Title
		if title == "" {
			titles[i] = title
			continue
		}
		if idx, ok := tierByTitle[title]; ok {
			c := m.sortChain[idx]
			var arrow string
			switch c.Direction {
			case "asc":
				arrow = sortAscendingGlyph
			case "desc":
				arrow = sortDescendingGlyph
			}
			// Only show "(N)" when the chain has more than one
			// tier — single-column case stays visually simple
			// (no parens) which matches the v1.6 look.
			if len(m.sortChain) > 1 {
				title = fmt.Sprintf("%s (%d) %s", title, idx+1, arrow)
			} else if arrow != "" {
				title = title + " " + arrow
			}
		}
		titles[i] = title
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
	// Apply the row style to each cell + separator + trailing padding
	// rather than wrapping the whole line. Per-cell ANSI dispatch was
	// originally there so the Pod-Status reset (\x1b[0m, emitted after
	// the colored STATUS span) wouldn't kill the row style for the rest
	// of the row. Status-column coloring was removed in v1.7.x for
	// cross-kind consistency; per-cell wrapping stays in case a future
	// per-column highlight returns.
	var parts []string
	for i, w := range colWidths {
		val := ""
		if i < len(values) {
			val = values[i]
		}
		// Visual-width-aware truncation. The old code used byte-based
		// len(val) and val[:w-1], which sliced UTF-8 mid-codepoint for any
		// multi-byte content — e.g. the Nerd Font helm glyph "" is
		// 3 bytes / 1 cell, so a 2-cell column would slice the first byte
		// and render \xee as ◇. ansi.Truncate is grapheme- and ANSI-aware
		// and accounts for wide characters; visual width comes from
		// lipgloss.Width for the padding side.
		if vw := lipgloss.Width(val); vw > w {
			if w >= 1 {
				val = ansi.Truncate(val, w, "…")
			} else {
				val = ""
			}
			vw = lipgloss.Width(val)
			if vw < w {
				val = val + strings.Repeat(" ", w-vw)
			}
		} else {
			val = val + strings.Repeat(" ", w-vw)
		}
		val = style.Render(val)
		parts = append(parts, val)
	}

	sep := style.Render(" ")
	line := strings.Join(parts, sep)

	// Trailing pad to full row width, also row-styled so the highlight
	// stretches edge-to-edge.
	lineLen := lipgloss.Width(line)
	if lineLen < m.width {
		line = line + style.Render(strings.Repeat(" ", m.width-lineLen))
	}

	return line
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

// ClearSearch drops any active table search filter + exits search-input
// mode. Used by AppModel when focus leaves the table panel — search is a
// transient navigation aid, not a persistent view state.
//
// Mirrors the in-panel Esc behavior: convert the filtered cursor back to
// its position in the unfiltered list, drop the filter, then ensure the
// cursor stays visible. Without filterRows() the rows slice would stay
// stuck on the filtered subset even though searchQuery is cleared —
// symptom is "search box gone but table still only shows 1 row".
func (m *TableModel) ClearSearch() {
	if m.searchQuery == "" && !m.searching {
		return
	}
	orig := m.OriginalIndex(m.cursor)
	m.searching = false
	m.searchQuery = ""
	m.filterRows()
	if orig >= 0 && orig < len(m.rows) {
		m.cursor = orig
	}
	m.ensureCursorVisible()
}

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

// SetCursor moves the cursor to the row at `originalIdx` (index into the
// unfiltered allRows). When a search filter is active, the cursor jumps
// to the matching position in the filtered view; if the target row is
// filtered out, the cursor stays put. Out-of-range indices are ignored.
// Used by the Relatives-tab "space — jump to this resource" flow once the
// new resource type's items arrive.
func (m *TableModel) SetCursor(originalIdx int) {
	if originalIdx < 0 || originalIdx >= len(m.allRows) {
		return
	}
	if m.filteredIndices == nil {
		m.cursor = originalIdx
		m.ensureCursorVisible()
		return
	}
	for i, orig := range m.filteredIndices {
		if orig == originalIdx {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
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
