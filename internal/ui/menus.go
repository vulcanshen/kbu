package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// The Space menu of every panel, and the global operation popup.
//
// Every panel's Space menu is built here, in tdp M2's fixed shape: the
// item operation region (what the cursor's item can do), the panel
// operation region (what the panel or its tab can do), then one Global
// operation row that opens the global operation popup (M4). Every item
// and panel operation kbu has is a row (M3) — a letter hotkey is only the
// shortcut of a row, and running a row runs that same hotkey (panelKey),
// so the two can never drift apart.

// globalOpAction is the Global operation row's action. The row has no
// hotkey (tdp M2: it is the one row every Space menu ends with).
const globalOpAction = "global"

var globalOpRow = menuItem{label: "Global operation", action: globalOpAction, hint: "actions for the whole app", opens: true}

// globalActions is the one list the global operation popup shows:
// everything that acts on kbu as a whole rather than on a panel or the
// cursor's item. Leaving the app is here (tdp K9). Each row runs the same
// code as its hotkey on a panel.
var globalActions = []menuItem{
	{label: "Namespace", key: "N", hint: "pick which namespaces to show", opens: true},
	{label: "Context", key: "C", hint: "switch the kubeconfig context", opens: true},
	{label: "[Alt-t]erm", name: "Alterm", key: "alt+t", hint: "show the embedded shell (Alterm)", opens: true},
	{label: "Settings", key: ">", hint: "mouse and scroll direction", opens: true},
	{label: "App log", key: "!", hint: "what kbu has done and what failed", opens: true},
	{label: "Quit", key: "q", hint: "leave kbu"},
}

// menuTitle formats a menu's title: a glyph and text (tdp D3), the text
// being the focused panel's [N] and what the menu acts on (D4).
func menuTitle(glyph, text string) string {
	return " " + glyph + " " + text + " "
}

// groupedMenu assembles a panel's Space menu in tdp M2's order: the item
// operation and panel operation regions, each under its header (even
// when only one is left — the Global operation row is always there, so
// the menu always holds more than one kind of thing), a rule, then the
// Global operation row, which carries no header of its own (tdp v0.1.7).
// A region with nothing in it is left out, header and all.
func groupedMenu(itemOps, panelOps []menuItem) []menuItem {
	var out []menuItem
	for _, r := range []struct {
		title string
		items []menuItem
	}{
		{"item operation", itemOps},
		{"panel operation", panelOps},
	} {
		if len(r.items) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, menuItem{separator: true})
		}
		out = append(out, menuItem{header: true, label: r.title})
		out = append(out, r.items...)
	}
	if len(out) > 0 {
		out = append(out, menuItem{separator: true})
	}
	return append(out, globalOpRow)
}

// openSpaceMenu opens the focused panel's Space menu (tdp K5, M2, M7:
// it always opens, even when only the Global operation row is left).
func (m *AppModel) openSpaceMenu() tea.Cmd {
	var (
		title string
		items []menuItem
		rt    k8s.ResourceType
		item  k8s.ResourceItem
	)
	switch m.activePanel {
	case SidebarPanel:
		title, items = m.sidebarMenu()
	case TablePanel:
		title, items, rt, item = m.tableMenu()
	case DetailPanel:
		title, items = m.detailMenu()
	}
	m.spaceMenu.SetSize(m.width, m.height)
	m.spaceMenu.SetLayer(m.popupDepth() + 1)
	return m.spaceMenu.Open(title, items, rt, item)
}

// sidebarMenu is panel 1's Space menu: the cursor's kind, then the panel.
func (m *AppModel) sidebarMenu() (string, []menuItem) {
	search := menuItem{label: "Search", key: "/", hint: "filter the kinds by name"}
	rt := m.sidebar.CursorResourceType()
	if rt == "" { // cursor on a category header: no item to act on
		return menuTitle(menuTitleGlyph, "[1] Kinds"), groupedMenu(nil, []menuItem{search})
	}
	label := rt.String()
	var itemOps, panelOps []menuItem
	if m.sidebar.IsPinned(rt) {
		itemOps = append(itemOps, menuItem{label: "Unpin " + label, key: "P", hint: "move it back to its category"})
	} else {
		itemOps = append(itemOps, menuItem{label: "Pin " + label, key: "P", hint: "keep it at the top of panel 1"})
	}
	if def := sortRegistry().Get(rt); def != nil && len(def.Columns) > 0 {
		itemOps = append(itemOps, menuItem{label: "Sort panel 2 list", key: "S", hint: "order the " + label + " list by a column", opens: true})
	}
	itemOps = append(itemOps,
		menuItem{label: "[Enter] Show in panel 2", name: "Show in panel 2", key: "enter", action: "enter", hint: "focus panel 2 on this kind"},
		menuItem{label: "Copy", key: "y", hint: "the kind's kubectl name"})
	if m.sidebar.CursorPinned() {
		panelOps = append(panelOps, menuItem{label: "Drag to reorder pinned kinds", key: "D",
			hint: "move this kind among the pinned ones", disabled: len(m.sidebar.PinnedKinds()) < 2})
	}
	panelOps = append(panelOps, search)
	return menuTitle(menuTitleGlyph, "[1] "+label), groupedMenu(itemOps, panelOps)
}

// tableMenu is panel 2's Space menu. The item operations act on the row
// under the cursor when the menu opened (captured in the menu).
func (m *AppModel) tableMenu() (string, []menuItem, k8s.ResourceType, k8s.ResourceItem) {
	panelOps := m.tablePanelOps()
	if m.drillDownPod != nil {
		title := menuTitle(menuTitleGlyph, "[2] "+m.drillDownPod.Name+" containers")
		var itemOps []menuItem
		if idx := m.table.SelectedRow(); idx >= 0 && idx < len(m.drillDownContainers) {
			title = menuTitle(menuTitleGlyph, "[2] container/"+m.drillDownContainers[idx].Name)
			itemOps = []menuItem{
				{label: "Shell", key: "S", hint: "kubectl exec -it into this container (also Enter)", opens: true},
				{label: "Copy", key: "y", hint: "this row, tab-separated"},
			}
		}
		return title, groupedMenu(itemOps, panelOps), k8s.ResourcePods, *m.drillDownPod
	}
	idx := m.table.SelectedRow()
	if idx < 0 || idx >= len(m.items) {
		return menuTitle(menuTitleGlyph, "[2] "+m.currentResource.String()), groupedMenu(nil, panelOps), m.currentResource, k8s.ResourceItem{}
	}
	item := m.items[idx]
	helmManaged := k8s.IsHelmManaged(item)
	var itemOps []menuItem
	switch m.currentResource {
	case k8s.ResourceReleases:
		itemOps = append(itemOps, menuItem{label: "YAML", key: "Y", hint: "the release record (also Enter)", opens: true})
		itemOps = append(itemOps, helmDocMenuItems()...)
		itemOps = append(itemOps, menuItem{label: "Copy", key: "y", hint: "this row, tab-separated"})
	case k8s.ResourceContexts:
		itemOps = panel2ItemOps(m.currentResource, item, helmManaged, m.compareCtxForMenu(item))
		itemOps = append([]menuItem{{label: "[Enter] Switch to this context", name: "Switch to this context", key: "enter",
			action: "enter", hint: "kbu only; ~/.kube/config is not changed", opens: true,
			disabled: item.Name == m.k8sClient.ContextName()}}, itemOps...)
	default:
		itemOps = panel2ItemOps(m.currentResource, item, helmManaged, m.compareCtxForMenu(item))
	}
	glyph := menuTitleGlyph
	if helmManaged {
		glyph = k8s.HelmIcon()
	}
	title := menuTitle(glyph, "[2] "+m.currentResource.KubectlName()+"/"+item.Name)
	return title, groupedMenu(itemOps, panelOps), m.currentResource, item
}

// tablePanelOps are panel 2's panel operations: they act on the list.
func (m *AppModel) tablePanelOps() []menuItem {
	var ops []menuItem
	if m.inCompareMode() {
		ops = append(ops, menuItem{label: "[Esc] Exit compare mode", name: "Exit compare mode", key: "esc", action: "exit-compare", hint: "drop the compare anchor"})
	}
	if m.drillDownPod != nil || len(m.drillDownStack) > 0 {
		back := menuItem{label: "[Esc] Back", name: "Back", key: "esc", action: "back", hint: "to the " + m.drillParentLabel() + " list"}
		if m.inCompareMode() {
			// Esc exits compare mode first (one layer per press, tdp
			// K4); this row still backs out in one step.
			back.label, back.key = "Back", ""
		}
		ops = append(ops, back)
	}
	if m.drillDownPod == nil {
		if def := sortRegistry().Get(m.currentResource); def != nil && len(def.Columns) > 0 {
			ops = append(ops, menuItem{label: "[Alt-S]ort panel 2 list", name: "Sort panel 2 list", key: "alt+S", hint: "order the rows by a column", opens: true})
		}
	}
	ops = append(ops, menuItem{label: "Search", key: "/", hint: "filter the rows by name"})
	if m.drillDownPod == nil && m.currentResource != k8s.ResourceReleases {
		ops = append(ops, menuItem{label: "Helm-managed rows", key: ".", hint: "show / hide rows a Helm chart manages"})
	}
	return append(ops, menuItem{label: "Zoom", key: "z", hint: "full-screen this panel"})
}

// drillParentLabel names the list Esc goes back to from a drill.
func (m *AppModel) drillParentLabel() string {
	if m.drillDownPod != nil {
		return m.currentResource.String()
	}
	if n := len(m.drillDownStack); n > 0 {
		return m.drillDownStack[n-1].parentType.String()
	}
	return ""
}

// detailMenu is panel 3's Space menu, by tab.
func (m *AppModel) detailMenu() (string, []menuItem) {
	tab := m.detail.ActiveTabName()
	title := menuTitle(menuTitleGlyph, "[3] "+tab)
	yaml := menuItem{label: "YAML", key: "Y", hint: "this resource's manifest", opens: true}
	copyAll := menuItem{label: "Copy", key: "y", hint: "everything in this tab"}
	zoom := menuItem{label: "Zoom", key: "z", hint: "full-screen this panel"}
	if tab := m.detail.ActiveTabName(); tab != "Relatives" && tab != "History" {
		zoom.hint = "full-screen this panel (also Enter)"
	}
	switchTab := menuItem{label: "Switch tab", key: "l", hint: "next tab (h: the previous one)", disabled: m.detail.TabCount() < 2}

	var itemOps, panelOps []menuItem
	switch tab {
	case "Logs":
		panelOps = []menuItem{{label: "Go live", key: "G", hint: "jump to the newest line and follow"}, copyAll, yaml, zoom, switchTab}
	case "Events":
		panelOps = []menuItem{{label: "Go live", key: "G", hint: "jump to the newest event and follow"}, copyAll, yaml, zoom, switchTab}
	case "Relatives":
		if m.detail.SelectedRelativeRef() != nil {
			itemOps = []menuItem{
				{label: "[Enter] Drill in", name: "Drill in", key: "enter", action: "rel-drill", hint: "show its relatives here"},
				{label: "YAML", key: "Y", hint: "this entry's manifest", opens: true},
				{label: "Copy", key: "y", hint: "this row"},
			}
		} else if m.detail.CopyableContent() != "" {
			itemOps = []menuItem{{label: "Copy", key: "y", hint: "this row"}}
		}
		if m.detail.Depth() > 1 {
			panelOps = append(panelOps,
				menuItem{label: "[Esc] Back", name: "Back", key: "esc", action: "rel-back", hint: "up one level of the chain"},
				menuItem{label: "Jump to an ancestor", action: "breadcrumb", hint: "switch panels 1 and 2 to a resource up the chain", opens: true})
		}
		if m.detail.SelectedRelativeRef() == nil {
			panelOps = append(panelOps, yaml)
		}
		panelOps = append(panelOps, zoom, switchTab)
	case "History":
		if rev, current := m.detail.HistoryCursor(); rev != nil {
			itemOps = []menuItem{
				{label: "[Enter] Roll back to this revision", name: "Roll back to this revision", key: "enter", action: "rollback",
					hint: "helm rollback", opens: true, disabled: current},
				{label: "Copy", key: "y", hint: "this row"},
			}
		}
		panelOps = []menuItem{yaml, zoom, switchTab}
	default: // Conditions, Info: content without a cursor
		panelOps = []menuItem{copyAll, yaml, zoom, switchTab}
	}
	return title, groupedMenu(itemOps, panelOps)
}

// openGlobalMenu opens the global operation popup over the Space menu
// (tdp M4, F4): Esc on it returns to the Space menu.
func (m *AppModel) openGlobalMenu() tea.Cmd {
	m.globalMenu.SetSize(m.width, m.height)
	m.globalMenu.SetLayer(m.popupDepth() + 1)
	return m.globalMenu.Open(menuTitle("\U000f01e7", "Global operation"), globalActions, "", k8s.ResourceItem{})
}

// runMenuAction runs a row committed on the Space menu or the global
// operation popup. A row that opens a popup leaves the menu beneath it
// (tdp F4); any other row has done its job and closes the Space menu
// (T1).
func (m *AppModel) runMenuAction(msg MenuActionMsg) tea.Cmd {
	if msg.Menu == "global" {
		return m.runGlobalAction(msg.Action)
	}
	var closeCmd tea.Cmd
	if !msg.Opens {
		closeCmd = m.spaceMenu.Close()
	}
	return tea.Batch(closeCmd, m.runSpaceAction(msg))
}

// runSpaceAction runs one Space-menu row.
func (m *AppModel) runSpaceAction(msg MenuActionMsg) tea.Cmd {
	switch msg.Action {
	case globalOpAction:
		return m.openGlobalMenu()
	case "drill":
		return m.enterDrillDown()
	case "back":
		m.clearCompareLock()
		return m.exitDrillDown()
	case "exit-compare":
		m.clearCompareLock()
		return nil
	case "rel-drill":
		if ref := m.detail.SelectedRelativeRef(); ref != nil {
			target := *ref
			return func() tea.Msg { return RelativePushMsg{Ref: target} }
		}
		return nil
	case "rel-back":
		return m.dispatchToPanel(tea.KeyMsg{Type: tea.KeyEsc})
	case "breadcrumb":
		if m.detail.Depth() <= 1 {
			return nil
		}
		m.breadcrumbPopup.SetSize(m.width, m.height)
		m.breadcrumbPopup.SetLayer(m.popupDepth() + 1)
		return m.breadcrumbPopup.Open(m.detail.DrillChain())
	case "rollback":
		return m.confirmRollback()
	case "enter":
		return m.enterKey()
	}
	if kind := helmDocKindOf(msg.Action); kind != "" {
		return fetchHelmDocCmd(kind, msg.Item.Name, msg.Item.Namespace)
	}
	// Panel 2 item operations act on the row captured when the menu
	// opened, not on whatever a watcher tick has since moved under the
	// cursor.
	if m.activePanel == TablePanel && m.drillDownPod == nil && msg.Item.Name != "" {
		switch msg.Action {
		case "Y":
			return m.openYamlFor(msg.Resource, msg.Item)
		case "E":
			return m.confirmEdit(msg.Resource, msg.Item)
		case "D":
			return m.confirmDelete(msg.Resource, msg.Item)
		case "C":
			return m.compareHotkeyDispatch(msg.Resource, msg.Item)
		}
	}
	// Every other row is the hotkey it names, run on the focused panel.
	return m.panelKey(keyMsgFor(msg.Action))
}

// runGlobalAction runs one row of the global operation popup — the same
// code as the row's hotkey on a panel.
func (m *AppModel) runGlobalAction(action string) tea.Cmd {
	switch action {
	case "N":
		return m.openNamespacePicker()
	case "C":
		return fetchContexts(m.k8sClient)
	case "alt+t":
		return m.toggleAlterm()
	case ">":
		return m.openSettings()
	case "!":
		return m.openAppLog()
	case "q":
		return func() tea.Msg { return quitMsg{} }
	}
	return nil
}

// keyMsgFor turns a menu row's hotkey back into the key press it stands
// for, so running the row runs the hotkey's own code path.
func keyMsgFor(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "alt+S":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}, Alt: true}
	case "alt+t":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}, Alt: true}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}
