// Package tui is kbu's terminal UI, built on terminu-go (tdp v0.7.4): the lib
// draws the screen and drives the core keys — the Space menu, the key
// reference, the hints, confirms, dimming, Esc one layer at a time — and
// this package registers kbu's three panels, their actions and popups, and
// feeds them the cluster's data.
package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// Options is what kbu starts with.
type Options struct {
	Client   *k8s.Client // nil in tests: nothing is fetched
	Config   *config.Config
	State    *config.State
	StateErr error
	// Notices go into the App Log at start (a migrated config directory, a
	// deprecated key).
	Notices []string
	// Moved is where the settings were moved at start (tdp E3): a toast
	// says so, or an error popup when the move failed.
	Moved config.Move
	// Status are the colours of a resource's status cells (the app's
	// content, tdp P4).
	Status theme.StatusColors
	// Version is what `kbu version` prints, for the splash.
	Version string
}

// Model is the whole of kbu's UI: the lib's app, the three panels, and the
// state behind them.
type Model struct {
	app    *terminu.App
	client *k8s.Client
	cfg    *config.Config
	state  *config.State
	status theme.StatusColors
	log    appLog

	watcher *k8s.Watcher
	logs    *k8s.LogStreamer
	// run runs a command to its end and returns what it printed: kubectl
	// delete, helm rollback. Tests replace it, so that no test ever deletes
	// anything.
	run func(name string, args ...string) (string, error)

	screen   *terminu.Screen
	kindsP   *terminu.Panel
	kinds    *terminu.List
	listP    *terminu.Panel
	list     *terminu.List
	detailP  *terminu.Panel
	relList  *terminu.List
	histList *terminu.List

	// panel 1
	pinned []k8s.ResourceType
	kindAt k8s.ResourceType // the kind under panel 1's cursor
	drag   *dragState

	// panel 2
	current    k8s.ResourceType // the kind panel 1 shows in panel 2, watched
	rootItems  []k8s.ResourceItem
	drill      []drillLevel // panel 2's drill chain, below current
	anchor     *anchorRef   // the compare anchor
	pendingSel *k8s.RefTarget
	kindSeq    int // invalidates a kind switch still waiting out its debounce
	rowSeq     int // the same for a row switch

	// panel 3
	det detailState

	// the PTYs
	alterm *terminu.Terminal
	tx     *terminu.Terminal // kubectl edit or exec

	startTab   string // the state's panel 3 tab, applied once the kind has its tabs
	startPanel string
	moved      config.Move
}

// New builds kbu's UI.
func New(o Options) *Model {
	m := &Model{
		client: o.Client,
		cfg:    o.Config,
		state:  o.State,
		status: o.Status,
		run:    runCommand,
		moved:  o.Moved,
	}
	if m.cfg == nil {
		m.cfg = config.DefaultConfig()
	}
	if m.state == nil {
		m.state = config.DefaultState()
	}
	if m.client != nil {
		m.watcher = k8s.NewWatcher(m.client.Clientset())
		m.logs = k8s.NewLogStreamer(m.client.Clientset())
	}
	// TODO(lib KB4): the mouse (click, wheel, right-click for Space) and the
	// Settings popup that held its two settings.
	a := terminu.New("kbu")
	a.QuitDesc = "leave kbu"
	a.ShiftTab = true
	a.InputTabWidth = m.cfg.InputTabWidth // the family's input_tab_width (tdp E3)
	a.Status = m.statusbar
	a.OnMsg = m.onMsg
	a.Start = m.start
	m.app = a

	m.buildKinds()
	m.buildList()
	m.buildDetail()
	// Each panel shows the details of the one before it
	// (components/layout/panel): panel 2 the kind under panel 1's cursor,
	// panel 3 the row under panel 2's.
	m.kindsP.Details, m.listP.Details = m.listP, m.detailP
	m.screen = &terminu.Screen{Name: "kbu", Panels: []*terminu.Panel{m.kindsP, m.listP, m.detailP},
		Layout: layout, NarrowBelow: 60}
	a.AddScreen(m.screen)
	a.Global = m.globalActions()
	if s, err := newSplash(o.Version); err == nil {
		a.Splash = s
	}

	m.log.info("kbu started")
	for _, n := range o.Notices {
		for _, line := range strings.Split(n, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				m.log.info(line)
			}
		}
	}
	if o.StateErr != nil {
		m.log.warn("state: " + o.StateErr.Error() + " — starting from the defaults")
	}
	for _, w := range m.cfg.DeprecationWarnings {
		m.log.warn(w)
	}
	m.restore()
	return m
}

// Run runs kbu until the user leaves, then stops what it started and writes
// down where the user was (the session state).
func (m *Model) Run() error {
	err := m.app.Run()
	m.shutdown()
	return err
}

// shutdown stops the watcher, the log stream and the PTYs, and saves the
// session state. A save that fails does not keep the user from leaving.
func (m *Model) shutdown() {
	if m.watcher != nil {
		m.watcher.Stop()
	}
	if m.logs != nil {
		m.logs.Stop()
	}
	for _, t := range []*terminu.Terminal{m.alterm, m.tx} {
		if t != nil && t.Running() {
			t.Stop()
		}
	}
	_ = m.sessionState().Save()
}

// App is the lib's app, for tests.
func (m *Model) App() *terminu.App { return m.app }

// layout puts panel 1 on the left, 24 cells wide, and panels 2 and 3 on the
// right, panel 3 under panel 2 (tdp leaves the arrangement to the app,
// Rules L1).
func layout(w, h int) []terminu.Rect {
	left := min(26, w/3) // the kinds' names and the hint Enter:open beside the scroll position
	bottom := min(14, h/2)
	if h-bottom < 4 {
		bottom = h / 2
	}
	return []terminu.Rect{
		{X: 0, Y: 0, W: left, H: h},
		{X: left, Y: 0, W: w - left, H: h - bottom},
		{X: left, Y: h - bottom, W: w - left, H: bottom},
	}
}

// globalActions are what acts on kbu as a whole (Principle P3): every one is
// a row of the global operation popup and a key on the panels (Rules M4).
func (m *Model) globalActions() []terminu.Action {
	return []terminu.Action{
		{Key: "N", Label: "Namespace", Desc: "pick which namespaces to show", Scope: terminu.GlobalScope, Do: m.openNamespaces},
		{Key: "C", Label: "Context", Desc: "switch the kubeconfig context", Scope: terminu.GlobalScope, Do: m.openContexts},
		{Key: "Alt-t", Label: "term", Desc: "show the embedded shell (Alterm)", Scope: terminu.GlobalScope, Do: m.showAlterm},
		{Key: "!", Label: "App log", Desc: "what kbu has done and what failed", Scope: terminu.GlobalScope, Stay: true, Do: m.openAppLog},
		// Not a key of the App Log viewer: a note only shows (Rules F1).
		{Label: "Clear the App log", Desc: "forget what it has kept", Scope: terminu.GlobalScope,
			Disabled: func() bool { return len(m.log.entries) == 0 },
			Do:       func() tea.Cmd { m.log.entries = nil; m.log.seen(); return nil }},
	}
}

// start is what runs when the program starts: who we are connected to, the
// namespaces checked against the cluster, the CRDs, the first kind, and the
// panel the user left from.
func (m *Model) start() tea.Cmd {
	cmds := []tea.Cmd{m.selectKind(m.kindAt, false)}
	if m.client != nil {
		info := m.client.GetClusterInfo()
		m.log.info(fmt.Sprintf("connected to %s (%s)", info.ContextName, info.ServerURL))
		if !k8s.HelmAvailable() {
			m.log.info("helm CLI not found — Helm Releases category hidden")
		}
		cmds = append(cmds, fetchNamespaces(m.client, true), discoverCRDs(m.client))
	}
	cmds = append(cmds, m.sayMoved())
	// The lib has no call that sets the focus: the panel the user left from
	// is reached with its digit, as the user would (Rules K1).
	// TODO(lib KB5): a way to set the focused panel.
	if n := map[string]rune{"table": '2', "detail": '3'}[m.startPanel]; n != 0 {
		cmds = append(cmds, func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{n}} })
	}
	return tea.Batch(cmds...)
}

// sayMoved says where the settings were moved at start (tdp E3): an Info
// toast, or an error popup with both paths when the move failed.
func (m *Model) sayMoved() tea.Cmd {
	mv := m.moved
	if mv.From == "" {
		return nil
	}
	if mv.Err != nil {
		m.log.err(fmt.Sprintf("config: could not move %s to %s: %v", mv.From, mv.To, mv.Err))
		return m.app.Error("Settings not moved", fmt.Sprintf("kbu could not move its settings from %s to %s (%v). "+
			"It keeps using %s; move the directory to %s yourself.", mv.From, mv.To, mv.Err, mv.From, mv.To))
	}
	m.log.info(fmt.Sprintf("config: moved %s to %s", mv.From, mv.To))
	return m.app.Toast(terminu.Info, fmt.Sprintf("Moved kbu's settings from %s to %s", mv.From, mv.To))
}

// statusbar is the row at the top (components/layout/screen): the context
// and the namespaces with their hotkeys, the Alterm in the background, the
// compare anchor, and on the right the errors and warnings not seen yet.
// When panel 2 has the focus its C (Compare) is the C that fires: [C]ompare
// is lit there and [C]ontext is not (Rules M9).
func (m *Model) statusbar(w int) string {
	grey, value := terminu.Fg(terminu.Overlay0), terminu.Fg(terminu.Lavender)
	lit, unlit := terminu.Fg(terminu.Blue), terminu.Fg(terminu.Surface2)
	onList := m.screen.Focused() == m.listP
	// panel 2's C is compare where it has a row to compare: there [C]ontext
	// does not fire (Rules M9).
	compareC := onList && !m.atContainers() && m.list.HasItem()
	ctxKey := lit
	if compareC {
		ctxKey = unlit
	}
	ctx, ns := "", "All Namespaces"
	if m.client != nil {
		ctx = m.client.ContextName()
		if sel := m.client.Selection(); !sel.IsAll() {
			if sel.Count() == 1 {
				ns = sel.List()[0]
			} else {
				ns = fmt.Sprintf("%d selected", sel.Count())
			}
		}
	}
	// tdp L2: the chips that come and go first, then the context in a fixed
	// width, and the namespaces last, as wide as they are: the last dynamic
	// field of a row pushes nothing.
	var chips []string
	if m.alterm != nil && m.alterm.Running() {
		// A live session is in effect (components/color: Green).
		chips = append(chips, lit.Render("[Alt-t]")+terminu.Fg(terminu.Green).Render("erm"))
	}
	if m.anchor != nil {
		cmpKey := unlit
		if compareC {
			cmpKey = lit
		}
		chips = append(chips, cmpKey.Render("[C]")+grey.Render("ompare"))
	}
	chips = append(chips, ctxKey.Render("[C]")+grey.Render("ontext: ")+value.Render(terminu.Pad(capMiddle(ctx, 24), 24)),
		lit.Render("[N]")+grey.Render("amespace: ")+value.Render(capMiddle(ns, 16)))
	left := " " + strings.Join(chips, "  ")
	right := ""
	// The count keeps its width as it grows (Rules L2).
	switch {
	case m.log.unseenErrors > 0:
		right = terminu.Fg(terminu.Red).Render(countText(m.log.unseenErrors, "error")) + grey.Render(" in the App Log [!] ")
	case m.log.unseenWarns > 0:
		right = terminu.Fg(terminu.Peach).Render(countText(m.log.unseenWarns, "warning")) + grey.Render(" in the App Log [!] ")
	}
	room := w - terminu.Width(right)
	if room < terminu.Width(left) {
		right, room = "", w
	}
	return terminu.Pad(terminu.Truncate(left, room), room) + right
}

// countText is a count for the statusbar, always as wide: "  3 errors  ".
func countText(n int, what string) string {
	return fmt.Sprintf("%3d %-8s", min(n, 999), plural(n, what)[len(fmt.Sprint(n))+1:])
}

func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

// capMiddle shortens s to at most w cells by cutting its middle, so both ends
// of a long context name stay readable.
func capMiddle(s string, w int) string {
	n := terminu.Width(s)
	if n <= w || w < 3 {
		return s
	}
	head := (w - 1) / 2
	tail := w - 1 - head
	// Cells, not runes (tdp E5): a wide character cut in half is a space.
	return terminu.StripStyles(terminu.Cut(s, 0, head)) + "…" + terminu.StripStyles(terminu.Cut(s, n-tail, n))
}

// runCommand runs a program to its end; its output is what it printed on
// both streams.
func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

// kubectlArgs adds kbu's own context to a kubectl command line: kbu never
// changes the kubeconfig, so every kubectl it runs is told which context.
func (m *Model) kubectlArgs(args ...string) []string {
	if m.client != nil {
		if ctx := m.client.ContextName(); ctx != "" {
			args = append(args, "--context", ctx)
		}
	}
	return args
}

// toastErr says a failure at once (Rules F5) and keeps it in the App Log.
func (m *Model) toastErr(text string) tea.Cmd {
	m.log.err(text)
	return m.app.Toast(terminu.Error, text)
}

// toastWarn is toastErr for something worth a look that is not broken.
func (m *Model) toastWarn(text string) tea.Cmd {
	m.log.warn(text)
	return m.app.Toast(terminu.Warning, text)
}
