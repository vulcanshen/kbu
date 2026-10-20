package tui

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// Panel 1, KINDS: the resource kinds in their categories, the pinned ones in
// a Pinned group at the top (a pinned kind moves there; it is not copied).
// Panel 2 is its details: it shows the kind under the cursor at once
// (components/layout/panel, details), and Enter takes the focus there.

const pinnedGroup = "Pinned"

// dragState is the Drag mode on the pinned kinds (tdp K11): the order to
// go back to on Esc.
type dragState struct {
	before []k8s.ResourceType
	kind   k8s.ResourceType
}

func (m *Model) buildKinds() {
	m.kinds = &terminu.List{Draggable: true, EmptyFact: "No kinds match", EmptyHint: "Press [Esc] to clear the filter"}
	m.kinds.OnMove = func(r terminu.Row) tea.Cmd {
		m.kindAt = k8s.ResourceType(r.ID)
		m.setKindActions()
		return m.selectKind(m.kindAt, true)
	}
	m.kindsP = &terminu.Panel{Label: "Kinds", Body: m.kinds}
	m.setKindActions()
}

// refreshKinds lays panel 1 out again from the registry and the pins.
func (m *Model) refreshKinds() {
	pinnedSet := map[k8s.ResourceType]bool{}
	var rows []terminu.Row
	for _, rt := range m.pinned {
		if def := k8s.DefaultRegistry.Get(rt); def != nil {
			pinnedSet[rt] = true
			rows = append(rows, terminu.Row{ID: string(rt), Cells: []string{def.DisplayName}, Group: pinnedGroup})
		}
	}
	for _, cat := range k8s.DefaultRegistry.SidebarCategories() {
		for _, def := range cat.Resources {
			if pinnedSet[def.Type] {
				continue
			}
			rows = append(rows, terminu.Row{ID: string(def.Type), Cells: []string{def.DisplayName}, Group: cat.Label})
		}
	}
	m.kinds.SetRows(rows)
}

// setKindActions lays out panel 1's actions for the kind under the cursor:
// the names say which kind (tdp M5, the description says what to what).
func (m *Model) setKindActions() {
	name := m.kindAt.String()
	pinned := m.isPinned(m.kindAt)
	pin := terminu.Action{Key: "P", Label: "Pin " + name, Desc: "keep it at the top of panel 1", Scope: terminu.ItemScope,
		Do: m.togglePin}
	if pinned {
		pin.Label, pin.Desc = "Unpin "+name, "move it back to its category"
	}
	// Sorting a kind's list is panel 2's (Alt-S there): only it shows the
	// list (tdp P3).
	m.kindsP.Actions = []terminu.Action{
		pin,
		{Key: "Enter", Label: "Show in panel 2", Desc: "focus panel 2 on this kind", Scope: terminu.ItemScope, Hint: "open",
			ToDetails: func() bool { return true },
			Do:        func() tea.Cmd { return m.selectKind(m.kindAt, false) }},
		{Key: "D", Label: "Drag to reorder pinned kinds", Desc: "move this kind among the pinned ones", Scope: terminu.ItemScope,
			Hidden:   func() bool { return !m.isPinned(m.kindAt) },
			Disabled: func() bool { return len(m.pinned) < 2 },
			Do:       m.startDrag},
		{Key: "y", Label: "Copy", Desc: "the kind's kubectl name", Scope: terminu.ItemScope,
			Do: func() tea.Cmd { return copyText(m.kindAt.KubectlName()) }},
		{Key: "/", Label: "Search", Desc: "filter the kinds by name", Scope: terminu.PanelScope, Hint: "search",
			Do: func() tea.Cmd { m.kinds.OpenFilter(); return nil }},
		{Key: "z", Label: "Zoom", Desc: "full-screen this panel", Scope: terminu.PanelScope, Do: m.toggleZoom},
	}
}

func (m *Model) isPinned(rt k8s.ResourceType) bool { return slices.Contains(m.pinned, rt) }

// loadPinned reads the pinned kinds from the pins and sorts, in their order;
// kinds that are not known now (a CRD that is gone) keep their pin there.
func (m *Model) loadPinned() []k8s.ResourceType {
	var out []k8s.ResourceType
	for _, name := range m.kindData.PinnedOrdered() {
		if rt := k8s.DefaultRegistry.LookupByKubectlName(name); rt != "" {
			out = append(out, rt)
		}
	}
	return out
}

// savePinned writes the pins in their order, 10 apart.
func (m *Model) savePinned() error {
	known := map[string]bool{}
	for _, rt := range k8s.DefaultRegistry.AllTypes() {
		known[rt.KubectlName()] = true
	}
	for _, name := range m.kindData.PinnedOrdered() {
		if known[name] {
			m.kindData.UnsetPinned(name)
		}
	}
	for i, rt := range m.pinned {
		m.kindData.SetPinned(rt.KubectlName(), (i+1)*10)
	}
	return m.saveKinds()
}

// saveKinds writes the pins and sorts back where they are kept: kinds.yaml,
// or config.yaml while kbu could not move them from there (tdp E3).
func (m *Model) saveKinds() error {
	if m.kindsInConfig {
		return m.cfg.Save()
	}
	return m.kindData.Save()
}

// togglePin pins the kind under the cursor, or puts it back in its category.
// The cursor goes with it.
func (m *Model) togglePin() tea.Cmd {
	rt := m.kindAt
	if i := slices.Index(m.pinned, rt); i >= 0 {
		m.pinned = slices.Delete(m.pinned, i, i+1)
	} else {
		m.pinned = append(m.pinned, rt)
	}
	m.refreshKinds()
	m.kinds.Select(string(rt))
	m.setKindActions()
	if err := m.savePinned(); err != nil {
		return m.toastErr("pin save failed: " + err.Error())
	}
	return nil
}

// startDrag enters the Drag mode on the pinned kind under the cursor (tdp
// K11): j|k move it among the pinned ones, wrapping at the ends (tdp K12),
// Enter drops it and keeps the order, Esc puts it back.
func (m *Model) startDrag() tea.Cmd {
	if !m.isPinned(m.kindAt) || len(m.pinned) < 2 {
		return nil
	}
	m.drag = &dragState{before: slices.Clone(m.pinned), kind: m.kindAt}
	m.kinds.Hold()
	m.kindsP.EnterMode(&terminu.Mode{
		Name: "Drag", Exit: "cancel", Pause: true,
		Keys: []terminu.ModeKey{
			{Key: "j|k", Keys: []string{"j", "k", "↓", "↑"}, Desc: "move it down, up among the pinned kinds", NoFooter: true,
				Do: func(key string) tea.Cmd { m.moveDragged(key == "j" || key == "↓"); return nil }},
			{Key: "Enter", Desc: "drop it here", Do: func(string) tea.Cmd { return m.dropDragged() }},
		},
		OnExit: func() tea.Cmd {
			m.pinned = m.drag.before
			m.endDrag()
			return nil
		},
	})
	return nil
}

// moveDragged moves the held kind one place among the pinned ones; past the
// last it goes to the top and the others move down one (tdp K12).
func (m *Model) moveDragged(down bool) {
	i := slices.Index(m.pinned, m.drag.kind)
	if i < 0 {
		return
	}
	n := len(m.pinned)
	switch {
	case down && i == n-1:
		m.pinned = append([]k8s.ResourceType{m.drag.kind}, m.pinned[:n-1]...)
	case !down && i == 0:
		m.pinned = append(slices.Clone(m.pinned[1:]), m.drag.kind)
	case down:
		m.pinned[i], m.pinned[i+1] = m.pinned[i+1], m.pinned[i]
	default:
		m.pinned[i], m.pinned[i-1] = m.pinned[i-1], m.pinned[i]
	}
	m.refreshKinds()
	m.kinds.Select(string(m.drag.kind))
}

// dropDragged keeps the new order.
func (m *Model) dropDragged() tea.Cmd {
	m.kindsP.LeaveMode()
	m.endDrag()
	if err := m.savePinned(); err != nil {
		return m.toastErr("pin order save failed: " + err.Error())
	}
	return nil
}

func (m *Model) endDrag() {
	kind := m.drag.kind
	m.drag = nil
	m.kinds.Drop()
	m.refreshKinds()
	m.kinds.Select(string(kind))
}
