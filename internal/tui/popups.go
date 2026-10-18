package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// The popups kbu opens; the lib draws, animates, stacks and dims them.

// Title glyphs kbu chooses (components/layout/popup: an input's, a form's
// and a note's glyph is the app's).
const (
	glyphYAML    = "\U0000F48A" // nf-oct-file_code
	glyphCompare = "\U000F08AA" // nf-md-vector_difference
	glyphLog     = "\U0000F03A" // nf-fa-list
	glyphNS      = "\U0000F51E" // nf-oct-stack
	glyphCtx     = "\U000F0237" // nf-md-folder_network
	glyphSearch  = "\U000F0233" // nf-md-file_find
)

// selection is the viewers' selection mode (Rules K11): v, then v again
// where the selection starts, y copies it.
func (m *Model) selection() *terminu.Selection {
	return &terminu.Selection{Key: "v", Name: "Visual", Copy: copyText}
}

// --- the YAML viewer ---------------------------------------------------------

// yamlSearch is the YAML viewer's search: the lines that match, and which of
// them the viewer is on.
type yamlSearch struct {
	v       *terminu.Viewer
	matches []int
	at      int
}

func (s *yamlSearch) find(q string) tea.Cmd {
	s.matches, s.at = nil, 0
	q = strings.ToLower(q)
	for i, l := range s.v.Lines {
		if strings.Contains(strings.ToLower(l), q) {
			s.matches = append(s.matches, i)
		}
	}
	return s.go_(0)
}

func (s *yamlSearch) go_(step int) tea.Cmd {
	if len(s.matches) == 0 {
		return nil
	}
	n := len(s.matches)
	s.at = ((s.at+step)%n + n) % n
	s.v.ScrollTo(s.matches[s.at])
	return nil
}

// openYAML shows a resource's YAML (components/dialog/note): / searches,
// n|N go to the next and the previous match, v selects, y copies, E edits.
func (m *Model) openYAML(rt k8s.ResourceType, item k8s.ResourceItem, yaml string) tea.Cmd {
	if yaml == "" {
		yaml = "# no YAML — the resource may still be loading"
	}
	v := &terminu.Viewer{Title: "YAML", Subject: subject(rt, item), Glyph: glyphYAML,
		Lines: strings.Split(strings.TrimRight(yaml, "\n"), "\n"), Lang: "yaml", Select: m.selection()}
	s := &yamlSearch{v: v}
	v.Keys = []terminu.ViewerKey{
		{Key: "/", Desc: "search", Do: func() tea.Cmd {
			return m.app.Text(terminu.TextInput{Title: "Search", Subject: subject(rt, item), Glyph: glyphSearch,
				Prompt: "Find in the YAML; [n] and [N] go to the next and the previous", Verb: "search",
				Done: func(q string) tea.Cmd {
					if s.find(q); len(s.matches) == 0 {
						return m.app.Toast(terminu.Info, "No match for "+q)
					}
					return nil
				}})
		}},
		{Key: "n", Desc: "next match", Do: func() tea.Cmd { return s.go_(1) }},
		{Key: "N", Desc: "previous match", Do: func() tea.Cmd { return s.go_(-1) }},
		{Key: "y", Desc: "copy", Do: func() tea.Cmd { return copyText(yaml) }},
	}
	if allowsEdit(rt) && item.Name != "" && !k8s.IsHelmManaged(item) {
		v.Keys = append(v.Keys, terminu.ViewerKey{Key: "E", Desc: "edit", Do: func() tea.Cmd { return m.confirmEdit(rt, item) }})
	}
	return m.app.Viewer(v)
}

// --- compare -------------------------------------------------------------------

// compareHere diffs the anchor against the row under the cursor, their YAML
// cleaned of what the server writes.
func (m *Model) compareHere() tea.Cmd {
	item, ok := m.currentItem()
	if !ok || m.anchor == nil {
		return nil
	}
	anchor := m.anchor.item
	if i := indexOfKey(m.shownItems(), m.anchor.key); i >= 0 {
		anchor = m.shownItems()[i]
	} else {
		return m.app.Toast(terminu.Info, "compare: anchor item gone")
	}
	left, right := k8s.MarshalItemYAMLForCompare(anchor), k8s.MarshalItemYAMLForCompare(item)
	ll, rl := diffLabel(anchor), diffLabel(item)
	split := m.cfg.Compare.Layout == "split"
	v := &terminu.Viewer{Title: "Compare", Subject: ll + " vs " + rl, Glyph: glyphCompare, Select: m.selection()}
	fill := func() {
		w, _ := m.app.Size()
		inner := max(min(w-2, 120)-4-5, 20) // the frame, its margins, the line numbers
		if split {
			v.Lines = splitDiff(left, right, ll, rl, inner)
		} else {
			v.Lines = unifiedDiff(left, right, ll, rl, inner)
		}
	}
	fill()
	v.Keys = []terminu.ViewerKey{{Key: "L", Desc: "layout", Do: func() tea.Cmd {
		split = !split
		fill()
		m.cfg.Compare.Layout = "unified"
		if split {
			m.cfg.Compare.Layout = "split"
		}
		if err := m.cfg.Save(); err != nil {
			return m.toastErr("compare layout save failed: " + err.Error())
		}
		return nil
	}}}
	return m.app.Viewer(v)
}

func diffLabel(it k8s.ResourceItem) string {
	if it.Namespace != "" {
		return it.Namespace + "/" + it.Name
	}
	return it.Name
}

// --- the App Log -------------------------------------------------------------

func (m *Model) openAppLog() tea.Cmd {
	m.log.seen()
	v := &terminu.Viewer{Title: "App log", Glyph: glyphLog, Lines: m.log.lines(), Select: m.selection()}
	if len(v.Lines) == 0 {
		v.Lines = []string{terminu.Fg(terminu.Overlay0).Render("No log entries")}
	}
	v.Keys = []terminu.ViewerKey{
		{Key: "y", Desc: "copy", Do: func() tea.Cmd {
			var b strings.Builder
			for _, l := range m.log.lines() {
				b.WriteString(terminu.StripStyles(l) + "\n")
			}
			return copyText(b.String())
		}},
		{Key: "D", Desc: "clear", Do: func() tea.Cmd {
			m.log.entries = nil
			v.Lines = []string{terminu.Fg(terminu.Overlay0).Render("No log entries")}
			return nil
		}},
	}
	m.log.viewer = v
	cmd := m.app.Viewer(v)
	v.ScrollTo(len(v.Lines) - 1) // the newest at the bottom, in sight
	return cmd
}

// --- namespaces ----------------------------------------------------------------

// openNamespaces asks the cluster for its namespaces; the picker opens when
// they come.
func (m *Model) openNamespaces() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return fetchNamespaces(m.client, false)
}

// namespacePicker picks which namespaces panel 2 shows: All Namespaces or
// any set of them (components/input/checkbox: Space checks a row, Enter
// takes the set).
func (m *Model) namespacePicker(names []string) tea.Cmd {
	sel := m.client.Selection()
	opts := append([]string{"All Namespaces"}, names...)
	checked := make([]bool, len(opts))
	checked[0] = sel.IsAll()
	for i, n := range names {
		checked[i+1] = !sel.IsAll() && sel.Contains(n)
	}
	return m.app.Checks(terminu.Checks{Title: "Namespaces", Glyph: glyphNS, Options: opts, Checked: checked,
		Done: func(now []bool) tea.Cmd { return m.setNamespaces(pickedSelection(sel, names, now)) }})
}

// pickedSelection is what the namespace picker's checks mean: All
// Namespaces, unless some namespaces are checked — checking a namespace
// while All is checked means just the ones checked; checking All again
// while some were picked means all; nothing checked is all too.
func pickedSelection(was k8s.NamespaceSelection, names []string, now []bool) k8s.NamespaceSelection {
	var picked []string
	for i, c := range now[1:] {
		if c {
			picked = append(picked, names[i])
		}
	}
	if len(picked) == 0 || (now[0] && !was.IsAll()) {
		return k8s.AllNamespaces()
	}
	return k8s.SelectedNamespaces(picked...)
}

// setNamespaces shows the namespaces in sel from now on.
func (m *Model) setNamespaces(sel k8s.NamespaceSelection) tea.Cmd {
	if m.client == nil {
		return nil
	}
	if sel.Equal(m.client.Selection()) {
		return nil
	}
	m.client.SetSelection(sel)
	m.log.info("namespace selection → " + selectionText(sel))
	m.stopLogs()
	if m.watcher != nil {
		m.watcher.Stop()
	}
	m.drill, m.anchor, m.rootItems = nil, nil, nil
	m.setListRows()
	m.listP.Loading = true
	m.resetDetail(m.current)
	return m.startWatch()
}

// --- contexts ------------------------------------------------------------------

// openContexts picks the context kbu talks to (components/input/select),
// the one now chosen; picking it is the confirmation (Rules F6).
func (m *Model) openContexts() tea.Cmd {
	if m.client == nil {
		return nil
	}
	names := m.client.ListContexts()
	cur := slices.Index(names, m.client.ContextName())
	return m.app.Select(terminu.Select{Title: "Contexts", Glyph: glyphCtx, Options: names, Selected: cur,
		Done: func(i int) tea.Cmd { return m.switchContext(names[i]) }})
}

// --- sort ----------------------------------------------------------------------

// openSort is the first of the sort's two steps: the column (Rules F1: each
// step its own popup). A column already sorted by says where in the order
// and which way.
func (m *Model) openSort(rt k8s.ResourceType) tea.Cmd {
	cols := columnsOf(rt)
	chain := m.cfg.GetSort(rt.KubectlName())
	var fields []terminu.Action
	for _, c := range cols {
		if c.Title == "" {
			continue
		}
		title := c.Title
		desc := "sort by " + strings.ToLower(title)
		if i := chain.IndexOf(title); i >= 0 {
			desc = fmt.Sprintf("sorted %s, %s", ordinal(i+1), directionWord(chain[i].Direction))
		}
		fields = append(fields, terminu.Action{Label: title, Desc: desc, Stay: true,
			Do: func() tea.Cmd { return m.openSortDirection(rt, title) }})
	}
	reset := terminu.Action{Label: "Reset", Desc: "back to the namespace, then the name", Disabled: func() bool { return len(chain) == 0 },
		Do: func() tea.Cmd {
			m.cfg.ResetSort(rt.KubectlName())
			return m.sortSaved(rt)
		}}
	return m.app.Menu("Sort "+rt.String()+" by", terminu.MenuSection{Title: "fields", Actions: fields},
		terminu.MenuSection{Title: "all", Actions: []terminu.Action{reset}})
}

// openSortDirection is the second step: the direction, or out of the order.
func (m *Model) openSortDirection(rt k8s.ResourceType, column string) tea.Cmd {
	chain := m.cfg.GetSort(rt.KubectlName())
	cur := ""
	if i := chain.IndexOf(column); i >= 0 {
		cur = chain[i].Direction
	}
	mark := func(dir, desc string) string {
		if dir == cur {
			return desc + " (current)"
		}
		return desc
	}
	set := func(dir string) func() tea.Cmd {
		return func() tea.Cmd {
			m.cfg.SetSort(rt.KubectlName(), column, dir)
			return m.sortSaved(rt)
		}
	}
	rows := []terminu.Action{
		{Label: "Ascending", Desc: mark(config.SortDirectionAscending, "smallest first"), Do: set(config.SortDirectionAscending)},
		{Label: "Descending", Desc: mark(config.SortDirectionDescending, "largest first"), Do: set(config.SortDirectionDescending)},
	}
	if cur != "" {
		rows = append(rows, terminu.Action{Label: "Unset", Desc: "take it out of the order", Do: func() tea.Cmd {
			m.cfg.UnsetSortColumn(rt.KubectlName(), column)
			return m.sortSaved(rt)
		}})
	}
	return m.app.Menu("Sort "+rt.String()+" by "+column, terminu.MenuSection{Actions: rows})
}

// sortSaved writes the sort down and shows it, when panel 2 shows that kind.
func (m *Model) sortSaved(rt k8s.ResourceType) tea.Cmd {
	if rt == m.shownKind() {
		m.list.SortBy(m.sortKeys(rt, columnsOf(rt))...)
	}
	if err := m.cfg.Save(); err != nil {
		return m.toastErr("sort save failed: " + err.Error())
	}
	return nil
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	}
	return fmt.Sprintf("%dth", n)
}

func directionWord(d string) string {
	if d == config.SortDirectionDescending {
		return "descending"
	}
	return "ascending"
}

// --- the breadcrumb ----------------------------------------------------------

// openBreadcrumb lists the resources down the Relatives chain: picking one
// switches panels 1 and 2 to it, after asking.
func (m *Model) openBreadcrumb() tea.Cmd {
	refs := m.chainRefs()
	var rows []terminu.Action
	for i, ref := range refs {
		ref := ref
		text := ref.Type.KubectlName() + "/" + ref.Name
		if ref.Namespace != "" {
			text += " in " + ref.Namespace
		}
		desc := "switch panels 1 and 2 to it"
		if i == len(refs)-1 {
			desc = "where the tab is now"
		}
		details := []string{ref.Type.KubectlName() + "/" + ref.Name}
		if ref.Namespace != "" {
			details[0] += "  namespace: " + ref.Namespace
		}
		rows = append(rows, terminu.Action{Label: fmt.Sprintf("%d. %s", i+1, text), Desc: desc,
			Ask: &terminu.Ask{Title: "Switch", Subject: ref.Name, Details: details,
				Question: "Switch panel 1 + 2 to this resource?", Verb: "switch"},
			Do: func() tea.Cmd { return m.switchTo(ref) }})
	}
	return m.app.Menu("Breadcrumb", terminu.MenuSection{Actions: rows})
}

// switchTo shows ref's kind in panels 1 and 2, the cursor on ref once its
// list arrives.
func (m *Model) switchTo(ref k8s.RefTarget) tea.Cmd {
	m.kinds.ClearFilter()
	m.kinds.Select(string(ref.Type))
	m.kindAt = ref.Type
	m.setKindActions()
	r := ref
	m.pendingSel = &r
	if ref.Type == m.current && m.kindSeq > 0 {
		m.drill = nil
		m.setListRows()
		for _, it := range m.rootItems {
			if it.Name == ref.Name && it.Namespace == ref.Namespace {
				m.list.Select(itemKey(it))
				m.pendingSel = nil
			}
		}
		return nil
	}
	return m.selectKind(ref.Type, false)
}
