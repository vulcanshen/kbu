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

// Title glyphs kbu chooses (tdp popup.1: an input's, a form's
// and a note's glyph is the app's).
const (
	glyphYAML   = "\U0000F48A" // nf-oct-file_code
	glyphNS     = "\U0000F51E" // nf-oct-stack
	glyphCtx    = "\U000F0237" // nf-md-folder_network
	glyphSearch = "\U000F0233" // nf-md-file_find
)

// selection is the viewers' selection mode (tdp K11): v, then v again
// where the selection starts, y copies it.
func (m *Model) selection() *terminu.Selection {
	return &terminu.Selection{Key: "v", Name: "Visual", Copy: copyText}
}

// --- the YAML viewer ---------------------------------------------------------

// yamlSearch is the YAML viewer's search: the viewer draws its hits, every
// one underlined and the one it is on as the cursor block
// (tdp note.2).
type yamlSearch struct{ v *terminu.Viewer }

func (s *yamlSearch) find(q string) tea.Cmd {
	s.v.Hits, s.v.Current = s.v.FindText(q), 0
	return s.go_(0)
}

func (s *yamlSearch) go_(step int) tea.Cmd {
	n := len(s.v.Hits)
	if n == 0 {
		return nil
	}
	s.v.Current = ((s.v.Current+step)%n + n) % n
	s.v.ScrollTo(s.v.Hits[s.v.Current].Line)
	return nil
}

// openYAML shows a resource's YAML (tdp note): / searches,
// n|N go to the next and the previous match, v selects, y copies, E edits.
func (m *Model) openYAML(rt k8s.ResourceType, item k8s.ResourceItem, yaml string) tea.Cmd {
	m.log.info("Opened the YAML of " + subject(rt, item))
	if yaml == "" {
		yaml = "# no YAML — the resource may still be loading"
	}
	v := &terminu.Viewer{Title: "YAML", Subject: subject(rt, item), Glyph: glyphYAML,
		Lines: strings.Split(strings.TrimRight(yaml, "\n"), "\n"), Lang: "yaml", Select: m.selection()}
	s := &yamlSearch{v: v}
	// show puts the resource as it is now in the viewer, after an edit.
	show := func(y string) {
		yaml, v.Lines, v.Hits = y, strings.Split(strings.TrimRight(y, "\n"), "\n"), nil
	}
	v.Keys = []terminu.ViewerKey{
		{Key: "/", Desc: "search", Do: func() tea.Cmd {
			// Search: a pasted control character is only drawn, not refused
			// (tdp input.8).
			return m.app.Text(terminu.TextInput{Title: "Search", Subject: subject(rt, item), Glyph: glyphSearch,
				Name: "The text to find", Search: true,
				Prompt: "Find in the YAML; [n] and [N] go to the next and the previous", Verb: "search",
				Done: func(q string) tea.Cmd {
					if s.find(q); len(v.Hits) == 0 {
						return m.app.Toast(terminu.Info, "No match for "+q)
					}
					return nil
				}})
		}},
		{Key: "n", Desc: "next", Do: func() tea.Cmd { return s.go_(1) }},
		{Key: "N", Desc: "previous", Do: func() tea.Cmd { return s.go_(-1) }},
		{Key: "y", Desc: "copy", Do: func() tea.Cmd { return copyText(yaml) }},
	}
	if allowsEdit(rt) && item.Name != "" && !k8s.IsHelmManaged(item) {
		// tdp T1: an edit in the editor keeps the viewer it was started
		// from; when the edit ends, the viewer reads the resource again.
		v.Keys = append(v.Keys, terminu.ViewerKey{Key: "E", Desc: "edit", Do: func() tea.Cmd {
			return m.confirmEdit(rt, item, func() tea.Cmd { return m.rereadYAML(rt, item, show) })
		}})
	}
	return m.app.Viewer(v)
}

// --- compare -------------------------------------------------------------------

func diffLabel(it k8s.ResourceItem) string {
	if it.Namespace != "" {
		return it.Namespace + "/" + it.Name
	}
	return it.Name
}

// --- namespaces ----------------------------------------------------------------

// openNamespaces opens the namespace picker at once and asks the cluster for
// its namespaces, which fill it as they come (onNamespaces).
func (m *Model) openNamespaces() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return tea.Batch(m.namespacePicker(m.client.Selection()), fetchNamespaces(m.client, false))
}

// namespacePicker picks which namespaces panel 2 shows, any set of them
// (tdp checkbox: Space checks a row, Enter takes the set; the
// lib's Select all row checks them all). All of them shows every one
// checked. It opens loading (tdp F7); fillNamespaces puts the namespaces in.
func (m *Model) namespacePicker(sel k8s.NamespaceSelection) tea.Cmd {
	var names []string
	c := &terminu.Checks{Title: "Namespaces", Glyph: glyphNS, Loading: true,
		Done: func(now []bool) tea.Cmd { return m.setNamespaces(pickedSelection(names, now)) }}
	m.fillNamespaces = func(got []string) {
		names = got
		for _, n := range got {
			c.Options = append(c.Options, n)
			c.Checked = append(c.Checked, sel.IsAll() || sel.Contains(n))
		}
		c.Loading = false
	}
	return m.app.LoadChecks(c)
}

// pickedSelection is what the namespace picker's checks mean: every one
// checked is the whole cluster, namespaces made later too (kbu's reading,
// tdp checkbox.3); nothing checked shows all too,
// there being nothing else to show; otherwise the namespaces checked.
func pickedSelection(names []string, now []bool) k8s.NamespaceSelection {
	var picked []string
	for i, c := range now {
		if c {
			picked = append(picked, names[i])
		}
	}
	if len(picked) == 0 || len(picked) == len(names) {
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
	m.log.info("Namespace selection → " + selectionText(sel))
	m.stopLogs()
	if m.watcher != nil {
		m.watcher.Stop()
	}
	m.drill, m.rootItems = nil, nil
	m.setListRows()
	m.listP.Loading = true
	m.resetDetail(m.current)
	return m.startWatch()
}

// --- contexts ------------------------------------------------------------------

// openContexts picks the context kbu talks to (tdp select),
// the one now chosen; picking it is the confirmation (tdp F6).
func (m *Model) openContexts() tea.Cmd {
	if m.client == nil {
		return nil
	}
	names := m.client.ListContexts()
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	cur := slices.Index(names, m.client.ContextName())
	return m.app.Select(terminu.Select{Title: "Contexts", Glyph: glyphCtx, Options: names, Selected: cur,
		Done: func(i int) tea.Cmd { return m.switchContext(names[i]) }})
}

// --- sort ----------------------------------------------------------------------

// openSort is the first of the sort's two steps: the column (tdp F1: each
// step its own popup). A column already sorted by says where in the order
// and which way.
func (m *Model) openSort(rt k8s.ResourceType) tea.Cmd {
	cols := columnsOf(rt)
	var fields []terminu.Action
	for _, c := range cols {
		if c.Title == "" {
			continue
		}
		title := c.Title
		// Live: back from the direction, the row says the sort it set.
		live := func() (string, string) {
			if i := m.kindData.GetSort(rt.KubectlName()).IndexOf(title); i >= 0 {
				chain := m.kindData.GetSort(rt.KubectlName())
				return title, fmt.Sprintf("sorted %s, %s", ordinal(i+1), directionWord(chain[i].Direction))
			}
			return title, "sort by " + strings.ToLower(title)
		}
		fields = append(fields, terminu.Action{Label: title, Live: live,
			Do: func() tea.Cmd { return m.openSortDirection(rt, title) }})
	}
	reset := terminu.Action{Label: "Reset", Desc: "back to the namespace, then the name",
		Disabled: func() bool { return len(m.kindData.GetSort(rt.KubectlName())) == 0 },
		Do: func() tea.Cmd {
			m.kindData.ResetSort(rt.KubectlName())
			return m.sortSaved(rt)
		}}
	// What the menu acts on goes after ": ", as it is (tdp popup.1,
	// the title).
	return m.app.MenuAbout("Sort by", rt.String(), terminu.MenuSection{Title: "fields", Actions: fields},
		terminu.MenuSection{Title: "all", Actions: []terminu.Action{reset}})
}

// openSortDirection is the second step: the direction, or out of the order.
// Choosing one writes it back to the columns, the first step, and closes only
// this one (tdp T1: a value written back to a picker keeps it), so a second
// column is the next row.
func (m *Model) openSortDirection(rt k8s.ResourceType, column string) tea.Cmd {
	chain := m.kindData.GetSort(rt.KubectlName())
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
			m.kindData.SetSort(rt.KubectlName(), column, dir)
			return tea.Batch(m.sortSaved(rt), m.app.CloseTop(1))
		}
	}
	rows := []terminu.Action{
		{Label: "Ascending", Desc: mark(config.SortDirectionAscending, "smallest first"), Stay: true, Do: set(config.SortDirectionAscending)},
		{Label: "Descending", Desc: mark(config.SortDirectionDescending, "largest first"), Stay: true, Do: set(config.SortDirectionDescending)},
	}
	if cur != "" {
		rows = append(rows, terminu.Action{Label: "Unset", Desc: "take it out of the order", Stay: true, Do: func() tea.Cmd {
			m.kindData.UnsetSortColumn(rt.KubectlName(), column)
			return tea.Batch(m.sortSaved(rt), m.app.CloseTop(1))
		}})
	}
	return m.app.MenuAbout("Direction", column, terminu.MenuSection{Actions: rows})
}

// sortSaved writes the sort down and shows it, when panel 2 shows that kind.
func (m *Model) sortSaved(rt k8s.ResourceType) tea.Cmd {
	by := "the namespace, then the name"
	if chain := m.kindData.GetSort(rt.KubectlName()); len(chain) > 0 {
		cols := make([]string, len(chain))
		for i, c := range chain {
			cols[i] = c.Column + " " + directionWord(c.Direction)
		}
		by = strings.Join(cols, ", then ")
	}
	m.log.info("Sorted " + rt.String() + " by " + by)
	if rt == m.shownKind() {
		m.list.SortBy(m.sortKeys(rt, columnsOf(rt))...)
	}
	if err := m.saveKinds(); err != nil {
		return m.toastErr("Sort save failed: " + err.Error())
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
		label := fmt.Sprintf("%d. %s", i+1, text)
		if i == len(refs)-1 {
			// Where the user is; a menu row is a short text, in no colour
			// of the app's own (tdp components.5).
			desc = "where the tab is now"
		}
		details := []string{ref.Type.KubectlName() + "/" + ref.Name}
		if ref.Namespace != "" {
			details[0] += "  namespace: " + ref.Namespace
		}
		rows = append(rows, terminu.Action{Label: label, Desc: desc, Clear: true,
			Ask: &terminu.Ask{Title: "Switch", Subject: ref.Name, Details: details,
				Question: "Switch panel 1 + 2 to this resource?", Verb: "switch"},
			Do: func() tea.Cmd { return m.switchTo(ref) }})
	}
	return m.app.Menu("Breadcrumb", terminu.MenuSection{Actions: rows})
}

// switchTo shows ref's kind in panels 1 and 2, the cursor on ref once its
// list arrives.
func (m *Model) switchTo(ref k8s.RefTarget) tea.Cmd {
	m.log.info("Switched panels 1 and 2 to " + refSubject(ref))
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
