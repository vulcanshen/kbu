package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	categoryIndex int // index into categories slice (-1 standalone, -2 Pinned virtual category)
	resourceIndex int // index into category's Items slice (-1 for category header)
	label         string
	resourceType  k8s.ResourceType
}

// pinnedCategoryIndex is the sentinel value for visibleItem.categoryIndex
// signalling "this item lives in the virtual Pinned category at the top
// of the sidebar". -1 was already taken by standalone resources; -2 is
// unambiguous and stable. Used by CursorPinned to recognise the Pinned
// section without a separate boolean flag — every kind appears in
// exactly one location, so the categoryIndex alone disambiguates.
const pinnedCategoryIndex = -2

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

	// pinned is the ordered list of resource kinds the user has pinned
	// to the top of the sidebar (Order value from config drives this
	// slice; sidebar internally just sees a position). Rendered as a
	// virtual "Pinned" category prepended to visibleItems(). Empty =
	// the Pinned category does not appear at all (no empty placeholder).
	//
	// A pinned kind is REMOVED from its original category — each kind
	// has exactly one location in the sidebar. Original category
	// headers hide when all their kinds get pinned out. This makes the
	// cursor logic simpler (snap by kind, no "which copy") and saves
	// vertical space in a panel that's chronically tight.
	pinned []k8s.ResourceType
}

// SetPinned replaces the pinned-kinds list — called at startup from
// the config-loaded slice. Duplicates / unknown kinds are NOT filtered
// here; the caller is responsible for resolving config strings to
// registered ResourceTypes before invoking this.
//
// Re-snaps the cursor onto m.selected so the row the user logically
// is "on" stays under the cursor even though prepending the Pinned
// virtual category shifted every other index down by len(pinned)+1.
// Without this, startup with non-empty pinned list landed the cursor
// on whatever happened to be at the original index — usually Nodes
// instead of Pods, while panel 2 still showed Pods.
func (m *SidebarModel) SetPinned(kinds []k8s.ResourceType) {
	m.pinned = append(m.pinned[:0], kinds...)
	m.restoreCursorToSelected()
}

// SnapCursorToKind points the cursor at the unique row carrying the
// given resource kind. Used by AppModel after a pin/unpin toggle so
// the cursor follows the kind through the layout shift (Pods moves
// from Workloads → Pinned, or back, and the cursor follows along).
//
// Each kind appears in exactly one location now that Pinned is a
// MOVE not a duplicate — so the previous "prefer this section vs that
// section" logic isn't needed. No-op when the kind isn't visible
// (search-filtered out / not registered).
func (m *SidebarModel) SnapCursorToKind(rt k8s.ResourceType) {
	if rt == "" {
		return
	}
	visible := m.visibleItems()
	for i, item := range visible {
		if !item.isCategory && item.resourceType == rt {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
}

// PinnedKinds returns a copy of the currently pinned list — used by
// AppModel when serialising to config and by the Space-menu action
// context to decide whether the cursor row is already pinned.
func (m SidebarModel) PinnedKinds() []k8s.ResourceType {
	out := make([]k8s.ResourceType, len(m.pinned))
	copy(out, m.pinned)
	return out
}

// SetCursorAtScreenY moves the cursor to the visible row a mouse
// click landed on. screenY is the row count from the panel's top
// BORDER (0 = top border, 1 = first content row). The math accounts
// for:
//   - the top border (1 row, never selectable)
//   - the search box header (1 row, present only when searching)
//   - the scrollOffset (so a click on a scrolled-down sidebar
//     resolves to the right item in visibleItems)
//
// Clicks on category headers or out-of-range positions are no-ops —
// cursor never sits on a category header in keyboard nav either. The
// returned cmd carries ResourceSelectedMsg when the cursor actually
// landed on a resource row, matching the keyboard moveDown / moveUp
// flow so the rest of the app responds the same way.
func (m *SidebarModel) SetCursorAtScreenY(screenY int) tea.Cmd {
	contentY := screenY - 1 // skip the top border
	if contentY < 0 {
		return nil
	}
	if m.searching || m.searchQuery != "" {
		// renderSearchBox emits a 3-line bordered box (top / mid /
		// bottom). Previous "contentY--" only stepped over one line,
		// offsetting every click by 2 rows when the user was in
		// search mode.
		contentY -= 3
		if contentY < 0 {
			return nil
		}
	}
	rowIdx := m.scrollOffset + contentY
	visible := m.visibleItems()
	if rowIdx < 0 || rowIdx >= len(visible) {
		return nil
	}
	row := visible[rowIdx]
	if row.isCategory {
		return nil
	}
	m.cursor = rowIdx
	m.selected = row.resourceType
	rt := row.resourceType
	return func() tea.Msg { return ResourceSelectedMsg{Type: rt} }
}

// AddPinned appends a kind to the pinned list (insertion order = render
// order). No-op when the kind is already pinned — pin is idempotent
// because the menu hides "Pin" once an item is in the list, but the
// guard catches programmatic callers / future hotkey races.
func (m *SidebarModel) AddPinned(rt k8s.ResourceType) {
	for _, existing := range m.pinned {
		if existing == rt {
			return
		}
	}
	m.pinned = append(m.pinned, rt)
}

// RemovePinned drops a kind from the pinned list. No-op if missing.
func (m *SidebarModel) RemovePinned(rt k8s.ResourceType) {
	for i, existing := range m.pinned {
		if existing == rt {
			m.pinned = append(m.pinned[:i], m.pinned[i+1:]...)
			return
		}
	}
}

// IsPinned reports whether the given kind is currently in the pinned
// list — used by the Space-menu logic to surface "Pin" vs "Unpin".
func (m SidebarModel) IsPinned(rt k8s.ResourceType) bool {
	for _, existing := range m.pinned {
		if existing == rt {
			return true
		}
	}
	return false
}

// CursorPinned reports whether the cursor is currently parked on a
// row inside the Pinned virtual category. Each kind lives in exactly
// one section now (pin moves rather than duplicates), so categoryIndex
// alone is the unambiguous signal — no per-row pinned flag needed.
func (m SidebarModel) CursorPinned() bool {
	visible := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(visible) {
		return false
	}
	row := visible[m.cursor]
	return !row.isCategory && row.categoryIndex == pinnedCategoryIndex
}

// CursorResourceType returns the ResourceType under the cursor when
// the cursor is on a resource row (in any category — Pinned, Cluster,
// Workloads, ...). Returns empty / zero for category headers or out-
// of-range cursor. Used by the panel-1 Space menu to decide which
// pin/unpin action to surface.
func (m SidebarModel) CursorResourceType() k8s.ResourceType {
	visible := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(visible) {
		return ""
	}
	item := visible[m.cursor]
	if item.isCategory {
		return ""
	}
	return item.resourceType
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
		// Enter no longer forwards focus to panel 2. Mouse use brought
		// double-click → Enter synthesis, and "click row → focus shifts
		// to another panel" felt wrong (the user just pointed at THIS
		// panel — they don't expect the focus to leave). Keyboard
		// users still have Tab / 1 / 2 / 3 to switch focus, so this
		// only costs one extra key per panel switch.
		return m, nil
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
		// No match: Enter behaves like Esc — clears the filter and
		// restores the cursor. Without this the sidebar landed in a
		// dead state (searching=false, searchQuery non-empty, visible
		// empty), and handleKey's "no visible items → return early"
		// guard swallowed every subsequent key, including `/` to start
		// a new search. Only escape was switching panel.
		if len(visible) == 0 {
			m.searchQuery = ""
			m.restoreCursorToSelected()
			return m, nil
		}
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
	// Reset the scroll window to the top so a stale scrollOffset from
	// the previous (larger) list doesn't push the first match out of
	// view — the symptom is an apparently empty panel with a "1 of 1"
	// indicator at the bottom border.
	m.scrollOffset = 0
	for i, item := range visible {
		if !item.isCategory {
			m.cursor = i
			m.selected = item.resourceType
			m.ensureCursorVisible()
			return
		}
	}
	// No non-category match (e.g. user typed only "wo" matching the
	// "Workloads" category label) — park the cursor on the first item
	// so View has something to anchor on. selected is left unchanged.
	if len(visible) > 0 {
		m.cursor = 0
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
	// Pinned category header in Catppuccin Mocha lavender — same hue
	// family as the blue system categories so it reads as "same kind
	// of header" while still being distinguishable as the user-
	// curated section. Earlier picks: peach (#fab387) collided with
	// the KM8erm status-bar marker and read too loud; mauve (#cba6f7)
	// was distinct but the popup-border association pulled it
	// conceptually toward "this is interactive" — too much for a
	// passive category label.
	pinnedCategoryStyle := categoryStyle.Foreground(lipgloss.Color("#b4befe"))

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
			style := categoryStyle
			if item.categoryIndex == pinnedCategoryIndex {
				style = pinnedCategoryStyle
			}
			line = style.Width(m.width).Render(truncateSidebarLabel(item.label, m.width))
		} else {
			label := "  " + truncateSidebarLabel(item.label, m.width-2)
			unfocusedSelStyle := m.theme.SidebarUnfocusedSelectedStyle()
			if isCursor && m.focused {
				line = selectedStyle.Width(m.width).Render(label)
			} else if isCursor {
				line = unfocusedSelStyle.Width(m.width).Render(label)
			} else if item.resourceType == m.selected {
				line = unfocusedSelStyle.Width(m.width).Render(label)
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
// A category-level match (e.g. typing "cluster") expands the whole category;
// otherwise only individual resource items matching the query are shown.
//
// Pinned kinds appear ONLY in the virtual Pinned category at the top —
// they are skipped from their original category. If a category's only
// non-pinned kinds get pinned out, the category header is hidden too
// (avoids a lonely "Workloads" header above empty space).
func (m SidebarModel) visibleItems() []visibleItem {
	query := strings.ToLower(m.searchQuery)
	pinnedSet := make(map[k8s.ResourceType]struct{}, len(m.pinned))
	for _, rt := range m.pinned {
		pinnedSet[rt] = struct{}{}
	}
	var items []visibleItem
	// Pinned virtual category — prepended only when the user has pinned
	// anything. Each pinned kind is resolved against the registry for
	// its DisplayName so the label matches what the same kind shows in
	// its original category. Kinds that no longer resolve (CRD removed,
	// etc.) are silently skipped.
	if len(m.pinned) > 0 {
		var pinnedChildren []visibleItem
		catLabel := "Pinned"
		catMatch := query != "" && strings.Contains(strings.ToLower(catLabel), query)
		for ri, rt := range m.pinned {
			label := string(rt)
			if def := k8s.DefaultRegistry.Get(rt); def != nil {
				label = def.DisplayName
			}
			if query != "" && !catMatch && !strings.Contains(strings.ToLower(label), query) {
				continue
			}
			pinnedChildren = append(pinnedChildren, visibleItem{
				isCategory:    false,
				categoryIndex: pinnedCategoryIndex,
				resourceIndex: ri,
				label:         label,
				resourceType:  rt,
			})
		}
		if len(pinnedChildren) > 0 || query == "" {
			items = append(items, visibleItem{
				isCategory:    true,
				categoryIndex: pinnedCategoryIndex,
				resourceIndex: -1,
				label:         catLabel,
			})
			items = append(items, pinnedChildren...)
		}
	}
	for ci, cat := range m.categories {
		catMatch := query != "" && strings.Contains(strings.ToLower(cat.Label), query)
		var children []visibleItem
		for ri, res := range cat.Items {
			if _, isPinned := pinnedSet[res.ResourceType]; isPinned {
				continue
			}
			if query != "" && !catMatch && !strings.Contains(strings.ToLower(res.Label), query) {
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
		// Hide the category entirely when:
		//   - search filter is active and produced no children, OR
		//   - all kinds in the category got pinned out (no children
		//     left at all).
		// Without this guard a fully pinned-out category would render
		// as a dangling header with empty space underneath.
		if len(children) == 0 {
			continue
		}
		items = append(items, visibleItem{
			isCategory:    true,
			categoryIndex: ci,
			resourceIndex: -1,
			label:         cat.Label,
		})
		items = append(items, children...)
	}
	for _, res := range m.standalone {
		if _, isPinned := pinnedSet[res.ResourceType]; isPinned {
			continue
		}
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
	// Bail when the panel hasn't been sized yet — happens during
	// AppModel init when SetPinned → restoreCursorToSelected fires
	// before the first WindowSizeMsg. viewportHeight() clamps to 1
	// for render safety, so checking the post-clamp value would
	// happily set scrollOffset = cursor and hide every row above the
	// cursor. The actual fix is to skip scroll math entirely until
	// height is real; SetSize re-runs us once it is.
	if m.height <= 0 {
		return
	}
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

// SetSize sets the dimensions of the sidebar panel. Triggers a
// cursor-visibility re-snap once height becomes real — handles both
// the init path (first WindowSizeMsg after NewAppModel) and runtime
// resizes that could leave the cursor off-screen.
func (m *SidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ensureCursorVisible()
}

// SetFocused sets whether the sidebar is focused.
func (m *SidebarModel) SetFocused(focused bool) {
	m.focused = focused
}

// Selected returns the currently selected resource type.
// ClearSearch drops any active sidebar search filter and exits search
// mode. Used by the Relatives-tab space hotkey AND focus-leave so a
// stale filter from before the switch doesn't hide the freshly-selected
// resource type.
//
// Also repositions the cursor onto `selected` after the filter drops —
// without this the cursor index from the filtered view falls onto the
// wrong row in the now-larger visible list, which surfaces to the user
// as "I picked Helm/Releases, focus moved to panel 2, but panel 1 now
// shows the cursor on some unrelated entry".
func (m *SidebarModel) ClearSearch() {
	m.searching = false
	m.searchQuery = ""
	if m.selected != "" {
		m.SetSelected(m.selected)
	}
}

// SetSelected programmatically moves the sidebar cursor to the visible
// item matching `rt` and marks it as selected. No-op when the type isn't
// in the currently visible set (e.g. hidden by category collapse or
// search filter). Caller is responsible for dispatching ResourceSelectedMsg
// separately if downstream side effects need to fire — SetSelected only
// updates sidebar state, doesn't emit anything.
//
// Used by the Relatives-tab "space — jump to this resource" flow so panel 1
// highlight tracks the new resource type.
func (m *SidebarModel) SetSelected(rt k8s.ResourceType) {
	visible := m.visibleItems()
	for i, item := range visible {
		if item.resourceType == rt {
			m.cursor = i
			m.selected = rt
			m.ensureCursorVisible()
			return
		}
	}
}

func (m SidebarModel) Selected() k8s.ResourceType {
	return m.selected
}

// truncateSidebarLabel trims a sidebar label to fit `maxWidth` cells using `…`.
// Full name is recoverable from the panel 2 border title once selected, so
// truncation is acceptable here.
func truncateSidebarLabel(label string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if len(label) <= maxWidth {
		return label
	}
	if maxWidth == 1 {
		return "…"
	}
	return label[:maxWidth-1] + "…"
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
