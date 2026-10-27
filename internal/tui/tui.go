// Package tui is kbu's terminal UI, built on terminu-go: the lib
// draws the screen and drives the core keys — the Space menu, the key
// reference, the hints, confirms, dimming, Esc one layer at a time — and
// this package registers kbu's three panels, their actions and popups, and
// feeds them the cluster's data.
package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// Options is what kbu starts with.
type Options struct {
	Client *k8s.Client // nil in tests: nothing is fetched
	Config *config.Config
	State  *config.State
	// Kinds are the pins and sorts, kbu's data (tdp E3), from kinds.yaml.
	Kinds config.KindData
	// ConfigErr, StateErr and KindsErr are why the settings, the state or
	// the pins and sorts could not be read: kbu starts on the defaults and
	// asks to move the file aside (tdp E3).
	ConfigErr, StateErr, KindsErr error
	// Notices go into the App Log at start (a migrated config directory, a
	// deprecated key).
	Notices []string
	// Moved is where the settings, the logs and the pins and sorts were
	// moved at start (tdp E3): a toast
	// says so, or an error popup when the move failed.
	Moved []config.Moved
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
	current      k8s.ResourceType // the kind panel 1 shows in panel 2, watched
	rootItems    []k8s.ResourceItem
	drill        []drillLevel // panel 2's drill chain, below current
	compareSplit bool         // the compare shows SPLIT, not UNIFIED: kept with the state
	pendingSel   *k8s.RefTarget
	kindSeq      int // invalidates a kind switch still waiting out its debounce
	rowSeq       int // the same for a row switch

	// panel 3
	det detailState

	// the PTYs
	alterm *terminu.Terminal
	tx     *terminu.Terminal // kubectl edit or exec

	startTab   string // the state's panel 3 tab, applied once the kind has its tabs
	startPanel string
	moved      []config.Moved

	// fillNamespaces puts the namespaces in the open namespace picker.
	fillNamespaces func([]string)

	// kindData are the pins and sorts: kinds.yaml's, or config.yaml's while
	// kbu could not move them from there (kindsInConfig; tdp E3).
	kindData      *config.KindData
	kindsInConfig bool

	// cfgErr, stateErr and kindsErr are the files kbu could not read;
	// asideAsked is the start asking to move one aside (tdp E3).
	cfgErr, stateErr, kindsErr error
	asideAsked                 bool
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
		cfgErr: o.ConfigErr, stateErr: o.StateErr, kindsErr: o.KindsErr,
	}
	if m.cfg == nil {
		m.cfg = config.DefaultConfig()
	}
	if o.State != nil {
		m.compareSplit = o.State.CompareSplit
	}
	m.kindData = &o.Kinds
	if m.kindsInConfig = config.KindDataInConfig(m.cfg); m.kindsInConfig {
		m.kindData = &m.cfg.ResourceKindConfig
	}
	if m.state == nil {
		m.state = config.DefaultState()
	}
	if m.client != nil {
		m.watcher = k8s.NewWatcher(m.client.Clientset())
		m.logs = k8s.NewLogStreamer(m.client.Clientset())
	}
	// No mouse: the terminu family is keyboard only (tdp X1; KB4, settled),
	// and so the Settings popup that held its two settings is gone too.
	a := terminu.New("kbu")
	m.log = appLog{app: a}
	a.QuitDesc = "leave kbu"
	a.ShiftTab = true
	// kbu's least size is lower than the family's 60×32 (tdp L1): it runs in
	// a terminal's usual 80×24 and in a split pane, its narrow layout (one
	// panel) working down to minWidth×minHeight. docs/dev-remarks.md says why.
	a.MinWidth, a.MinHeight = minWidth, minHeight
	a.InputTabWidth = m.cfg.InputTabWidth // the family's input_tab_width (tdp E3)
	a.Status = m.statusbar
	a.OnMsg = m.onMsg
	a.Start = m.start
	m.app = a

	m.buildKinds()
	m.buildList()
	m.buildDetail()
	// Each panel shows the details of the one before it
	// (tdp panel.13): panel 2 the kind under panel 1's cursor,
	// panel 3 the row under panel 2's.
	m.kindsP.Details, m.listP.Details = m.listP, m.detailP
	m.screen = &terminu.Screen{Name: "kbu", Panels: []*terminu.Panel{m.kindsP, m.listP, m.detailP},
		Layout: layout, NarrowBelow: 60}
	a.AddScreen(m.screen)
	a.Global = m.globalActions()
	if s, err := newSplash(o.Version); err == nil {
		a.Splash = s
	}

	m.log.info("Started kbu")
	for _, n := range o.Notices {
		for _, line := range strings.Split(n, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				m.log.info(line)
			}
		}
	}
	for _, w := range m.cfg.DeprecationWarnings {
		m.log.warn(w)
	}
	// What an older kbu kept that this one no longer reads: an INFO for
	// each still there, at every start, and nothing else (tdp E3-23).
	a.NoLongerUsed(config.NoLongerUsed()...)
	m.restore()
	return m
}

// Run runs kbu until the user leaves, then stops what it started and writes
// down where the user was (the session state).
func (m *Model) Run() error {
	err := m.app.Run()
	m.shutdown()
	if err != nil && m.asideAsked && !errors.Is(err, tea.ErrProgramPanic) {
		return NotRead{err}
	}
	return err
}

// NotRead is Run's error when the user left at a file kbu could not read,
// not moving it aside (tdp E3): main writes it to stderr, and it is no crash.
type NotRead struct{ Err error }

func (e NotRead) Error() string { return e.Err.Error() }
func (e NotRead) Unwrap() error { return e.Err }

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

// minWidth and minHeight are kbu's least size (tdp L1): below it only its
// name and the size it needs are drawn.
const minWidth, minHeight = 40, 15

// layout puts panel 1 on the left, 24 cells wide, and panels 2 and 3 on the
// right, panel 3 under panel 2 (tdp leaves the arrangement to the app,
// tdp L1).
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

// globalActions are what acts on kbu as a whole (tdp P3): every one is
// a row of the global operation popup and a key on the panels (tdp M4).
func (m *Model) globalActions() []terminu.Action {
	return []terminu.Action{
		{Key: "N", Label: "Namespace", Desc: "pick which namespaces to show", Scope: terminu.GlobalScope, Do: m.openNamespaces},
		{Key: "C", Label: "Context", Desc: "switch the kubeconfig context", Scope: terminu.GlobalScope, Do: m.openContexts},
		{Key: "Alt-t", Label: "Term", Desc: "show the embedded shell (Alterm)", Scope: terminu.GlobalScope, Clear: true, Do: m.showAlterm},
	}
}

// start is what runs when the program starts: who we are connected to, the
// namespaces checked against the cluster, the CRDs, the first kind, and the
// panel the user left from.
func (m *Model) start() tea.Cmd {
	cmds := []tea.Cmd{m.selectKind(m.kindAt, false)}
	if m.client != nil {
		info := m.client.GetClusterInfo()
		m.log.info(fmt.Sprintf("Connected to %s (%s)", info.ContextName, info.ServerURL))
		if !k8s.HelmAvailable() {
			m.log.info("Helm CLI not found — Helm Releases category hidden")
		}
		cmds = append(cmds, fetchNamespaces(m.client, true), discoverCRDs(m.client))
	}
	cmds = append(cmds, m.sayMoved(), m.askAside())
	// Back on the panel the user left from: starting kbu is what they did,
	// and where it starts is what they expect (tdp K3).
	if p := map[string]*terminu.Panel{"table": m.listP, "detail": m.detailP}[m.startPanel]; p != nil {
		cmds = append(cmds, m.app.Focus(p))
	}
	return tea.Batch(cmds...)
}

// askAside asks about the files of kbu's own it could not read (tdp E3), one
// after the other: moved aside, kbu goes on as on its first start, the
// settings on their defaults, the state and the pins and sorts empty, and
// none of them written until it is saved, as the first start writes none
// (tdp E3-18); or the user leaves. kbu runs on the defaults meanwhile.
func (m *Model) askAside() tea.Cmd {
	files := []struct {
		path string
		err  error
	}{
		{config.ConfigPath(), m.cfgErr},
		{config.StatePath(), m.stateErr},
		{config.KindDataPath(), m.kindsErr},
	}
	var ask func(i int) tea.Cmd
	ask = func(i int) tea.Cmd {
		for i < len(files) && files[i].err == nil {
			i++
		}
		if i == len(files) {
			return nil
		}
		f := files[i]
		m.asideAsked = true
		return m.app.MoveAside(f.path, f.err, func() tea.Cmd { return ask(i + 1) })
	}
	return ask(0)
}

// sayMoved says what was moved at start (tdp E3): an Info toast from where
// to where, and an error popup with both paths for what stayed where it was.
func (m *Model) sayMoved() tea.Cmd {
	var done, left []string
	said := map[string]bool{}
	for _, mv := range m.moved {
		switch {
		case mv.Err == nil:
			from, to := mv.From, mv.To
			if mv.What == "settings" {
				from, to = filepath.Dir(from), filepath.Dir(to) // the files of one directory
			}
			if s := fmt.Sprintf("%s from %s to %s", mv.What, from, to); !said[s] {
				said[s] = true
				done = append(done, s)
			}
		case mv.What == "pins and sorts" && errors.Is(mv.Err, config.ErrThereAlready):
			left = append(left, fmt.Sprintf("The pins and sorts in %s stay where they are: %s is there already. Merge the two yourself.", mv.From, mv.To))
		case mv.What == "pins and sorts":
			left = append(left, fmt.Sprintf("The pins and sorts in %s could not be moved to %s (%v): kbu keeps using them there. Move them yourself.", mv.From, mv.To, mv.Err))
		case errors.Is(mv.Err, config.ErrThereAlready):
			left = append(left, fmt.Sprintf("%s stays where it is: %s is there already. Merge the two yourself.", mv.From, mv.To))
		default:
			keeps := ""
			if mv.What == "settings" {
				keeps = ": kbu keeps using it"
			}
			left = append(left, fmt.Sprintf("%s could not be moved to %s (%v)%s. Move it yourself.", mv.From, mv.To, mv.Err, keeps))
		}
	}
	var cmds []tea.Cmd
	if len(done) > 0 {
		cmds = append(cmds, m.app.Toast(terminu.Info, "Moved kbu's "+strings.Join(done, ", and its ")))
	}
	if len(left) > 0 {
		cmds = append(cmds, m.app.Error("Not moved", left...))
	}
	return tea.Batch(cmds...)
}

// statusbar is the row at the top (tdp screen.4): the context
// and the namespaces with their hotkeys, the Alterm in the background, the
// compare anchor, and on the right the errors and warnings not seen yet.
// When panel 2 has the focus its C (Compare) is the C that fires: [C]ompare
// is lit there and [C]ontext is not (tdp M9).
func (m *Model) statusbar(int) []terminu.Span {
	grey, value := terminu.Fg(terminu.Overlay0), terminu.Fg(terminu.Lavender)
	lit, unlit := terminu.Fg(terminu.Blue), terminu.Fg(terminu.Surface2)
	onList := m.screen.Focused() == m.listP
	// panel 2's C is compare where it has a row to compare: there [C]ontext
	// does not fire (tdp M9).
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
	// field of a row pushes nothing. The lib fits the row to the terminal,
	// and draws a control character in a name by its name.
	out := []terminu.Span{{Text: " "}}
	add := func(spans ...terminu.Span) {
		if len(out) > 1 {
			out = append(out, terminu.Span{Text: "  "})
		}
		out = append(out, spans...)
	}
	if m.alterm != nil && m.alterm.Running() {
		// A live session is in effect (tdp color.2: Green).
		add(terminu.Span{Text: "[Alt-t]", Style: lit}, terminu.Span{Text: "erm", Style: terminu.Fg(terminu.Green)})
	}
	if m.list.Anchor != "" { // in the Compare mode
		cmpKey := unlit
		if compareC {
			cmpKey = lit
		}
		add(terminu.Span{Text: "[C]", Style: cmpKey}, terminu.Span{Text: "ompare", Style: grey})
	}
	add(terminu.Span{Text: "[C]", Style: ctxKey}, terminu.Span{Text: "ontext: ", Style: grey},
		terminu.Span{Text: terminu.Pad(capMiddle(ctx, 24), 24), Style: value})
	add(terminu.Span{Text: "[N]", Style: lit}, terminu.Span{Text: "amespace: ", Style: grey},
		terminu.Span{Text: capMiddle(ns, 16), Style: value})
	return out
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

// toastErr says a failure at once (tdp F5); the lib keeps it in the App Log.
func (m *Model) toastErr(text string) tea.Cmd { return m.toast(terminu.Error, text) }

// toastWarn is toastErr for something done, or still usable, that is worth
// a look (tdp toast.2).
func (m *Model) toastWarn(text string) tea.Cmd { return m.toast(terminu.Warning, text) }

// toastDone says an action the user asked for is done: a Success toast, in
// Green (tdp toast.2).
func (m *Model) toastDone(text string) tea.Cmd { return m.toast(terminu.Success, text) }

// toast is a toast of a text from outside too (kubectl's output, the API's
// error): a toast is a short text, a line break in it drawn by its name
// (tdp components.5), so its first line goes in the toast
// and the whole of it in the App Log.
func (m *Model) toast(kind terminu.ToastKind, text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if first, rest, ok := strings.Cut(text, "\n"); ok && strings.TrimSpace(rest) != "" {
		m.log.app.Log(kind, text)
		text = strings.TrimSpace(first)
	}
	return m.app.Toast(kind, text)
}

// errorPopup is an error popup of a text from outside (the API's error), as
// toast is a toast of one: its first line in the popup, the whole of it in
// the App Log (tdp components.5). There the title goes first: kbu's words
// start the entry with a capital, and the text is kept as it is, often
// starting with a name, x509: or pods "a" (tdp toast.2).
func (m *Model) errorPopup(title, text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if first, rest, ok := strings.Cut(text, "\n"); ok && strings.TrimSpace(rest) != "" {
		m.log.app.Log(terminu.Error, title+": "+text)
		text = strings.TrimSpace(first)
	}
	return m.app.Error(title, text)
}
