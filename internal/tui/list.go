package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// Panel 2: the resources of the kind panel 1 shows, live through the
// Kubernetes watch. Enter drills into what a resource runs (a workload into
// its pods, a pod into its containers); Esc comes back one level. Panel 3 is
// its details.

// drillLevel is one level below the kind panel 1 shows: the children of the
// row drilled into, or a pod's containers.
type drillLevel struct {
	parentKind k8s.ResourceType
	parent     k8s.ResourceItem
	kind       k8s.ResourceType // the children's kind; "" for a pod's containers
	items      []k8s.ResourceItem
	containers []k8s.ContainerInfo
}

// anchorRef is the compare anchor: the row a later C compares against.
type anchorRef struct {
	kind k8s.ResourceType
	key  string
	item k8s.ResourceItem
}

func (m *Model) buildList() {
	m.list = &terminu.List{}
	m.list.OnMove = func(r terminu.Row) tea.Cmd { return m.rowMoved(r.ID) }
	m.listP = &terminu.Panel{Label: "Pods", Body: m.list, OnEsc: m.listEsc}
	m.setListActions()
}

// shownKind is the kind of panel 2's rows: the kind drilled into, or the one
// panel 1 shows; "" for a pod's containers.
func (m *Model) shownKind() k8s.ResourceType {
	if n := len(m.drill); n > 0 {
		return m.drill[n-1].kind
	}
	return m.current
}

// atContainers reports panel 2 showing a pod's containers.
func (m *Model) atContainers() bool {
	n := len(m.drill)
	return n > 0 && m.drill[n-1].kind == ""
}

// shownItems are panel 2's rows as resources (none for containers).
func (m *Model) shownItems() []k8s.ResourceItem {
	if n := len(m.drill); n > 0 {
		return m.drill[n-1].items
	}
	return m.rootItems
}

// currentItem is the resource under panel 2's cursor.
func (m *Model) currentItem() (k8s.ResourceItem, bool) {
	r, ok := m.list.Current()
	if !ok || m.atContainers() {
		return k8s.ResourceItem{}, false
	}
	items := m.shownItems()
	if i := indexOfKey(items, r.ID); i >= 0 {
		return items[i], true
	}
	return k8s.ResourceItem{}, false
}

// currentContainer is the container under the cursor, at a pod's containers.
func (m *Model) currentContainer() (k8s.ContainerInfo, k8s.ResourceItem, bool) {
	r, ok := m.list.Current()
	if !ok || !m.atContainers() {
		return k8s.ContainerInfo{}, k8s.ResourceItem{}, false
	}
	lv := m.drill[len(m.drill)-1]
	for _, c := range lv.containers {
		if c.Name == r.ID {
			return c, lv.parent, true
		}
	}
	return k8s.ContainerInfo{}, k8s.ResourceItem{}, false
}

// selectKind shows rt in panel 2. The watch starts once the cursor has
// rested on the kind (kindDebounce) when debounce is set.
func (m *Model) selectKind(rt k8s.ResourceType, debounce bool) tea.Cmd {
	if rt == "" || (rt == m.current && m.kindSeq > 0) {
		return nil
	}
	m.log.info("switched to " + rt.String())
	m.current = rt
	m.drill, m.anchor = nil, nil
	m.rootItems = nil
	m.stopLogs()
	if m.watcher != nil {
		m.watcher.Stop()
	}
	m.list.ClearFilter()
	m.setListRows()
	m.listP.Loading, m.listP.Fail = m.client != nil, nil
	m.listP.LoadingText = "loading " + strings.ToLower(rt.String()) + "…"
	m.resetDetail(rt)
	m.kindSeq++
	if !debounce {
		return m.startWatch()
	}
	seq := m.kindSeq
	return tea.Tick(kindDebounce, func(time.Time) tea.Msg { return kindTickMsg{seq: seq} })
}

// startWatch starts watching the kind panel 1 shows.
func (m *Model) startWatch() tea.Cmd {
	if m.watcher == nil || m.client == nil {
		return nil
	}
	m.watcher.Start(m.current, m.client.Selection())
	return m.waitWatch()
}

// onWatchData takes a new list of the watched kind.
func (m *Model) onWatchData(msg watchDataMsg) tea.Cmd {
	if msg.kind != m.current {
		return nil // a list from before the kind changed; its watch is gone
	}
	m.listP.Loading, m.listP.Fail = false, nil
	m.rootItems = filterHelm(msg.items, msg.kind)
	sortDefault(m.rootItems)
	cmds := []tea.Cmd{m.waitWatch()}
	if len(m.drill) > 0 {
		return tea.Batch(cmds...)
	}
	if m.anchor != nil && indexOfKey(m.rootItems, m.anchor.key) < 0 {
		m.anchor = nil
		cmds = append(cmds, m.toastWarn("compare: locked item gone"))
	}
	m.setListRows()
	if p := m.pendingSel; p != nil && p.Type == m.current {
		for _, it := range m.rootItems {
			if it.Name == p.Name && it.Namespace == p.Namespace {
				m.list.Select(itemKey(it))
				break
			}
		}
		m.pendingSel = nil
	}
	// The row's details come again with every change: its relatives,
	// conditions and events move with the cluster.
	if item, ok := m.currentItem(); ok && itemKey(item) == m.det.uid && m.det.loaded {
		cmds = append(cmds, fetchDetail(m.client, m.current, item), m.ensureLogs(item))
	}
	return tea.Batch(cmds...)
}

// filterHelm leaves out what a Helm chart manages while the user hides it
// (the . key), and the release records among the Secrets.
func filterHelm(items []k8s.ResourceItem, rt k8s.ResourceType) []k8s.ResourceItem {
	if !k8s.HelmHideManaged() || rt == k8s.ResourceReleases {
		return items
	}
	out := make([]k8s.ResourceItem, 0, len(items))
	for _, it := range items {
		if k8s.IsHelmManaged(it) || (rt == k8s.ResourceSecrets && k8s.IsHelmStorageSecret(it)) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// sortDefault is the order before any sort the user chose: by namespace,
// then by name.
func sortDefault(items []k8s.ResourceItem) {
	slices.SortStableFunc(items, func(a, b k8s.ResourceItem) int {
		if c := strings.Compare(a.Namespace, b.Namespace); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
}

// columnsOf are panel 2's columns for a kind: a namespaced kind starts with
// Namespace, then Name, then a column without a title for the Helm mark;
// Helm releases keep their own.
func columnsOf(rt k8s.ResourceType) []k8s.Column {
	def := k8s.DefaultRegistry.Get(rt)
	if def == nil {
		return nil
	}
	cols := def.Columns
	if rt == k8s.ResourceReleases || len(cols) == 0 {
		return cols
	}
	own := -1
	if !def.ClusterScoped {
		own = slices.IndexFunc(cols, func(c k8s.Column) bool { return c.Title == "Namespace" })
	}
	out := make([]k8s.Column, 0, len(cols)+2)
	if !def.ClusterScoped {
		out = append(out, k8s.Column{Title: "Namespace"})
	}
	out = append(out, cols[0], k8s.Column{Title: ""})
	for i := 1; i < len(cols); i++ {
		if i != own {
			out = append(out, cols[i])
		}
	}
	return out
}

// rowCells are an item's cells in columnsOf's order.
func rowCells(rt k8s.ResourceType, item k8s.ResourceItem) []string {
	def := k8s.DefaultRegistry.Get(rt)
	if def == nil || rt == k8s.ResourceReleases || len(item.Row) == 0 {
		return slices.Clone(item.Row)
	}
	own := -1
	if !def.ClusterScoped {
		own = slices.IndexFunc(def.Columns, func(c k8s.Column) bool { return c.Title == "Namespace" })
	}
	out := make([]string, 0, len(item.Row)+2)
	if !def.ClusterScoped {
		out = append(out, item.Namespace)
	}
	out = append(out, item.Row[0], k8s.MarkHelm(item))
	for j := 1; j < len(item.Row); j++ {
		if j != own {
			out = append(out, item.Row[j])
		}
	}
	return out
}

// numberColumns are the columns that hold a count (or an age): right-aligned
// and sorted by value (components/layout/panel).
var numberColumns = map[string]bool{"Restarts": true, "Desired": true, "Current": true, "Up-to-date": true,
	"Available": true, "Active": true, "Rev": true, "Age": true, "Updated": true, "Data": true, "Secrets": true,
	"Min": true, "Max": true, "Replicas": true, "Allowed": true}

// keptColumns keep their width on a narrow panel: what a row is and how it
// is doing. The others give up room first (an IP, a node, an image), and on
// a panel too narrow for every column they are left out (tdp L1).
var keptColumns = map[string]bool{"Namespace": true, "Name": true, "": true, "Ready": true, "Status": true,
	"Restarts": true, "Age": true, "Type": true, "Reason": true}

// listColumns are the lib's columns for panel 2's kind. A sorted column
// keeps its width: its sort arrow must stay in sight (components/layout/panel).
// Of Namespace and Name, Name gives up room when nameGives says it can,
// else Namespace.
func listColumns(cols []k8s.Column, sorted map[string]bool, nameGives bool) []terminu.Column {
	out := make([]terminu.Column, len(cols))
	for i, c := range cols {
		flex := !keptColumns[c.Title]
		if c.Title == "Namespace" || c.Title == "Name" {
			flex = (c.Title == "Name") == nameGives
		}
		flex = flex && !sorted[c.Title]
		out[i] = terminu.Column{Title: c.Title, Number: numberColumns[c.Title], Flex: flex}
	}
	return out
}

// nameGives is whether Name can give up all the room the columns in keep
// lack in w and still be its MinWidth wide. Then Name gives it, and
// Namespace, which tells same-named objects apart, keeps its width; past
// that, Namespace gives first (TODO(lib KB17): the lib cuts the column that
// gives to three cells at once). A sorted Name keeps its width.
func nameGives(cols []k8s.Column, cells [][]string, keep []int, w int, sorted map[string]bool) bool {
	total, name := 2*(len(keep)-1), -1
	for _, i := range keep {
		arrow := 0
		if sorted[cols[i].Title] {
			arrow = 4
		}
		total += naturalWidth(cols, cells, i, arrow)
		if cols[i].Title == "Name" {
			name = i
		}
	}
	if name < 0 || sorted["Name"] || w <= 0 {
		return false
	}
	return naturalWidth(cols, cells, name, 0)-(total-w) >= max(cols[name].MinWidth, 3)
}

// naturalWidth is column i's widest value or title, the title with arrow
// cells for a sort arrow.
func naturalWidth(cols []k8s.Column, cells [][]string, i, arrow int) int {
	n := terminu.Width(terminu.Upper(cols[i].Title)) + arrow
	for _, r := range cells {
		if i < len(r) {
			n = max(n, terminu.Width(r[i]))
		}
	}
	return n
}

// listWidth is how many cells panel 2's rows have now.
func (m *Model) listWidth() int {
	w, h := m.app.Size()
	if w == 0 {
		return 0
	}
	if m.app.ForceNarrow || w < m.screen.NarrowBelow {
		return w - 4
	}
	return layout(w, h)[1].W - 4
}

// fitColumns leaves out the columns that give up room first, from the
// right, while the natural widths do not fit in w (TODO(lib KB17): the lib
// cuts the widest column to three cells at once): a list of eight columns
// cut to three cells each says nothing (tdp L1). A sorted column stays.
func fitColumns(cols []k8s.Column, cells [][]string, w int, sorted map[string]bool) []int {
	keep := make([]int, len(cols))
	for i := range keep {
		keep[i] = i
	}
	if w <= 0 {
		return keep
	}
	natural := func(i int) int { return naturalWidth(cols, cells, i, 4) } // room for a sort arrow
	total := func() int {
		t := 2 * (len(keep) - 1)
		for _, i := range keep {
			t += natural(i)
		}
		return t
	}
	for k := len(keep) - 1; k >= 0 && total() > w; k-- {
		if c := cols[keep[k]].Title; !keptColumns[c] && !sorted[c] {
			keep = append(keep[:k], keep[k+1:]...)
		}
	}
	return keep
}

// setListRows lays panel 2 out for what it shows now: its label, columns,
// rows, sort and actions.
func (m *Model) setListRows() {
	if m.atContainers() {
		lv := m.drill[len(m.drill)-1]
		m.listP.Label = capRunes(lv.parent.Name, 20) + " > Containers"
		m.list.Columns = []terminu.Column{{Title: "Name", Flex: true}, {Title: "Image", Flex: true}, {Title: "State"},
			{Title: "Ready"}, {Title: "Restarts", Number: true}}
		rows := make([]terminu.Row, len(lv.containers))
		for i, c := range lv.containers {
			name := c.Name
			if c.Init {
				name = "(init) " + name
			}
			rows[i] = terminu.Row{ID: c.Name, Cells: []string{name, c.Image, c.State, strconv.FormatBool(c.Ready), strconv.Itoa(c.Restarts)}}
		}
		m.list.SortBy()
		m.list.SetRows(rows)
		m.setListActions()
		return
	}
	rt := m.shownKind()
	m.listP.Label = rt.String()
	if n := len(m.drill); n > 0 {
		m.listP.Label = capRunes(m.drill[n-1].parent.Name, 20) + " > " + rt.String()
	}
	all := columnsOf(rt)
	sorted := map[string]bool{}
	for _, t := range m.cfg.GetSort(rt.KubectlName()) {
		sorted[t.Column] = true
	}
	items := m.shownItems()
	allCells := make([][]string, len(items))
	for i, it := range items {
		allCells[i] = rowCells(rt, it)
	}
	keep := fitColumns(all, allCells, m.listWidth(), sorted)
	cols := make([]k8s.Column, len(keep))
	for i, c := range keep {
		cols[i] = all[c]
	}
	m.list.Columns = listColumns(cols, sorted, nameGives(all, allCells, keep, m.listWidth(), sorted))
	rows := make([]terminu.Row, len(items))
	current := ""
	if rt == k8s.ResourceContexts && m.client != nil {
		current = m.client.ContextName()
	}
	for i, it := range items {
		cells := make([]string, len(keep))
		for j, c := range keep {
			if c < len(allCells[i]) {
				cells[j] = allCells[i][c]
			}
		}
		sortVals := make([]string, len(cells))
		for c := range cells {
			title := ""
			if c < len(cols) {
				title = cols[c].Title
			}
			sortVals[c] = sortValue(title, it, cells[c])
			cells[c] = m.colorCell(rt, title, cells[c])
		}
		if current != "" && it.Name == current && len(cells) > 0 {
			cells[0] += " *"
		}
		if m.anchor != nil && itemKey(it) == m.anchor.key && len(cells) > 0 {
			// The anchor's row is the user's footprint (components/color):
			// its name in Lavender.
			name := 0
			if len(cols) > 1 && cols[0].Title == "Namespace" {
				name = 1
			}
			cells[name] = terminu.Fg(terminu.Lavender).Render(terminu.StripStyles(cells[name]))
		}
		rows[i] = terminu.Row{ID: itemKey(it), Cells: cells, Sort: sortVals}
	}
	m.list.EmptyFact = "No " + strings.ToLower(rt.String())
	m.list.EmptyHint = "Press [N] to pick other namespaces, or [Space] to see what you can do here"
	if def := k8s.DefaultRegistry.Get(rt); def != nil && def.ClusterScoped {
		m.list.EmptyHint = "Press [Space] to see what you can do here"
	}
	m.list.SortBy()
	m.list.SetRows(rows)
	m.list.SortBy(m.sortKeys(rt, cols)...)
	m.setListActions()
}

// sortKeys turn the kind's saved sort into the lib's: a column the kind no
// longer has is skipped.
func (m *Model) sortKeys(rt k8s.ResourceType, cols []k8s.Column) []terminu.SortKey {
	var keys []terminu.SortKey
	for _, t := range m.cfg.GetSort(rt.KubectlName()) {
		if i := slices.IndexFunc(cols, func(c k8s.Column) bool { return c.Title == t.Column }); i >= 0 {
			keys = append(keys, terminu.SortKey{Col: i, Desc: t.Direction == config.SortDirectionDescending})
		}
	}
	return keys
}

// sortValue is what a cell sorts by: an age by when the object was made
// (oldest first going up, as kubectl's --sort-by), "N/M" by both numbers,
// the rest by what it shows.
func sortValue(title string, item k8s.ResourceItem, cell string) string {
	switch title {
	case "Age":
		if obj, err := meta.Accessor(item.Raw); err == nil {
			return strconv.FormatInt(obj.GetCreationTimestamp().Unix(), 10)
		}
	case "Ready":
		if a, b, ok := strings.Cut(strings.TrimSpace(cell), "/"); ok {
			x, _ := strconv.Atoi(a)
			y, _ := strconv.Atoi(b)
			return fmt.Sprintf("%09d/%09d", x, y)
		}
	}
	return terminu.StripStyles(cell)
}

// colorCell colours what needs a look in a status cell: transitional or
// degraded in the pending colour, failures in the error colour (the app's
// content, tdp P4). Healthy stays plain.
func (m *Model) colorCell(rt k8s.ResourceType, title, cell string) string {
	raw := strings.TrimSpace(cell)
	color := ""
	switch {
	case title == "Status":
		color = statusColor(raw, m.status.Pending, m.status.Error)
	case title == "Type" && rt == k8s.ResourceEvents && raw == "Warning":
		color = m.status.Error
	}
	if color == "" {
		return cell
	}
	return terminu.Fg(terminu.Hex(color)).Render(cell)
}

// statusColor is the colour of a status value, "" for a healthy or unknown
// one.
func statusColor(status, pending, failed string) string {
	switch status {
	case "Pending", "ContainerCreating", "PodInitializing", "Init:PodInitializing",
		"Terminating", "SchedulingDisabled", "Released",
		"pending-install", "pending-upgrade", "pending-rollback", "uninstalling":
		return pending
	case "CrashLoopBackOff", "Error", "ImagePullBackOff", "ErrImagePull",
		"CreateContainerConfigError", "CreateContainerError",
		"InvalidImageName", "Evicted", "OOMKilled", "Failed",
		"Lost", "NotReady", "failed":
		return failed
	}
	if strings.HasPrefix(status, "Init:") {
		return failed
	}
	return ""
}

// capRunes fits s in n cells, cutting its end (tdp E5: by the display
// width).
func capRunes(s string, n int) string { return terminu.Truncate(s, n) }

// listEsc is Esc on panel 2, one level at a time (tdp K4): the compare
// anchor, then the drill.
func (m *Model) listEsc() tea.Cmd {
	switch {
	case m.anchor != nil:
		m.clearAnchor()
		return nil
	case len(m.drill) > 0:
		return m.drillOut()
	}
	return nil
}

// allowsEdit and allowsDelete: kbu never runs kubectl edit or delete on
// what is not a cluster object kubectl changes (events, contexts, Helm
// releases), nor deletes a node.
func allowsEdit(rt k8s.ResourceType) bool {
	switch rt {
	case k8s.ResourceEvents, k8s.ResourceContexts, k8s.ResourceReleases, "":
		return false
	}
	return true
}

func allowsDelete(rt k8s.ResourceType) bool {
	switch rt {
	case k8s.ResourceEvents, k8s.ResourceNodes, k8s.ResourceContexts, k8s.ResourceReleases, "":
		return false
	}
	return true
}

// setListActions lays out panel 2's actions for what it shows (tdp M2, tdp M3):
// the names and descriptions say what they do to the row.
func (m *Model) setListActions() {
	rt := m.shownKind()
	cur, hasCur := m.currentItem()
	helmManaged := func() bool {
		it, ok := m.currentItem()
		return ok && k8s.IsHelmManaged(it)
	}
	var item []terminu.Action
	switch {
	case m.atContainers():
		shell := m.shellAsk()
		item = []terminu.Action{
			{Key: "Enter", Label: "Open a shell", Desc: "kubectl exec -it into this container, as [S] does", Scope: terminu.ItemScope,
				Hint: "shell", Ask: shell, Do: m.runShell},
			{Key: "S", Label: "Shell", Desc: "kubectl exec -it into this container", Scope: terminu.ItemScope, Ask: shell, Do: m.runShell},
			{Key: "y", Label: "Copy", Desc: "this row, tab-separated", Scope: terminu.ItemScope, Do: m.copyRow},
		}
	case rt == k8s.ResourceReleases:
		item = []terminu.Action{
			{Key: "Enter", Label: "Open", Desc: "the release record, as [Y] does", Scope: terminu.ItemScope, Hint: "yaml", Stay: true, Do: m.yamlHere},
			{Key: "Y", Label: "YAML", Desc: "the release record", Scope: terminu.ItemScope, Stay: true, Do: m.yamlHere},
		}
		for _, d := range helmDocs {
			d := d
			item = append(item, terminu.Action{Label: d.label, Desc: d.desc, Scope: terminu.ItemScope, Stay: true,
				Do: func() tea.Cmd { return m.fetchHelmDoc(d) }})
		}
		item = append(item, terminu.Action{Key: "y", Label: "Copy", Desc: "this row, tab-separated", Scope: terminu.ItemScope, Do: m.copyRow})
	default:
		if rt == k8s.ResourceContexts {
			item = append(item, terminu.Action{Key: "Enter", Label: "Switch to this context", Desc: "kbu only; ~/.kube/config is not changed",
				Scope: terminu.ItemScope, Hint: "switch", Ask: contextAsk(cur),
				Disabled: func() bool {
					it, ok := m.currentItem()
					return !ok || m.client == nil || it.Name == m.client.ContextName()
				},
				Do: func() tea.Cmd { return m.switchContext(cur.Name) }})
		}
		item = append(item,
			terminu.Action{Key: "C", Label: "Compare to anchor", Desc: "open the YAML diff popup", Scope: terminu.ItemScope,
				Hidden: func() bool { return !m.canCompare() }, Stay: true, Do: m.compareHere},
		)
		switch {
		case rt.SupportsDrillDown():
			item = append(item, terminu.Action{Key: "Enter", Label: "Drill in", Desc: "into its " + drillLabel(rt, cur),
				Scope: terminu.ItemScope, Hint: "drill", Do: m.drillIn})
		case rt != k8s.ResourceContexts:
			item = append(item, terminu.Action{Key: "Enter", Label: "Open", Desc: "its YAML, as [Y] does", Scope: terminu.ItemScope,
				Hint: "yaml", Stay: true, Do: m.yamlHere})
		}
		item = append(item, terminu.Action{Key: "Y", Label: "YAML", Desc: "view resource manifest", Scope: terminu.ItemScope, Stay: true, Do: m.yamlHere})
		if allowsEdit(rt) && hasCur {
			// TODO(lib S4): an edit keeps the menu it was run from (tdp T1), but
			// an action that asks clears the whole stack when accepted.
			item = append(item, terminu.Action{Key: "E", Label: "Edit", Desc: "kubectl edit", Scope: terminu.ItemScope,
				Disabled: helmManaged, Ask: m.editAsk(rt, cur), Do: func() tea.Cmd { return m.runEdit(rt, cur, nil) }})
		}
		if rt == k8s.ResourcePods && hasCur {
			item = append(item, terminu.Action{Key: "S", Label: "Shell", Desc: "kubectl exec -it", Scope: terminu.ItemScope,
				Hidden: func() bool { return m.shellAsk() == nil }, Ask: m.shellAsk(), Do: m.runShell})
		}
		if allowsDelete(rt) && hasCur {
			item = append(item, terminu.Action{Key: "D", Label: "Delete", Desc: "kubectl delete", Scope: terminu.ItemScope,
				Disabled: helmManaged, Ask: m.deleteAsk(rt, cur), Do: func() tea.Cmd { return m.runDelete(rt, cur) }})
		}
		item = append(item,
			terminu.Action{Key: "C", Label: "Unmark Compare anchor", Desc: "cancel anchor, exit compare mode", Scope: terminu.ItemScope,
				Hidden: func() bool { return !m.onAnchor() }, Do: func() tea.Cmd { m.clearAnchor(); return nil }},
			terminu.Action{Key: "C", Label: "Mark as Compare anchor", Desc: "set this row as the diff baseline", Scope: terminu.ItemScope,
				Hidden:   func() bool { return m.anchor != nil },
				Disabled: func() bool { return len(m.shownItems()) < 2 },
				Do:       m.markAnchor},
			terminu.Action{Key: "y", Label: "Copy", Desc: "this row, tab-separated", Scope: terminu.ItemScope, Do: m.copyRow},
		)
	}
	panel := []terminu.Action{
		{Label: "Exit compare mode", Desc: "drop the compare anchor ([Esc])", Scope: terminu.PanelScope,
			Hidden: func() bool { return m.anchor == nil }, Do: func() tea.Cmd { m.clearAnchor(); return nil }},
		{Label: "Back", Desc: "to the " + m.drillParentLabel() + " list ([Esc])", Scope: terminu.PanelScope,
			Hidden: func() bool { return len(m.drill) == 0 }, Do: m.drillOut},
		{Key: "Alt-S", Label: "Sort panel 2 list", Desc: "order the rows by a column", Scope: terminu.PanelScope,
			Hidden: func() bool { return m.atContainers() || len(columnsOf(m.shownKind())) == 0 },
			Do:     func() tea.Cmd { return m.openSort(m.shownKind()) }},
		{Key: "/", Label: "Search", Desc: "narrow the rows as you type", Scope: terminu.PanelScope, Hint: "search",
			Do: func() tea.Cmd { m.list.OpenFilter(); return nil }},
		{Key: ".", Label: "Helm-managed rows", Desc: "show / hide rows a Helm chart manages", Scope: terminu.PanelScope, Hint: "helm",
			Hidden: func() bool {
				k := m.shownKind()
				return m.atContainers() || k == k8s.ResourceReleases || k == k8s.ResourceContexts
			}, Do: m.toggleHelm},
		{Key: "z", Label: "Zoom", Desc: "full-screen this panel", Scope: terminu.PanelScope, Do: m.toggleZoom},
	}
	m.listP.Actions = append(item, panel...)
	// Esc is one level up only when there is one (tdp K4): the anchor,
	// then the drill; otherwise ? does not list it.
	m.listP.OnEsc = nil
	if m.anchor != nil || len(m.drill) > 0 {
		m.listP.OnEsc = m.listEsc
	}
}

// drillLabel is what Enter drills item into, for the menu: an HPA into its
// target's kind.
func drillLabel(rt k8s.ResourceType, item k8s.ResourceItem) string {
	if rt == k8s.ResourcePods {
		return "containers"
	}
	if def := k8s.DefaultRegistry.Get(rt); def != nil && def.DrillDown != nil {
		if child := def.DrillDown.ChildTypeFor(item); child != "" {
			return child.KubectlName() + "s"
		}
	}
	return "children"
}

// drillParentLabel names the list Back goes to.
func (m *Model) drillParentLabel() string {
	if n := len(m.drill); n > 0 {
		return m.drill[n-1].parentKind.String()
	}
	return m.current.String()
}

// drillIn goes into what the row under the cursor runs: a pod's containers,
// a workload's pods, a CronJob's jobs. Its children are a snapshot, taken
// once.
func (m *Model) drillIn() tea.Cmd {
	item, ok := m.currentItem()
	if !ok {
		return nil
	}
	rt := m.shownKind()
	if rt == k8s.ResourcePods {
		d := k8s.GetResourceDetail(k8s.ResourcePods, item)
		m.drill = append(m.drill, drillLevel{parentKind: rt, parent: item, containers: d.Containers})
		m.setListRows()
		m.rowSeq++
		m.det.uid = ""
		return m.rowMovedNow()
	}
	if !rt.SupportsDrillDown() || m.client == nil {
		return nil
	}
	cs := m.client.Clientset()
	return func() tea.Msg {
		kind, children, err := k8s.FetchChildResources(context.Background(), cs, rt, item)
		return childrenMsg{parent: item, kind: kind, children: children, err: err}
	}
}

// onChildren shows what a drill found, or says why there is nothing (tdp
// F5: a failure is seen at once; an empty result is not an ignored Enter).
func (m *Model) onChildren(msg childrenMsg) tea.Cmd {
	if msg.err != nil {
		return m.toastErr(fmt.Sprintf("drill failed: %v", msg.err))
	}
	if len(msg.children) == 0 {
		what := "children"
		if msg.kind != "" {
			what = msg.kind.String()
		}
		return m.app.Toast(terminu.Info, "no "+what+" under "+msg.parent.Name)
	}
	items := filterHelm(msg.children, msg.kind)
	sortDefault(items)
	m.drill = append(m.drill, drillLevel{parentKind: m.shownKind(), parent: msg.parent, kind: msg.kind, items: items})
	m.anchor = nil
	m.stopLogs()
	m.list.ClearFilter()
	m.setListRows()
	m.resetDetail(msg.kind)
	return m.rowMovedNow()
}

// drillOut goes back up one level, the cursor on the row it went into.
func (m *Model) drillOut() tea.Cmd {
	n := len(m.drill)
	if n == 0 {
		return nil
	}
	back := m.drill[n-1].parent
	m.drill = m.drill[:n-1]
	m.anchor = nil
	m.stopLogs()
	m.list.ClearFilter()
	m.setListRows()
	m.list.Select(itemKey(back))
	m.resetDetail(m.shownKind())
	return m.rowMovedNow()
}

// --- compare --------------------------------------------------------------

func (m *Model) onAnchor() bool {
	r, ok := m.list.Current()
	return ok && m.anchor != nil && r.ID == m.anchor.key
}

func (m *Model) canCompare() bool {
	r, ok := m.list.Current()
	return ok && m.anchor != nil && m.anchor.kind == m.shownKind() && r.ID != m.anchor.key
}

func (m *Model) markAnchor() tea.Cmd {
	item, ok := m.currentItem()
	if !ok || len(m.shownItems()) < 2 {
		return nil
	}
	m.anchor = &anchorRef{kind: m.shownKind(), key: itemKey(item), item: item}
	m.log.info(fmt.Sprintf("compare: anchor set on %s/%s", m.shownKind().KubectlName(), item.Name))
	m.setListRows()
	return nil
}

func (m *Model) clearAnchor() {
	if m.anchor == nil {
		return
	}
	m.anchor = nil
	m.log.info("compare: anchor cleared")
	m.setListRows()
}

// --- the helm filter and zoom ----------------------------------------------

func (m *Model) toggleHelm() tea.Cmd {
	hidden := k8s.ToggleHelmHideManaged()
	state := "shown"
	if hidden {
		state = "hidden"
	}
	m.rootItems = filterHelm(m.rootItems, m.current)
	m.setListRows()
	cmds := []tea.Cmd{m.app.Toast(terminu.Info, "Helm-managed rows "+state)}
	if m.watcher != nil && len(m.drill) == 0 {
		m.watcher.Stop()
		cmds = append(cmds, m.startWatch())
	}
	return tea.Batch(cmds...)
}

// toggleZoom draws only the focused panel, across the whole screen, or puts
// the three back. It is a change of layout, not a mode: z puts it back.
func (m *Model) toggleZoom() tea.Cmd {
	m.app.ForceNarrow = !m.app.ForceNarrow
	m.setListRows() // panel 2's width changed: which columns fit
	return nil
}
