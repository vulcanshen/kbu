package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
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
		m.setDetail(msg.detail, msg.events)
		if msg.err != nil {
			// An empty tab says why it is empty (components/layout/panel,
			// errors); the App Log keeps it.
			fact, _, _ := strings.Cut(msg.err.Error(), "\n")
			m.log.warn("Details: " + fact)
			m.detailP.Fail = &terminu.Failure{Fact: fact, Hint: "kbu asks again with the next change; [!] shows the App Log"}
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

// onWatchErr is the watch failing: the App Log keeps it; the first time, a
// toast says it (tdp F5), and an empty panel 2 says why it is empty
// (components/layout/panel, errors). The watcher tries again on its own.
func (m *Model) onWatchErr(err error) tea.Cmd {
	text := capital(err.Error()) // "listing pods: …", as Go errors are written
	m.log.err(text)
	cmds := []tea.Cmd{m.waitWatch()}
	if m.listP.Fail == nil {
		fact, _, _ := strings.Cut(text, "\n")
		m.listP.Loading = false
		m.listP.Fail = &terminu.Failure{Fact: fact, Hint: "kbu tries again on its own; [!] shows the App Log"}
		cmds = append(cmds, m.app.Toast(terminu.Error, fact))
	}
	return tea.Batch(cmds...)
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
		m.log.warn("CRD discovery: " + msg.err.Error())
		return nil
	}
	if msg.count > 0 {
		m.log.info(fmt.Sprintf("Discovered %d CRDs", msg.count))
	}
	m.pinned = m.loadPinned()
	m.refreshKinds()
	// A saved kind that is a CRD is known only now.
	if m.state.Kind != "" && m.current == k8s.ResourcePods && m.state.Kind != k8s.ResourcePods.KubectlName() {
		if rt := k8s.DefaultRegistry.LookupByKubectlName(m.state.Kind); rt != "" {
			m.kinds.Select(string(rt))
			return m.selectKind(rt, false)
		}
	}
	return nil
}

// onNamespaces fills the namespace picker, or, at start, drops the saved
// namespaces that are gone.
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
			return m.app.Error("Namespaces not read", msg.err.Error())
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
	m.log.info(fmt.Sprintf("Namespace selection: %d no longer exist, now %s", sel.Count()-kept.Count(), selectionText(kept)))
	return m.setNamespaces(kept)
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
			m.log.info(fmt.Sprintf("State: recorded kind %q not known yet, starting on Pods", s.Kind))
		}
		m.startTab, m.startPanel = s.Tab, s.Panel
	}
	m.refreshKinds()
	m.kinds.Select(string(m.kindAt))
}

// sessionState is where the user is, for state.yaml.
func (m *Model) sessionState() *config.State {
	s := &config.State{Kind: m.current.KubectlName(), Tab: m.currentTab(), CompareSplit: m.compareSplit}
	if m.client != nil {
		s.Context = m.client.ContextName()
		s.Namespaces = m.client.Selection().List()
	}
	if len(m.drill) == 0 {
		if item, ok := m.currentItem(); ok {
			s.ObjectName, s.ObjectNamespace = item.Name, item.Namespace
		}
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
