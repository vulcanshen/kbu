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

	"github.com/vulcanshen/kbu/internal/k8s"
)

// Panel 3: the details of the row under panel 2's cursor, in tabs — Logs,
// Relatives, Events, Conditions, History, Info, as the kind has them. It
// follows panel 2's cursor at once (components/layout/panel, details).

const (
	glyphLive   = "\U000F0753" // nf-md-play_circle: following the newest lines
	glyphPaused = "\U000F0754" // nf-md-pause_circle: scrolled away from them
	glyphLink   = "\U0000F43E" // nf-oct-arrow_right: a relative Enter goes into
	maxLogLines = 1000
)

// logLine is one line of the Logs tab; pod is set for a workload's merged
// stream, so each line says which pod it came from.
type logLine struct{ pod, container, text string }

// relFrame is one level down the Relatives tab's chain.
type relFrame struct {
	ref    k8s.RefTarget
	item   k8s.ResourceItem
	detail k8s.ResourceDetail
}

// detailState is what panel 3 shows.
type detailState struct {
	kind   k8s.ResourceType // the kind of the row it shows ("" for a container)
	uid    string           // the row's key
	loaded bool             // its details have been asked for
	detail k8s.ResourceDetail
	events []k8s.EventItem
	tabs   []string

	logs      []logLine
	logsOn    bool
	nextRetry time.Time

	chain []relFrame

	logBody, eventBody, condBody, infoBody *textBody
}

func (m *Model) buildDetail() {
	d := &m.det
	d.logBody = newTextBody(m.renderLogs, "Waiting for logs…", true)
	d.eventBody = newTextBody(m.renderEvents, "No events", true)
	d.condBody = newTextBody(m.renderConditions, "No conditions", false)
	d.infoBody = newTextBody(m.renderInfo, "No detail", false)
	d.logBody.onLive = m.setTabLabels
	d.eventBody.onLive = m.setTabLabels
	m.relList = &terminu.List{Columns: []terminu.Column{{Title: "Name"}, {Title: "Target", Flex: true}},
		EmptyFact: "No relatives to show", EmptyHint: "Press [Y] for the full YAML"}
	m.histList = &terminu.List{Columns: []terminu.Column{{Title: ""}, {Title: "Rev", Number: true}, {Title: "Status"},
		{Title: "Date"}, {Title: "Chart"}, {Title: "Description", Flex: true}}, EmptyFact: "No revision history",
		EmptyHint: "Press [Space] to see what you can do here"}
	m.histList.OnMove = func(terminu.Row) tea.Cmd { m.setDetailActions(); return nil } // the rollback asks about this row
	m.detailP = &terminu.Panel{Label: "Relatives", OnEsc: m.detailEsc, LoadingText: "loading details…"}
	m.detailP.OnTab = func(int) tea.Cmd { m.showTab(); return nil }
	m.resetDetail(k8s.ResourcePods)
}

// tabsOf are the tabs a kind's details have.
func tabsOf(rt k8s.ResourceType) []string {
	var tabs []string
	switch rt {
	case k8s.ResourcePods, k8s.ResourceDeployments, k8s.ResourceStatefulSets, k8s.ResourceDaemonSets,
		k8s.ResourceJobs, k8s.ResourceCronJobs:
		tabs = []string{"Logs", "Relatives", "Events"}
	case "": // a container
		tabs = []string{"Logs"}
	case k8s.ResourceEvents:
		tabs = []string{"Relatives"}
	case k8s.ResourceReleases:
		tabs = []string{"Relatives", "History"}
	case k8s.ResourceContexts:
		tabs = []string{"Info"}
	case k8s.ResourceNamespaces:
		tabs = []string{"Events"}
	default:
		tabs = []string{"Relatives", "Events"}
	}
	switch rt {
	case k8s.ResourcePods, k8s.ResourceNodes, k8s.ResourcePersistentVolumeClaims, k8s.ResourceDeployments,
		k8s.ResourceStatefulSets, k8s.ResourceDaemonSets, k8s.ResourceJobs, k8s.ResourceHorizontalPodAutoscalers,
		k8s.ResourceIngresses:
		tabs = append(tabs, "Conditions")
	}
	return tabs
}

// supportsLogs: the workload kinds stream logs (a workload merges its pods').
func supportsLogs(rt k8s.ResourceType) bool {
	return slices.Contains(tabsOf(rt), "Logs")
}

// resetDetail clears panel 3 for a row of kind rt, its tabs laid out for it.
func (m *Model) resetDetail(rt k8s.ResourceType) {
	d := &m.det
	prev := m.currentTab()
	d.kind, d.uid, d.loaded = rt, "", false
	d.detail, d.events, d.chain = k8s.ResourceDetail{}, nil, nil
	d.logs = nil
	d.tabs = tabsOf(rt)
	tab := 0
	want := prev
	if m.startTab != "" {
		want, m.startTab = m.startTab, ""
	}
	if i := slices.Index(d.tabs, want); i >= 0 {
		tab = i
	}
	m.detailP.Tab = tab
	for _, b := range []*textBody{d.logBody, d.eventBody, d.condBody, d.infoBody} {
		b.reset()
	}
	m.refreshRelatives()
	m.refreshHistory()
	m.showTab()
}

// showTab puts the current tab's body in the panel, its labels and actions.
func (m *Model) showTab() {
	d := &m.det
	switch m.currentTab() {
	case "Logs":
		m.detailP.Body = d.logBody
	case "Events":
		m.detailP.Body = d.eventBody
	case "Conditions":
		m.detailP.Body = d.condBody
	case "Info":
		m.detailP.Body = d.infoBody
	case "Relatives":
		m.detailP.Body = m.relList
	case "History":
		m.detailP.Body = m.histList
	}
	t := m.currentTab()
	m.detailP.Stream = t == "Logs" || t == "Events" // streaming content is not dimmed (tdp T2)
	m.setTabLabels()
	m.setDetailActions()
}

// currentTab is the name of the tab panel 3 shows.
func (m *Model) currentTab() string {
	if t := m.detailP.Tab; t >= 0 && t < len(m.det.tabs) {
		return m.det.tabs[t]
	}
	return ""
}

// setTabLabels writes the tabs on panel 3's frame: Logs and Events with
// whether they follow the newest lines (the glyph keeps its width either
// way, tdp L2), Relatives with how deep its chain is.
func (m *Model) setTabLabels() {
	d := &m.det
	names := make([]string, len(d.tabs))
	for i, t := range d.tabs {
		names[i] = t
		switch t {
		case "Logs":
			names[i] += " " + liveGlyph(d.logBody.live)
		case "Events":
			names[i] += " " + liveGlyph(d.eventBody.live)
		case "Relatives":
			// How deep the chain is, from 1: always there, so the tabs
			// after it do not move when it changes (tdp L2).
			names[i] += " " + glyphLink + strconv.Itoa(len(d.chain)+1)
		}
	}
	if len(names) == 0 {
		names = []string{"Details"}
	}
	m.detailP.Label, m.detailP.Tabs = names[0], names[1:]
}

func liveGlyph(live bool) string {
	if live {
		return glyphLive
	}
	return glyphPaused
}

// rowMoved is panel 2's cursor on another row: panel 3 empties and fetches
// the new row once the cursor has rested on it (rowDebounce).
func (m *Model) rowMoved(key string) tea.Cmd {
	if key == m.det.uid {
		return nil
	}
	m.stopLogs()
	kind := m.shownKind()
	if m.atContainers() {
		kind = ""
	}
	if kind != m.det.kind {
		m.resetDetail(kind)
	} else {
		m.det.detail, m.det.events, m.det.chain, m.det.logs = k8s.ResourceDetail{}, nil, nil, nil
		for _, b := range []*textBody{m.det.logBody, m.det.eventBody, m.det.condBody, m.det.infoBody} {
			b.reset()
		}
		m.refreshRelatives()
		m.refreshHistory()
		m.setTabLabels()
		m.setDetailActions()
	}
	m.det.uid, m.det.loaded = key, false
	m.det.nextRetry = time.Time{}
	m.detailP.Loading, m.detailP.Fail = m.client != nil && key != "", nil
	m.setListActions() // the descriptions name the row's own (an HPA's target)
	m.rowSeq++
	seq := m.rowSeq
	return tea.Tick(rowDebounce, func(time.Time) tea.Msg { return rowTickMsg{seq: seq} })
}

// rowMovedNow is rowMoved without the wait, for a drill: the user asked for
// that row.
func (m *Model) rowMovedNow() tea.Cmd {
	r, ok := m.list.Current()
	if !ok {
		m.det.uid = ""
		return nil
	}
	m.det.uid = ""
	m.rowMoved(r.ID)
	m.rowSeq++
	return m.loadRow()
}

// loadRow fetches the details of the row under panel 2's cursor and starts
// its logs.
func (m *Model) loadRow() tea.Cmd {
	m.det.loaded = true
	if c, pod, ok := m.currentContainer(); ok {
		m.setDetail(containerDetail(c, pod), nil)
		if m.logs != nil {
			m.logs.Start(pod.Name, pod.Namespace, []string{c.Name})
			m.det.logsOn = true
			return waitLogLine(m.logs)
		}
		return nil
	}
	item, ok := m.currentItem()
	if !ok {
		return nil
	}
	return tea.Batch(fetchDetail(m.client, m.shownKind(), item), m.ensureLogs(item))
}

// ensureLogs starts the row's log stream when it has logs and none runs: a
// pod's own, or the pods a workload runs now, merged.
func (m *Model) ensureLogs(item k8s.ResourceItem) tea.Cmd {
	rt := m.shownKind()
	if m.logs == nil || m.det.logsOn || !supportsLogs(rt) || m.atContainers() {
		return nil
	}
	if rt == k8s.ResourcePods {
		containers := k8s.ContainerNames(item.Raw)
		if len(containers) == 0 {
			return nil
		}
		m.logs.Start(item.Name, item.Namespace, containers)
		m.det.logsOn = true
		return waitLogLine(m.logs)
	}
	if time.Now().Before(m.det.nextRetry) {
		return nil
	}
	m.det.logsOn = true // asked: a watcher tick before the answer must not ask again
	return fetchPods(m.client, item)
}

// onPods starts a workload's merged logs.
func (m *Model) onPods(msg podsMsg) tea.Cmd {
	if msg.uid != m.det.uid {
		return nil
	}
	if msg.err != nil || len(msg.targets) == 0 {
		if msg.err != nil {
			m.log.warn("aggregate logs: " + msg.err.Error())
		} else {
			m.log.info("aggregate logs: no pods running")
		}
		m.det.logsOn = false
		m.det.nextRetry = time.Now().Add(aggregateRetry)
		return nil
	}
	m.logs.StartMulti(msg.targets)
	m.det.logsOn = true
	return waitLogLine(m.logs)
}

func (m *Model) stopLogs() {
	if m.logs != nil {
		m.logs.Stop()
	}
	m.det.logsOn = false
	m.det.logs = nil
	m.det.logBody.reset()
}

// appendLog adds a line to the Logs tab, at most maxLogLines kept.
func (m *Model) appendLog(l k8s.LogLine) {
	pod := "" // a pod's own logs, or a container's: the pod goes without saying
	if m.shownKind() != k8s.ResourcePods && !m.atContainers() {
		pod = l.Pod // a workload's merged stream says which pod each line came from
	}
	m.det.logs = append(m.det.logs, logLine{pod: pod, container: l.Container, text: sanitizeLog(l.Text)})
	if n := len(m.det.logs); n > maxLogLines {
		m.det.logs = m.det.logs[n-maxLogLines:]
	}
	m.det.logBody.changed()
}

// setDetail shows a row's fetched details.
func (m *Model) setDetail(d k8s.ResourceDetail, events []k8s.EventItem) {
	m.det.detail, m.det.events = d, events
	m.detailP.Loading = false
	for _, b := range []*textBody{m.det.eventBody, m.det.condBody, m.det.infoBody} {
		b.changed()
	}
	m.refreshRelatives()
	m.refreshHistory()
	m.setDetailActions()
}

// containerDetail is what panel 3 shows of a container.
func containerDetail(c k8s.ContainerInfo, pod k8s.ResourceItem) k8s.ResourceDetail {
	name := c.Name
	if c.Init {
		name = "(init) " + name
	}
	fields := []k8s.DetailField{{Label: "Pod", Value: pod.Name}, {Label: "Image", Value: c.Image},
		{Label: "State", Value: c.State}, {Label: "Ready", Value: strconv.FormatBool(c.Ready)}}
	if c.Restarts > 0 {
		fields = append(fields, k8s.DetailField{Label: "Restarts", Value: strconv.Itoa(c.Restarts)})
	}
	if c.Ports != "" {
		fields = append(fields, k8s.DetailField{Label: "Ports", Value: c.Ports})
	}
	return k8s.ResourceDetail{Name: name, Namespace: pod.Namespace, Kind: "Container",
		YAML: k8s.MarshalContainerYAML(pod, c.Name), Fields: fields}
}

// --- the tabs' content -----------------------------------------------------

// sanitizeLog keeps what a log line shows: a carriage-return progress bar
// leaves only its last state, escape codes and other control characters go
// (they would move the cursor inside the frame), a tab becomes a space.
func sanitizeLog(s string) string {
	if strings.ContainsRune(s, '\r') {
		parts := strings.Split(s, "\r")
		s = ""
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				s = parts[i]
				break
			}
		}
	}
	s = terminu.StripStyles(s)
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r < 0x20, r >= 0x7f && r < 0xa0:
			return -1
		}
		return r
	}, s)
}

// logPalette colours a container's and a pod's name in the Logs tab, the
// same name always the same colour (the app's content, tdp P4).
var logPalette = []terminu.Color{
	terminu.Hex("#f38ba8"), terminu.Hex("#fab387"), terminu.Hex("#f9e2af"), terminu.Hex("#a6e3a1"),
	terminu.Hex("#94e2d5"), terminu.Hex("#89b4fa"), terminu.Hex("#cba6f7"), terminu.Hex("#f5c2e7"),
}

func paletteColor(name string) terminu.Color {
	h := uint32(2166136261)
	for _, b := range []byte(name) {
		h = (h ^ uint32(b)) * 16777619
	}
	return logPalette[int(h)%len(logPalette)]
}

// podTag is the end of a pod's name, its random part ("nginx-7f9c-abc12" →
// "abc12").
func podTag(name string) string {
	const want = 5
	if i := strings.LastIndex(name, "-"); i >= 0 && i < len(name)-1 {
		tail := name[i+1:]
		if len(tail) > want {
			return tail[len(tail)-want:]
		}
		return tail
	}
	if len(name) > want {
		return name[len(name)-want:]
	}
	return name
}

// renderLogs lays the log lines out: `container │ text`, or
// `container@pod │ text` in a merged stream, a long line folding under its
// text.
func (m *Model) renderLogs(w int) (header, rows []string, line []int) {
	sep := terminu.Fg(terminu.Overlay0)
	for n, l := range m.det.logs {
		ctr := terminu.Fg(paletteColor(l.container)).Render(l.container)
		plain := l.container
		if l.pod != "" {
			tag := podTag(l.pod)
			ctr += sep.Render("@") + terminu.Fg(paletteColor(l.pod)).Render(tag)
			plain += "@" + tag
		}
		prefix := ctr + sep.Render(" │ ")
		pw := terminu.Width(plain) + 3
		text := terminu.Wrap(l.text, max(w-pw, 1))
		if len(text) == 0 {
			text = []string{""}
		}
		rows, line = append(rows, prefix+text[0]), append(line, n)
		for _, t := range text[1:] {
			rows, line = append(rows, terminu.Repeat(" ", pw-2)+sep.Render("│ ")+t), append(line, n)
		}
	}
	return nil, rows, line
}

// table lays out a table whose last-but-one column (a message) folds: the
// header in Blue, the rows in red when bad says so.
func table(w int, titles []string, rows [][]string, bad func(i int) bool) (header, out []string, lineOf []int) {
	n := len(titles)
	ws := make([]int, n)
	for c, t := range titles {
		ws[c] = terminu.Width(t)
	}
	for _, r := range rows {
		for c := range n {
			if c < len(r) {
				ws[c] = max(ws[c], terminu.Width(r[c]))
			}
		}
	}
	msg := n - 2 // the column that folds
	fixed := 2 * (n - 1)
	for c := range n {
		if c != msg {
			fixed += ws[c]
		}
	}
	ws[msg] = max(min(ws[msg], w-fixed), 10)
	line := func(cells []string) []string {
		folded := terminu.Wrap(cells[msg], ws[msg])
		if len(folded) == 0 {
			folded = []string{""}
		}
		var parts []string
		for c := range n {
			v := cells[c]
			if c == msg {
				v = folded[0]
			}
			if c == n-1 { // the age: a number column, right-aligned (components/layout/panel)
				parts = append(parts, terminu.PadLeft(v, ws[c]))
				continue
			}
			parts = append(parts, terminu.Pad(v, ws[c]))
		}
		lines := []string{strings.Join(parts, "  ")}
		indent := 0
		for c := 0; c < msg; c++ {
			indent += ws[c] + 2
		}
		for _, f := range folded[1:] {
			lines = append(lines, terminu.Repeat(" ", indent)+f)
		}
		return lines
	}
	upper := make([]string, n)
	for c, t := range titles {
		upper[c] = strings.ToUpper(t)
	}
	header = []string{terminu.Fg(terminu.Blue).Render(line(upper)[0])}
	for i, r := range rows {
		cells := make([]string, n)
		copy(cells, r)
		style := terminu.Fg(terminu.Text)
		if bad != nil && bad(i) {
			style = terminu.Fg(terminu.Red)
		}
		for _, l := range line(cells) {
			out, lineOf = append(out, style.Render(l)), append(lineOf, i)
		}
	}
	return header, out, lineOf
}

func (m *Model) renderEvents(w int) (header, rows []string, line []int) {
	ev := m.det.events
	cells := make([][]string, len(ev))
	for i, e := range ev {
		cells[i] = []string{e.Type, e.Reason, e.Object, e.Message, e.Age}
	}
	if len(cells) == 0 {
		return nil, nil, nil
	}
	return table(w, []string{"Type", "Reason", "Object", "Message", "Age"}, cells,
		func(i int) bool { return ev[i].Type == "Warning" })
}

func (m *Model) renderConditions(w int) (header, rows []string, line []int) {
	cs := m.det.detail.Conditions
	if len(cs) == 0 {
		return nil, nil, nil
	}
	cells := make([][]string, len(cs))
	for i, c := range cs {
		cells[i] = []string{c.Type, c.Status, c.Reason, c.Message, c.Age}
	}
	// A condition that is False is usually the answer to "why is it not
	// ready": it is in red.
	return table(w, []string{"Type", "Status", "Reason", "Message", "Age"}, cells,
		func(i int) bool { return cs[i].Status == "False" })
}

func (m *Model) renderInfo(w int) (header, rows []string, line []int) {
	fs := m.det.detail.Fields
	lw := 0
	for _, f := range fs {
		lw = max(lw, terminu.Width(f.Label))
	}
	for n, f := range fs {
		label := terminu.Fg(terminu.Blue).Render(terminu.Pad(strings.ToUpper(f.Label), lw))
		vals := terminu.Wrap(f.Value, max(w-lw-2, 1))
		if len(vals) == 0 {
			vals = []string{""}
		}
		rows, line = append(rows, label+"  "+vals[0]), append(line, n)
		for _, v := range vals[1:] {
			rows, line = append(rows, terminu.Repeat(" ", lw+2)+v), append(line, n)
		}
	}
	return nil, rows, line
}

// --- Relatives -------------------------------------------------------------

// relatedGroup is the group of the relatives with no section of their own:
// in a grouped list every row is under a title (components/layout/panel).
const relatedGroup = "Related"

func cmpOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// relEntry is one row of the Relatives tab.
type relEntry struct {
	group, label, value string
	ref                 *k8s.RefTarget
}

// levelDetail is the detail the Relatives tab is at: the row's, or the
// chain's last.
func (m *Model) levelDetail() k8s.ResourceDetail {
	if n := len(m.det.chain); n > 0 {
		return m.det.chain[n-1].detail
	}
	return m.det.detail
}

// relEntries are the relatives of d, in their sections. Only what kbu can go
// to is listed: an image name or an emptyDir is in the YAML.
func relEntries(d k8s.ResourceDetail) []relEntry {
	var out []relEntry
	if po := d.PodRelatives; po != nil {
		if po.Owner != nil {
			out = append(out, relEntry{group: relatedGroup, label: "Owner", value: ownerText(*po.Owner), ref: po.Owner})
		}
		if po.Node != nil {
			out = append(out, relEntry{group: relatedGroup, label: "Node", value: po.Node.Name, ref: po.Node})
		}
		if po.ServiceAccount != nil {
			out = append(out, relEntry{group: relatedGroup, label: "ServiceAccount", value: po.ServiceAccount.Name, ref: po.ServiceAccount})
		}
		for _, v := range po.Volumes {
			if v.Ref != nil {
				out = append(out, relEntry{group: "Volumes", label: v.Name, value: v.Kind + "/" + v.Ref.Name, ref: v.Ref})
			}
		}
		return out
	}
	if sl := d.ServiceRelatives; sl != nil && len(sl.Pods) > 0 {
		g := fmt.Sprintf("Pods (%d)", len(sl.Pods))
		for i := range sl.Pods {
			p := &sl.Pods[i]
			out = append(out, relEntry{group: g, label: p.Name, value: "pod", ref: p})
		}
		return out
	}
	for _, sec := range d.Relatives {
		for i := range sec.Entries {
			e := sec.Entries[i]
			out = append(out, relEntry{group: cmpOr(sec.Title, relatedGroup), label: strings.TrimSpace(e.Label), value: e.Value, ref: e.Ref})
		}
	}
	return out
}

func ownerText(ref k8s.RefTarget) string {
	kind := string(ref.Type)
	if def := k8s.DefaultRegistry.Get(ref.Type); def != nil {
		kind = strings.TrimSuffix(def.DisplayName, "s")
	}
	return kind + "/" + ref.Name
}

// refreshRelatives lays the Relatives tab out for the level it is at.
func (m *Model) refreshRelatives() {
	entries := relEntries(m.levelDetail())
	rows := make([]terminu.Row, len(entries))
	for i, e := range entries {
		v := e.value
		if e.ref != nil {
			v += " " + terminu.Fg(terminu.Overlay0).Render(glyphLink)
		}
		rows[i] = terminu.Row{ID: strconv.Itoa(i), Cells: []string{e.label, v}, Group: e.group}
	}
	m.relList.SetRows(rows)
}

// relAt is the relative under the Relatives tab's cursor.
func (m *Model) relAt() (relEntry, bool) {
	r, ok := m.relList.Current()
	if !ok {
		return relEntry{}, false
	}
	i, _ := strconv.Atoi(r.ID)
	entries := relEntries(m.levelDetail())
	if i < 0 || i >= len(entries) {
		return relEntry{}, false
	}
	return entries[i], true
}

// relDrill goes into the relative under the cursor: its own relatives
// appear here, one level further down the chain. A resource already in the
// chain is not gone into again.
func (m *Model) relDrill() tea.Cmd {
	e, ok := m.relAt()
	if !ok || e.ref == nil || m.client == nil {
		return nil
	}
	ref := *e.ref
	for _, f := range m.chainRefs() {
		if f.Type == ref.Type && f.Name == ref.Name && f.Namespace == ref.Namespace {
			return m.toastWarn(fmt.Sprintf("cycle blocked: %s/%s already in chain", ref.Type, ref.Name))
		}
	}
	src := m.det.uid
	cs := m.client.Clientset()
	return func() tea.Msg {
		ctx := context.Background()
		item, err := k8s.FetchResourceByRef(ctx, cs, ref)
		if err != nil {
			return relativeMsg{ref: ref, sourceUID: src, err: err}
		}
		d := k8s.GetResourceDetail(ref.Type, item)
		d.YAML = k8s.MarshalItemYAML(item)
		k8s.EnrichRelatives(ctx, cs, ref.Type, item, &d)
		return relativeMsg{ref: ref, sourceUID: src, item: item, detail: d}
	}
}

func (m *Model) onRelative(msg relativeMsg) tea.Cmd {
	if msg.sourceUID != m.det.uid {
		return nil // the user moved on
	}
	if msg.err != nil {
		return m.toastErr(fmt.Sprintf("drill failed: %v", msg.err))
	}
	m.det.chain = append(m.det.chain, relFrame{ref: msg.ref, item: msg.item, detail: msg.detail})
	m.refreshRelatives()
	m.setTabLabels()
	m.setDetailActions()
	return nil
}

// chainRefs are the resources down the chain, the row's first.
func (m *Model) chainRefs() []k8s.RefTarget {
	refs := []k8s.RefTarget{}
	if item, ok := m.currentItem(); ok {
		refs = append(refs, k8s.RefTarget{Type: m.shownKind(), Name: item.Name, Namespace: item.Namespace})
	}
	for _, f := range m.det.chain {
		refs = append(refs, f.ref)
	}
	return refs
}

// relBack goes one level back up the chain.
func (m *Model) relBack() tea.Cmd {
	if n := len(m.det.chain); n > 0 {
		m.det.chain = m.det.chain[:n-1]
		m.refreshRelatives()
		m.setTabLabels()
		m.setDetailActions()
	}
	return nil
}

func (m *Model) detailEsc() tea.Cmd {
	if m.currentTab() == "Relatives" && len(m.det.chain) > 0 {
		return m.relBack()
	}
	return nil
}

// --- History (Helm releases) -----------------------------------------------

func (m *Model) refreshHistory() {
	revs := m.det.detail.ReleaseHistory
	current := currentRevision(revs)
	rows := make([]terminu.Row, len(revs))
	for i, r := range revs {
		mark := ""
		if r.Revision == current {
			mark = terminu.Fg(terminu.Green).Render("●")
		}
		rows[i] = terminu.Row{ID: strconv.Itoa(r.Revision), Cells: []string{mark, strconv.Itoa(r.Revision), r.Status,
			k8s.FormatHelmHistoryDate(r.Updated), r.Chart, r.Description}}
	}
	had := len(m.histList.Rows()) > 0
	m.histList.SetRows(rows)
	if !had && current > 0 {
		m.histList.Select(strconv.Itoa(current))
	}
}

// currentRevision is the deployed revision, -1 for none.
func currentRevision(revs []k8s.ReleaseRevision) int {
	cur := -1
	for _, r := range revs {
		if r.Status == "deployed" {
			cur = r.Revision
		}
	}
	return cur
}

// revisionAt is the revision under the History tab's cursor.
func (m *Model) revisionAt() (k8s.ReleaseRevision, bool) {
	r, ok := m.histList.Current()
	if !ok {
		return k8s.ReleaseRevision{}, false
	}
	n, _ := strconv.Atoi(r.ID)
	for _, rev := range m.det.detail.ReleaseHistory {
		if rev.Revision == n {
			return rev, true
		}
	}
	return k8s.ReleaseRevision{}, false
}

// --- panel 3's actions -----------------------------------------------------

func (m *Model) setDetailActions() {
	tab := m.currentTab()
	zoom := terminu.Action{Key: "z", Label: "Zoom", Desc: "full-screen this panel", Scope: terminu.PanelScope, Do: m.toggleZoom}
	// Where Enter is an item's, zoom is in the hint: an empty tab still has
	// a key for the whole panel (components/layout/panel).
	zoomHinted := zoom
	zoomHinted.Hint = "zoom"
	yaml := terminu.Action{Key: "Y", Label: "YAML", Desc: "this resource's manifest", Scope: terminu.PanelScope, Stay: true, Do: m.yamlOfLevel}
	copyAll := terminu.Action{Key: "y", Label: "Copy", Desc: "everything in this tab", Scope: terminu.PanelScope, Do: m.copyTab}
	var acts []terminu.Action
	switch tab {
	case "Logs", "Events":
		what := "line"
		if tab == "Events" {
			what = "event"
		}
		acts = []terminu.Action{
			{Key: "Enter", Label: "Zoom", Desc: "full-screen this panel", Scope: terminu.PanelScope, Hint: "zoom", Do: m.toggleZoom},
			{Label: "Go live", Desc: "jump to the newest " + what + " and follow ([G])", Scope: terminu.PanelScope,
				Do: func() tea.Cmd { m.detailP.Body.Key("G", tea.KeyMsg{}); return nil }},
			copyAll, yaml, zoom,
		}
	case "Relatives":
		onRef := func() bool { e, ok := m.relAt(); return ok && e.ref != nil }
		acts = []terminu.Action{
			{Key: "Enter", Label: "Drill in", Desc: "show its relatives here", Scope: terminu.ItemScope, Hint: "drill",
				Disabled: func() bool { return !onRef() }, Do: m.relDrill},
			// Y is the entry's YAML on an entry kbu can go to (an item
			// operation), else the YAML of the level the tab is at (the
			// panel's): one key, the object under the cursor first.
			{Key: "Y", Label: "YAML", Desc: "this entry's manifest", Scope: terminu.ItemScope,
				Hidden: func() bool { return !onRef() }, Stay: true, Do: m.yamlOfRelative},
			{Key: "y", Label: "Copy", Desc: "this row", Scope: terminu.ItemScope, Do: m.copyRelative},
			{Label: "Back", Desc: "up one level of the chain ([Esc])", Scope: terminu.PanelScope,
				Hidden: func() bool { return len(m.det.chain) == 0 }, Do: m.relBack},
			{Key: "B", Label: "Breadcrumb", Desc: "switch panels 1 and 2 to a resource up the chain", Scope: terminu.PanelScope,
				Hidden: func() bool { return len(m.det.chain) == 0 }, Do: m.openBreadcrumb},
			{Key: "Y", Label: "YAML", Desc: "the manifest of the resource this level is at", Scope: terminu.PanelScope,
				Hidden: onRef, Stay: true, Do: m.yamlOfLevel},
			zoomHinted,
		}
	case "History":
		acts = []terminu.Action{
			{Key: "Enter", Label: "Roll back to this revision", Desc: "helm rollback", Scope: terminu.ItemScope, Hint: "rollback",
				Disabled: func() bool {
					rev, ok := m.revisionAt()
					return !ok || rev.Revision == currentRevision(m.det.detail.ReleaseHistory)
				}, Ask: m.rollbackAsk(), Do: m.runRollback},
			{Key: "y", Label: "Copy", Desc: "this row", Scope: terminu.ItemScope, Do: m.copyRevision},
			yaml, zoomHinted,
		}
	default: // Conditions, Info
		acts = []terminu.Action{
			{Key: "Enter", Label: "Zoom", Desc: "full-screen this panel", Scope: terminu.PanelScope, Hint: "zoom", Do: m.toggleZoom},
			copyAll, yaml, zoom,
		}
	}
	m.detailP.Actions = acts
	// Esc is one level up only on a Relatives chain (tdp K4).
	m.detailP.OnEsc = nil
	if tab == "Relatives" && len(m.det.chain) > 0 {
		m.detailP.OnEsc = m.detailEsc
	}
}
