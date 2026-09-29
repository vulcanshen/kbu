package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// The `?` key reference lists the keys of the frontmost surface (tdp K6,
// M4): the popup on top — only that popup's keys — the mode the user is
// in, or the focused panel. A panel's list starts from its Space menu's
// rows, so the reference and the menu cannot drift apart.

// openKeyRef opens the key reference over whatever is frontmost.
func (m *AppModel) openKeyRef() tea.Cmd {
	title, rows := m.keyRef()
	m.help.SetSize(m.width, m.height)
	m.help.SetLayer(m.popupDepth() + 1)
	return m.help.Open(title, rows)
}

// keyRef returns the title and rows for the frontmost surface, walking
// the stack top-first the way keys are routed.
func (m *AppModel) keyRef() (string, []helpRow) {
	switch top := m.topLayer(); top {
	case &m.spaceMenu:
		rows := menuRows(m.spaceMenu.items)
		return "Space menu keys", append(rows, menuMoveRows("run the highlighted row", "close the menu", true)...)
	case &m.globalMenu:
		rows := menuRows(m.globalMenu.items)
		return "Global operation keys", append(rows, menuMoveRows("run the highlighted row", "back to the Space menu", false)...)
	case &m.listPicker:
		return "Sort keys", pickerRows("sort by the highlighted column", "close the sort picker")
	case &m.sortDirPicker:
		return "Sort direction keys", pickerRows("use the highlighted direction", "back to the columns")
	case &m.settingsPopup:
		return "Settings keys", pickerRows("change the highlighted setting", "close Settings")
	case &m.breadcrumbPopup:
		return "Breadcrumb keys", pickerRows("switch panels 1 and 2 to that resource", "close the breadcrumb")
	case &m.namespacePicker:
		return "Namespace picker keys", filterPickerRows("check / uncheck the highlighted namespace", m.namespacePicker.loading)
	case &m.contextPicker:
		return "Context picker keys", filterPickerRows("switch to the highlighted context", false)
	case &m.appLog:
		return "App log keys", []helpRow{
			{key: "y", desc: "copy the whole log"},
			{key: "D", desc: "clear the log"},
			{header: true, desc: "keys"},
			{key: "j/k", desc: "scroll a line"},
			{key: "u/d", desc: "scroll half a page"},
			{key: "g/G", desc: "newest / oldest"},
			{key: "Esc", desc: "close the log"},
		}
	case &m.yamlPopup:
		if m.yamlPopup.visualMode {
			return "Selection keys", yamlVisualRows()
		}
		return "YAML keys", yamlRows(m.yamlPopup.HasEdit(), m.yamlPopup.CanEdit())
	case &m.comparePopup:
		return "Compare keys", []helpRow{
			{key: "L", desc: "switch layout: unified / side by side"},
			{header: true, desc: "keys"},
			{key: "j/k", desc: "scroll a line"},
			{key: "u/d", desc: "scroll half a page"},
			{key: "gg/G", desc: "top / bottom"},
			{key: "Esc", desc: "close the diff"},
		}
	case &m.confirm:
		verb := confirmVerb(m.confirm.action)
		return "Confirm keys", []helpRow{
			{key: "Enter/y", desc: verb},
			{key: "Esc/n", desc: "cancel"},
		}
	case nil:
		if m.activePanel == SidebarPanel && m.sidebar.IsDragging() {
			return dragKeyRef()
		}
		return m.panelKeyRef()
	}
	return "Keys", nil
}

// keyName is how a hotkey is written in the reference — the way the
// menus, the footer and the hints write it (tdp M5: one notation, the
// key as pressed; Alt-S is Alt with a capital S). Keys that do the same
// thing share a row, joined with / (j/k), a range with – (1–3).
func keyName(k string) string {
	switch k {
	case "alt+S":
		return "Alt-S"
	case "alt+t":
		return "Alt-t"
	case "enter":
		return "Enter"
	case "esc":
		return "Esc"
	}
	return k
}

// menuRows turns menu rows into key-reference rows: every row with a key
// the user can press, under the region header it sits in. Rows without
// one (the Global operation row, the Helm documents) and regions left
// empty are dropped. Dimmed rows stay, dimmed here too: the key exists,
// it just can't run right now (tdp M6).
func menuRows(items []menuItem) []helpRow {
	var rows []helpRow
	var pending *helpRow // a region header, written once a row under it is
	for _, it := range items {
		switch {
		case it.separator:
			continue
		case it.header:
			h := helpRow{header: true, desc: it.label}
			pending = &h
			continue
		case it.key == "":
			continue
		}
		if pending != nil {
			rows = append(rows, *pending)
			pending = nil
		}
		desc := it.plainName()
		if it.hint != "" {
			desc += " — " + it.hint
		}
		rows = append(rows, helpRow{key: keyName(it.key), desc: desc, dim: it.disabled})
	}
	return rows
}

// menuMoveRows are the keys every menu answers to.
func menuMoveRows(enter, esc string, spaceCloses bool) []helpRow {
	rows := []helpRow{
		{header: true, desc: "keys"},
		{key: "j/k", desc: "move the cursor"},
		{key: "Enter", desc: enter},
		{key: "Esc", desc: esc},
	}
	if spaceCloses {
		rows = append(rows, helpRow{key: "Space", desc: "close the menu"})
	}
	return rows
}

// pickerRows are the keys of a pick-one list.
func pickerRows(enter, esc string) []helpRow {
	return []helpRow{
		{key: "j/k", desc: "move the cursor"},
		{key: "g/G", desc: "first / last row"},
		{key: "Enter", desc: enter},
		{key: "Esc", desc: esc},
	}
}

// filterPickerRows are the keys of the namespace / context pickers, list
// phase (while typing, every key but these is a character). loading: the
// picker is open but its list hasn't arrived — only Esc works, the rest
// are dimmed (tdp M6). The "while typing" section describes another
// surface (? there is a character), so it stays bright.
func filterPickerRows(enter string, loading bool) []helpRow {
	return []helpRow{
		{key: "j/k", desc: "move the cursor", dim: loading},
		{key: "u/d", desc: "half a page", dim: loading},
		{key: "gg/G", desc: "first / last row", dim: loading},
		{key: "Enter", desc: enter, dim: loading},
		{key: "/", desc: "type to filter (a new filter)", dim: loading},
		{key: "Tab", desc: "back to typing, keeping the filter", dim: loading},
		{key: "Esc", desc: "close the picker"},
		{header: true, desc: "while typing"},
		{key: "↑/↓", desc: "move the cursor"},
		{key: "Enter", desc: enter},
		{key: "Tab", desc: "to the list, keeping the filter"},
		{key: "Esc", desc: "close the picker"},
	}
}

// yamlRows are the YAML viewer's keys. E is listed where the viewer has
// it, dimmed where it can't run right now (hasEdit, canEdit: tdp M6).
func yamlRows(hasEdit, canEdit bool) []helpRow {
	rows := []helpRow{
		{key: "/", desc: "search; n / N next / previous match"},
		{key: "v", desc: "select characters (a mode — ? there lists its keys)"},
		{key: "y", desc: "copy the whole YAML"},
	}
	if hasEdit {
		rows = append(rows, helpRow{key: "E", desc: "kubectl edit this resource (asks first)", dim: !canEdit})
	}
	return append(rows, []helpRow{
		{header: true, desc: "move"},
		{key: "h/j/k/l", desc: "cursor left / down / up / right"},
		{key: "w/b/e", desc: "next word / previous word / word end"},
		{key: "0/$", desc: "line start / end"},
		{key: "u/d", desc: "half a page"},
		{key: "gg/G", desc: "top / bottom"},
		{key: "Esc", desc: "clear the search, then close"},
	}...)
}

// yamlVisualRows are the keys of the YAML viewer's selection mode (tdp
// K11: a mode has no Space menu; its keys live here and in the hint).
func yamlVisualRows() []helpRow {
	return []helpRow{
		{key: "h/j/k/l", desc: "extend the selection"},
		{key: "w/b/e", desc: "extend by word"},
		{key: "0/$", desc: "extend to line start / end"},
		{key: "y", desc: "copy the selection and leave"},
		{key: "v/Esc", desc: "leave the selection"},
		{key: "q/Ctrl-C", desc: "quit kbu"},
	}
}

// panelKeyRef lists the focused panel's keys: its Space menu rows (only
// the keys that can be pressed), then moving, panels, the core keys and
// the app-wide hotkeys.
func (m *AppModel) panelKeyRef() (string, []helpRow) {
	var title string
	var items []menuItem
	switch m.activePanel {
	case SidebarPanel:
		title, items = m.sidebarMenu()
	case TablePanel:
		title, items, _, _ = m.tableMenu()
	case DetailPanel:
		title, items = m.detailMenu()
	}
	rows := menuRows(items)
	rows = append(rows, helpRow{header: true, desc: "move"})
	if m.activePanel == DetailPanel {
		oneTab := m.detail.TabCount() < 2 // like Switch tab (tdp M6)
		rows = append(rows,
			helpRow{key: "j/k", desc: "scroll a line (on Relatives / History: move the cursor)"},
			helpRow{key: "u/d", desc: "half a page"},
			helpRow{key: "gg/G", desc: "top / bottom"},
			helpRow{key: "h/[", desc: "previous tab", dim: oneTab},
			helpRow{key: "l/]", desc: "next tab", dim: oneTab})
	} else {
		rows = append(rows,
			helpRow{key: "j/k", desc: "move the cursor"},
			helpRow{key: "u/d", desc: "half a page"},
			helpRow{key: "gg/G", desc: "first / last row"})
	}
	rows = append(rows,
		helpRow{header: true, desc: "panels"},
		helpRow{key: "Tab", desc: "next panel"},
		helpRow{key: "Shift-Tab", desc: "previous panel"},
		helpRow{key: "1–3", desc: "go to a panel"})
	rows = append(rows, helpRow{header: true, desc: "core keys"})
	if d := m.enterDesc(); d != "" && !hasKey(items, "enter") {
		rows = append(rows, helpRow{key: "Enter", desc: d})
	}
	if d := m.escDesc(); d != "" && !hasKey(items, "esc") {
		rows = append(rows, helpRow{key: "Esc", desc: d})
	}
	rows = append(rows,
		helpRow{key: "Space", desc: "the menu of what you can do here"},
		helpRow{key: "?", desc: "these keys"},
		helpRow{key: "q", desc: "quit kbu"},
		helpRow{key: "Ctrl-C", desc: "quit kbu, even while typing"})
	rows = append(rows, helpRow{header: true, desc: "app-wide (also in Global operation)"})
	rows = append(rows, helpRow{key: "N", desc: "pick which namespaces to show"})
	if m.activePanel != TablePanel { // on panel 2, C is Compare
		rows = append(rows, helpRow{key: "C", desc: "switch the kubeconfig context"})
	}
	rows = append(rows,
		helpRow{key: "Alt-t", desc: "show / hide the embedded shell (Alterm)"},
		helpRow{key: ">", desc: "Settings"},
		helpRow{key: "!", desc: "App log"})
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), menuTitleGlyph)) + " keys", rows
}

func hasKey(items []menuItem, key string) bool {
	for _, it := range items {
		if it.selectable() && it.key == key {
			return true
		}
	}
	return false
}

// enterDesc is what Enter does on the focused panel when its Space menu
// has no [Enter] row of its own (tdp K3). "" = Enter does nothing here.
func (m *AppModel) enterDesc() string {
	switch m.activePanel {
	case TablePanel:
		if m.drillDownPod != nil {
			return "shell into the container (same as S)"
		}
		if len(m.items) > 0 && !m.currentResource.SupportsDrillDown() && m.currentResource != k8s.ResourceContexts {
			return "open the YAML (same as Y)"
		}
	case DetailPanel:
		switch m.detail.ActiveTabName() {
		case "Relatives", "History":
			return ""
		}
		return "full-screen this panel (same as z)"
	}
	return ""
}

// escDesc is what Esc does on the focused panel (tdp K4: one step back,
// never out of the app).
func (m *AppModel) escDesc() string {
	switch m.activePanel {
	case SidebarPanel:
		if m.sidebar.HasActiveFilter() {
			return "clear the search filter"
		}
	case TablePanel:
		if m.table.HasActiveFilter() {
			return "clear the search filter"
		}
	}
	return "nothing here (closes popups; never leaves kbu)"
}

// dragKeyRef lists the keys of the pinned-kind drag mode (tdp K11: a mode
// has no Space menu and no list of runnable keys — this is where its
// keys are read).
func dragKeyRef() (string, []helpRow) {
	return "Drag mode keys", []helpRow{
		{key: "j/k", desc: "move the kind down / up among the pinned"},
		{key: "Enter/D", desc: "drop it here (keep the new order)"},
		{key: "Esc", desc: "cancel — back to the old order"},
		{key: "?", desc: "these keys"},
		{key: "q/Ctrl-C", desc: "quit kbu (the drag is not kept)"},
	}
}
