package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/v4/internal/config"
	"github.com/vulcanshen/kbu/v4/internal/k8s"
)

// The cluster's data arrives as messages the lib does not handle itself; they
// come to onMsg. Every fetch runs off the update loop; a result that arrives
// after the user moved on is dropped by its sequence number or UID.

// kindDebounce and rowDebounce are how long a cursor must rest on a kind or a
// row before kbu starts its watch or fetches its details: going down a list
// with j would otherwise start a watch, a fetch and a log stream per row.
const (
	kindDebounce = 300 * time.Millisecond
	rowDebounce  = 300 * time.Millisecond
	// aggregateRetry is how long a workload's logs wait before trying again
	// after they failed or found no pods: a watcher tick a second would
	// otherwise ask again every second.
	aggregateRetry = 10 * time.Second
)

type (
	kindTickMsg  struct{ seq int }
	rowTickMsg   struct{ seq int }
	watchDataMsg struct {
		kind  k8s.ResourceType
		items []k8s.ResourceItem
	}
	watchErrMsg struct{ err error }
	detailMsg   struct {
		uid    string
		detail k8s.ResourceDetail
		events []k8s.EventItem
		err    error // the events could not be had
	}
	logLineMsg struct{ line k8s.LogLine }
	podsMsg    struct {
		uid     string
		targets []k8s.PodTarget
		err     error
	}
	namespacesMsg struct {
		names []string
		err   error
		check bool // the start-up check of the saved selection, not the picker
	}
	crdsMsg struct {
		count int
		err   error
	}
	childrenMsg struct {
		parent   k8s.ResourceItem
		kind     k8s.ResourceType
		children []k8s.ResourceItem
		err      error
	}
	relativeMsg struct {
		ref       k8s.RefTarget
		sourceUID string
		item      k8s.ResourceItem
		detail    k8s.ResourceDetail
		err       error
	}
	refYAMLMsg struct {
		ref  k8s.RefTarget
		item k8s.ResourceItem
		err  error
	}
	helmDocMsg struct {
		label, release, namespace, content string
		err                                error
	}
	commandDoneMsg struct {
		what   string // "delete", "rollback"
		target string
		out    string
		err    error
	}
	copiedMsg struct {
		lines int
		err   error
	}
)

// onMsg handles what the lib passes on.
func (m *Model) onMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case kindTickMsg:
		if msg.seq != m.kindSeq {
			return nil
		}
		return m.startWatch()
	case watchDataMsg:
		return m.onWatchData(msg)
	case watchErrMsg:
		return m.onWatchErr(msg.err)
	case rowTickMsg:
		if msg.seq != m.rowSeq {
			return nil
		}
		return m.loadRow()
	case detailMsg:
		if msg.uid == "" || msg.uid != m.det.uid {
			return nil
		}
		if msg.err != nil && m.det.got {
			msg.events = m.det.events // read again: what the tab has stays (tdp panel.15)
		}
		m.setDetail(msg.detail, msg.events)
		if msg.err != nil {
			// The details are read again with every change: an Error toast
			// the first time, again only for another failure (tdp note.7);
			// a tab that had nothing yet says why it is empty (tdp panel.15).
			if !m.det.got {
				fact, _, _ := strings.Cut(msg.err.Error(), "\n")
				m.detailP.Fail = &terminu.Failure{Fact: fact, Hint: "kbu asks again with the next change; [!] shows the App Log"}
			}
			if text := "Details: " + msg.err.Error(); m.detailErr.failed(text) {
				return m.toast(terminu.Error, text)
			}
			return nil
		}
		m.det.got = true
		if m.detailErr.ok() {
			m.log.info("The details can be read again")
		}
		return nil
	case logLineMsg:
		if m.logs == nil || msg.line.StreamID != m.logs.CurrentStreamID() {
			return nil
		}
		m.appendLog(msg.line)
		return waitLogLine(m.logs)
	case podsMsg:
		return m.onPods(msg)
	case namespacesMsg:
		return m.onNamespaces(msg)
	case crdsMsg:
		return m.onCRDs(msg)
	case childrenMsg:
		return m.onChildren(msg)
	case relativeMsg:
		return m.onRelative(msg)
	case rereadMsg:
		return m.onReread(msg)
	case refYAMLMsg:
		if msg.err != nil {
			return m.toastErr(fmt.Sprintf("YAML of %s/%s: %v", msg.ref.Type.KubectlName(), msg.ref.Name, msg.err))
		}
		return m.openYAML(msg.ref.Type, msg.item, k8s.MarshalItemYAML(msg.item))
	case helmDocMsg:
		return m.onHelmDoc(msg)
	case commandDoneMsg:
		return m.onCommandDone(msg)
	case copiedMsg:
		if msg.err != nil {
			// Copying is done from a panel or a viewer: what failed is shown
			// in full (tdp F5, tdp F7: an action's failure is an error popup).
			return m.app.Error("Copy failed", msg.err.Error())
		}
		return m.toastDone("Copied " + plural(msg.lines, "line"))
	}
	return nil
}

// waitWatch waits for the watcher's next list or error.
func (m *Model) waitWatch() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	updates, errs := m.watcher.Channels()
	kind := m.current
	return func() tea.Msg {
		select {
		case u, ok := <-updates:
			if !ok {
				return nil // the watcher was restarted; its new run has its own wait
			}
			return watchDataMsg{kind: kind, items: u.Items}
		case e, ok := <-errs:
			if !ok {
				return nil
			}
			return watchErrMsg{err: e.Err}
		}
	}
}

// onWatchErr is the watch failing; the watcher tries again on its own. An
// Error toast says it the first time, and again only for another failure:
// the same one again is neither shown nor logged (tdp note.7). The rows
// panel 2 has stay; an empty one says why it is empty (tdp panel.15).
func (m *Model) onWatchErr(err error) tea.Cmd {
	// The watcher's own "listing pods: " or "watching pods: " starts it, the
	// API's words after it kept as they are (tdp toast.2).
	text := capital(err.Error())
	cmds := []tea.Cmd{m.waitWatch()}
	if m.watchErr.failed(text) {
		m.listP.Loading = false
		if len(m.rootItems) == 0 {
			fact, _, _ := strings.Cut(text, "\n")
			m.listP.Fail = &terminu.Failure{Fact: fact, Hint: "kbu tries again on its own; [!] shows the App Log"}
		}
		cmds = append(cmds, m.toast(terminu.Error, text))
	}
	return tea.Batch(cmds...)
}

// rereadErr is the last failure of what kbu reads again on its own, so that
// the same failure again says nothing (tdp note.7).
type rereadErr struct{ last string }

// failed reports whether fact is a new failure: the first, or another one.
func (r *rereadErr) failed(fact string) bool {
	if fact == r.last {
		return false
	}
	r.last = fact
	return true
}

// ok reports whether reading works again after a failure.
func (r *rereadErr) ok() bool {
	failed := r.last != ""
	r.last = ""
	return failed
}

// helmChanged is what the poll found changed in the Helm releases since it
// last read them, for one INFO entry (tdp note.7), "" when nothing did or
// when it reads them for the first time.
func (m *Model) helmChanged(items []k8s.ResourceItem) string {
	seen := map[string]string{}
	for _, it := range items {
		if r, ok := it.Raw.(*k8s.Release); ok {
			seen[diffLabel(it)] = "revision " + r.Revision + ", " + r.Status
		}
	}
	before := m.helmSeen
	m.helmSeen = seen
	if before == nil {
		return ""
	}
	var what []string
	for name, now := range seen {
		switch was, ok := before[name]; {
		case !ok:
			what = append(what, name+" installed ("+now+")")
		case was != now:
			what = append(what, name+" now "+now)
		}
	}
	for name := range before {
		if _, ok := seen[name]; !ok {
			what = append(what, name+" uninstalled")
		}
	}
	slices.Sort(what)
	return strings.Join(what, "; ")
}

// fetchDetail fetches what panel 3 shows of item: its details, its relatives
// that need a call to the API, its conditions and its events.
func fetchDetail(client *k8s.Client, kind k8s.ResourceType, item k8s.ResourceItem) tea.Cmd {
	if client == nil {
		return nil
	}
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				config.WriteCrashLog(r)
			}
		}()
		ctx := context.Background()
		d := k8s.GetResourceDetail(kind, item)
		d.YAML = k8s.MarshalItemYAML(item)
		k8s.EnrichRelatives(ctx, client.Clientset(), kind, item, &d)
		d.Conditions = k8s.ExtractConditions(item)
		events, err := k8s.FetchResourceEventsAggregated(ctx, client.Clientset(), item)
		return detailMsg{uid: itemKey(item), detail: d, events: events, err: err}
	}
}

// fetchPods finds the pods a workload runs now, for its merged logs.
func fetchPods(client *k8s.Client, item k8s.ResourceItem) tea.Cmd {
	return func() tea.Msg {
		targets, err := k8s.PodsForWorkload(context.Background(), client.Clientset(), item, true)
		return podsMsg{uid: itemKey(item), targets: targets, err: err}
	}
}

// waitLogLine waits for the next log line. The channel is taken now, not
// in the goroutine: a Stop in between would leave the goroutine reading a
// nil channel for ever.
func waitLogLine(ls *k8s.LogStreamer) tea.Cmd {
	if ls == nil {
		return nil
	}
	ch := ls.Lines()
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return logLineMsg{line: line}
	}
}

func fetchNamespaces(client *k8s.Client, check bool) tea.Cmd {
	return func() tea.Msg {
		items, err := k8s.FetchResources(context.Background(), client.Clientset(), k8s.ResourceNamespaces, "")
		if err != nil {
			return namespacesMsg{err: err, check: check}
		}
		names := make([]string, len(items))
		for i, it := range items {
			names[i] = it.Name
		}
		return namespacesMsg{names: names, check: check}
	}
}

func discoverCRDs(client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				config.WriteCrashLog(r)
			}
		}()
		n, err := k8s.DiscoverCRDs(context.Background(), client)
		return crdsMsg{count: n, err: err}
	}
}

// onCRDs puts the CRDs found into panel 1.
func (m *Model) onCRDs(msg crdsMsg) tea.Cmd {
	if msg.err != nil {
		// panel 1 keeps the built-in kinds; the toast says why the custom
		// ones are not there (tdp panel.15, F5).
		return m.toast(terminu.Error, "CRD discovery: "+msg.err.Error())
	}
	if msg.count > 0 {
		m.log.info(fmt.Sprintf("Discovered %d CRDs", msg.count))
	}
	m.pinned = m.loadPinned()
	m.refreshKinds()
	// The recorded kind not known at start is known now, or not there:
	// then a Warning says so, once, and state.yaml no longer gets it (tdp
	// E3-41, E3-42). kbu stays where it is either way: discovery ending is
	// no reason to move (tdp E3, K3).
	if k := m.keepKind; k != nil && k8s.DefaultRegistry.LookupByKubectlName(k.Kind) == "" {
		m.keepKind = nil
		return m.app.Notify(terminu.Notice{Kind: terminu.Warning,
			Text: fmt.Sprintf("Kind %q in state.yaml is not in the cluster: showing Pods", k.Kind)})
	}
	return nil
}

// onNamespaces fills the namespace picker, or, at start, shows the saved
// namespaces the cluster has, a Warning naming those it has not, which
// state.yaml no longer gets (tdp E3-41).
func (m *Model) onNamespaces(msg namespacesMsg) tea.Cmd {
	if !msg.check {
		fill := m.fillNamespaces
		m.fillNamespaces = nil
		if fill == nil {
			return nil
		}
		fill(msg.names)
		if msg.err != nil {
			// The picker's own load failed: an error popup over it (tdp F7).
			return m.errorPopup("Namespaces not read", msg.err.Error())
		}
		return nil
	}
	if msg.err != nil {
		m.log.warn("Namespaces: " + msg.err.Error())
		return nil
	}
	if m.client == nil {
		return nil
	}
	sel := m.client.Selection()
	if sel.IsAll() {
		return nil
	}
	kept := sel.FilterExisting(msg.names)
	if kept.Count() == sel.Count() {
		return nil
	}
	var gone []string
	for _, n := range sel.List() {
		if kept.IsAll() || !kept.Contains(n) {
			gone = append(gone, fmt.Sprintf("%q", n))
		}
	}
	what := "Namespace " + gone[0] + " in state.yaml is"
	if len(gone) > 1 {
		what = "Namespaces " + strings.Join(gone, ", ") + " in state.yaml are"
	}
	// A toast of its own, stacked over the start's (tdp E3-37).
	late := terminu.Notice{Kind: terminu.Warning, Text: what + " not in the cluster: showing " + selectionText(kept)}
	return tea.Batch(m.app.Notify(late), m.showNamespaces(kept))
}

func selectionText(sel k8s.NamespaceSelection) string {
	if sel.IsAll() {
		return "All Namespaces"
	}
	return fmt.Sprint(sel.List())
}

// restore puts kbu back where the user left it (the session state): the
// kind, the row, the panel 3 tab and the panel — what can be had before
// the cluster answers.
func (m *Model) restore() {
	m.pinned = m.loadPinned()
	m.kindAt = k8s.ResourcePods
	if s := m.state; s != nil && s.Kind != "" {
		if rt := k8s.DefaultRegistry.LookupByKubectlName(s.Kind); rt != "" {
			m.kindAt = rt
			if s.ObjectName != "" {
				m.pendingSel = &k8s.RefTarget{Type: rt, Name: s.ObjectName, Namespace: s.ObjectNamespace}
			}
		} else {
			// Not known yet: a CRD, found only once discovery is done
			// (onCRDs). kbu starts on Pods meanwhile and keeps it.
			m.keepKind = &config.State{Kind: s.Kind, ObjectNamespace: s.ObjectNamespace, ObjectName: s.ObjectName, Tab: s.Tab}
		}
		m.startTab, m.startPanel = s.Tab, s.Panel
	}
	m.refreshKinds()
	m.kinds.Select(string(m.kindAt))
}

// sessionState is where the user is, for state.yaml.
func (m *Model) sessionState() *config.State {
	s := &config.State{Kind: m.current.KubectlName(), Tab: m.currentTab()}
	if m.client != nil {
		s.Context = m.client.ContextName()
		s.Namespaces = m.client.Selection().List()
	}
	if len(m.drill) == 0 {
		if item, ok := m.currentItem(); ok {
			s.ObjectName, s.ObjectNamespace = item.Name, item.Namespace
		}
	}
	if k := m.keepKind; k != nil {
		s.Kind, s.ObjectNamespace, s.ObjectName, s.Tab = k.Kind, k.ObjectNamespace, k.ObjectName, k.Tab
	}
	switch m.screen.Focused() {
	case m.listP:
		s.Panel = "table"
	case m.detailP:
		s.Panel = "detail"
	default:
		s.Panel = "sidebar"
	}
	return s
}

// itemKey is what a row of panel 2 is known by: its UID, or, for what has
// none (a context, a Helm release, a container), its namespace and name.
func itemKey(item k8s.ResourceItem) string {
	if item.UID != "" {
		return item.UID
	}
	return item.Namespace + "/" + item.Name
}

// indexOfKey finds a row by its key.
func indexOfKey(items []k8s.ResourceItem, key string) int {
	return slices.IndexFunc(items, func(it k8s.ResourceItem) bool { return itemKey(it) == key })
}
