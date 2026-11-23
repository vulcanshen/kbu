package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/v4/internal/config"
	"github.com/vulcanshen/kbu/v4/internal/k8s"
)

// tdp L4: every row is exactly as wide as the terminal, at every size, with
// icons and powerline symbols one cell or two (tdp E5), each kind of popup
// kbu opens on top — menus, the key reference, a confirm, the viewers —
// including screens smaller than the popup in width, in height, in both.
func TestL4EveryRowIsTheTerminalWidth(t *testing.T) {
	popups := [][]string{
		nil,                     // the panels alone
		{"Space"},               // the Space menu
		{"Space", "G", "Enter"}, // the global operation popup over it
		{"?"},                   // the key reference
		{"Tab", "Y"},            // the YAML viewer
		{"Tab", "Alt-S"},        // the sort menu
		{"Tab", "x"},            // a confirm
		{"Tab", "D", "j", "D"},  // the diff viewer
		{"!"},                   // the App Log
	}
	// Below kbu's least size too, down to 1×1 (tdp E5): only the name and
	// the size it needs are drawn there.
	sizes := [][2]int{{120, 40}, {80, 40}, {80, 24}, {59, 20}, {40, 15}, {200, 60}, {24, 40}, {200, 7}, {16, 6},
		{80, 5}, {3, 2}, {1, 1}}
	for _, widths := range [][2]int{{1, 1}, {2, 1}, {2, 2}} {
		for _, open := range popups {
			m := newTestModel(t)
			terminu.SetIconWidth(widths[0], widths[1])
			m.feed(pod("demo", "web-1"), pod("demo", "web-2"))
			m.press(open...)
			// The popup stays open while the terminal changes size under it.
			for _, size := range sizes {
				m.size(size[0], size[1])
				frame := m.app.Frame()
				if len(frame) != size[1] {
					t.Fatalf("icon %v, %dx%d, after %v: %d rows, want %d", widths, size[0], size[1], open, len(frame), size[1])
				}
				for i, line := range frame {
					if w := terminu.Width(line); w != size[0] {
						t.Fatalf("icon %v, %dx%d, after %v: row %d is %d cells, want %d:\n%s", widths, size[0], size[1], open, i, w, size[0], line)
					}
				}
			}
		}
	}
	terminu.SetIconWidth(1, 1)
}

// Panel 2 shows the watched kind: a namespaced kind leads with Namespace,
// then Name, then the Helm mark's column.
func TestListRowsFromTheWatch(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "web-2"), pod("demo", "web-1"), pod("alpha", "db"))
	rows := m.list.Rows()
	if len(rows) != 3 {
		t.Fatalf("%d rows, want 3", len(rows))
	}
	if m.list.Columns[0].Title != "Namespace" || m.list.Columns[1].Title != "Name" || m.list.Columns[2].Title != "" {
		t.Errorf("columns start %q %q %q, want Namespace, Name, the Helm mark", m.list.Columns[0].Title, m.list.Columns[1].Title, m.list.Columns[2].Title)
	}
	// With no sort chosen: by namespace, then by name.
	if r, _ := m.list.Current(); r.Cells[1] != "db" {
		t.Errorf("first row %v, want alpha/db first", r.Cells)
	}
	if !strings.Contains(m.screen_(), "web-1") {
		t.Errorf("the rows are not drawn:\n%s", m.screen_())
	}
}

// tdp panel.5: the columns that do not fit go whole, the
// minor ones first (from the right), then Restarts, Age, Ready, Status, the
// Helm mark and Namespace; the name never goes, and it is cut in the middle.
// A sorted column goes last; with one namespace shown, Namespace goes first.
func TestListColumnsDropOrder(t *testing.T) {
	cols := columnsOf(k8s.ResourcePods)
	drops := func(sorted map[string]bool, one bool) map[string]int {
		d := map[string]int{}
		for _, c := range listColumns(cols, sorted, one) {
			d[c.Title] = c.Drop
			if name := c.Title == "Name"; c.Middle != name || c.Shrink != name || (name && c.MinWidth != nameRoom) {
				t.Errorf("%q: Middle %v, Shrink %v, MinWidth %d", c.Title, c.Middle, c.Shrink, c.MinWidth)
			}
		}
		return d
	}
	d := drops(nil, false)
	if d["Name"] != 0 {
		t.Errorf("Name drops at %d, want never", d["Name"])
	}
	order := []string{"Node", "IP", "Restarts", "Age", "Ready", "Status", "", "Namespace"}
	for i := 1; i < len(order); i++ {
		if d[order[i-1]] >= d[order[i]] {
			t.Errorf("%q drops at %d, not before %q at %d", order[i-1], d[order[i-1]], order[i], d[order[i]])
		}
	}
	// The long columns shrink first, so they go after the short ones.
	ev := map[string]int{}
	for _, c := range listColumns(columnsOf(k8s.ResourceEvents), nil, false) {
		ev[c.Title] = c.Drop
	}
	for _, short := range []string{"Age", "Type", "Reason", "Namespace"} {
		if ev[short] >= ev["Message"] || ev[short] >= ev["Object"] {
			t.Errorf("Events: %q drops at %d, not before Message (%d) and Object (%d)", short, ev[short], ev["Message"], ev["Object"])
		}
	}
	if d := drops(nil, true); d["Namespace"] != 1 {
		t.Errorf("one namespace: Namespace drops at %d, want first", d["Namespace"])
	}
	if d := drops(map[string]bool{"Restarts": true}, false); d["Restarts"] != len(cols)-1 {
		t.Errorf("sorted Restarts drops at %d, want last (%d)", d["Restarts"], len(cols)-1)
	}
}

// A long name shrinks before any column goes, cut in the middle, down to
// nameRoom (tdp panel.5): at 100 and 80 columns a list with one
// still shows its namespace, readiness and status.
func TestALongNameShrinksFirst(t *testing.T) {
	for _, c := range []struct {
		w    int
		want []string
	}{
		{100, []string{"kube-system", "READY", "STATUS", "RESTARTS"}},
		{80, []string{"kube-system", "READY", "STATUS"}},
	} {
		m := newTestModel(t)
		m.size(c.w, 30)
		m.feed(pod("kube-system", "prometheus-kube-prometheus-operator-6644ffcf4d-x2b9q"), pod("demo", "web-1"))
		m.press("Tab") // panel 2 has the focus: at 80 columns the layout is not narrow yet
		s := m.screen_()
		for _, want := range append(c.want, "x2b9q", "promethe") {
			if !strings.Contains(s, want) {
				t.Errorf("at %d columns, no %q:\n%s", c.w, want, s)
			}
		}
		if strings.Contains(s, "prometheus-kube-prometheus-operator-6644ffcf4d-x2b9q") {
			t.Errorf("at %d columns the long name is not cut:\n%s", c.w, s)
		}
	}
}

// tdp L1: below kbu's least size only its name and the size it needs are
// drawn, and the screen comes back as it was when the terminal grows.
func TestBelowTheLeastSize(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "web-1"))
	m.size(minWidth-1, minHeight)
	s := m.screen_()
	if !strings.Contains(s, "kbu") || !strings.Contains(s, fmt.Sprintf("needs a terminal of at least %d×%d", minWidth, minHeight)) {
		t.Errorf("below the least size:\n%s", s)
	}
	if m.size(minWidth, minHeight); strings.Contains(m.screen_(), "needs a terminal") {
		t.Errorf("at the least size:\n%s", m.screen_())
	}
	m.size(120, 40)
	if s := m.screen_(); !strings.Contains(s, "web-1") || strings.Contains(s, "needs a terminal") {
		t.Errorf("grown again, the screen is not back:\n%s", s)
	}
}

// tdp E3: what moved at start is told in an Info toast, from where to where;
// what stayed where it was opens an error popup with both paths.
func TestTheMovedSettingsAreTold(t *testing.T) {
	m := newTestModelWith(t, Options{Moved: []config.Moved{
		{What: "settings", From: "/old/kbu/config.yaml", To: "/new/kbu/config.yaml"},
		{What: "settings", From: "/old/kbu/state.yaml", To: "/new/kbu/state.yaml"},
		{What: "logs", From: "/old/kbu/logs", To: "/data/logs"},
	}})
	// The directory is filepath.Dir's: \old\kbu on Windows.
	want := "Moved kbu's settings from " + filepath.FromSlash("/old/kbu") + " to " + filepath.FromSlash("/new/kbu") + ", and its logs from"
	if s := m.screen_(); !strings.Contains(s, want) {
		t.Errorf("no toast for the move:\n%s", s)
	}
	m = newTestModelWith(t, Options{Moved: []config.Moved{
		{What: "settings", From: "/old/kbu", To: "/new/kbu", Err: errors.New("permission denied")},
		{What: "logs", From: "/old/logs", To: "/data/logs", Err: config.ErrThereAlready},
	}})
	s := m.screen_()
	for _, want := range []string{"NOT MOVED", "/old/kbu", "/new/kbu", "permission denied", "kbu keeps using it",
		"/old/logs stays", "/data/logs is there already", "Merge the two yourself"} {
		if !strings.Contains(s, want) {
			t.Errorf("the error popup has no %q:\n%s", want, s)
		}
	}
	// One row a sentence (tdp components.5): the second
	// starts a row of its own, right after the popup's frame.
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "/old/logs stays"); i >= 0 {
			before := line[:i]
			if j := strings.LastIndex(before, "┃"); j < 0 || strings.TrimSpace(before[j+len("┃"):]) != "" {
				t.Errorf("the second sentence does not start a row: %q", line)
			}
		}
	}
	if m = newTestModel(t); strings.Contains(m.screen_(), "Moved") {
		t.Errorf("nothing moved, yet the screen talks about it:\n%s", m.screen_())
	}
}

// The pins and sorts moved out of config.yaml are told as the settings are;
// when they stayed, the error popup says where they are and why.
func TestTheMovedPinsAndSortsAreTold(t *testing.T) {
	m := newTestModelWith(t, Options{Moved: []config.Moved{
		{What: "pins and sorts", From: "/cfg/config.yaml", To: "/data/kinds.yaml"},
	}})
	if s := m.screen_(); !strings.Contains(s, "Moved kbu's pins and sorts from /cfg/config.yaml to /data/kinds.yaml") {
		t.Errorf("no toast for the move:\n%s", s)
	}
	for _, mv := range []config.Moved{
		{What: "pins and sorts", From: "/cfg/config.yaml", To: "/data/kinds.yaml", Err: config.ErrThereAlready},
		{What: "pins and sorts", From: "/cfg/config.yaml", To: "/data/kinds.yaml", Err: errors.New("read-only")},
	} {
		m = newTestModelWith(t, Options{Moved: []config.Moved{mv}})
		s := m.screen_()
		wants := []string{"NOT MOVED", "/cfg/config.yaml", "/data/kinds.yaml"}
		if mv.Err == config.ErrThereAlready {
			wants = append(wants, "stay where they are", "Merge the two")
		} else {
			wants = append(wants, "read-only", "keeps using them")
		}
		for _, want := range wants {
			if !strings.Contains(s, want) {
				t.Errorf("the error popup has no %q:\n%s", want, s)
			}
		}
	}
}

// tdp E3: while the pins and sorts could not be moved out of config.yaml,
// kbu keeps reading and writing them there, and makes no kinds.yaml.
func TestPinsAndSortsNotMovedStayInConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ResourceKindConfig.SetPinned("deployment", 10)
	m := newTestModelWith(t, Options{Config: cfg})
	if !slices.Equal(m.pinned, []k8s.ResourceType{k8s.ResourceDeployments}) {
		t.Fatalf("pinned %v, want the config's Deployments", m.pinned)
	}
	m.kinds.Select(string(k8s.ResourcePods))
	m.size(120, 40) // a frame: OnMove takes the cursor's kind
	m.press("P")
	saved, err := config.Load()
	if err != nil || !slices.Equal(saved.ResourceKindConfig.PinnedOrdered(), []string{"deployment", "pod"}) {
		t.Errorf("config.yaml pins %v, %v", saved.ResourceKindConfig.PinnedOrdered(), err)
	}
	if _, err := os.Stat(config.KindDataPath()); err == nil {
		t.Error("a kinds.yaml was made")
	}
}

// P pins the kind under panel 1's cursor: it moves to the Pinned group at
// the top (it is not copied), and the order is saved. P again puts it back.
func TestPinMovesTheKindToPinned(t *testing.T) {
	m := newTestModel(t)
	m.press("P")
	rows := m.kinds.Rows()
	if rows[0].ID != string(k8s.ResourcePods) || rows[0].Group != pinnedGroup {
		t.Fatalf("first row %+v, want Pods in Pinned", rows[0])
	}
	n := 0
	for _, r := range rows {
		if r.ID == string(k8s.ResourcePods) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Pods is in panel 1 %d times, want once", n)
	}
	saved, err := config.LoadKindData()
	if err != nil || !saved.IsPinned("pod") {
		t.Errorf("pin not saved: %v %v", err, saved.PinnedOrdered())
	}
	m.press("P")
	if m.kinds.Rows()[0].Group == pinnedGroup {
		t.Errorf("Unpin left the Pinned group: %+v", m.kinds.Rows()[0])
	}
}

// D on a pinned kind is the Drag mode (tdp K11): j moves it, past the end
// it wraps to the top (tdp K12), Enter keeps the order; Esc puts it back.
func TestDragReordersThePinnedKinds(t *testing.T) {
	m := newTestModel(t)
	m.press("P") // Pods
	m.kinds.Select(string(k8s.ResourceDeployments))
	m.size(120, 40) // a frame: OnMove takes the cursor's kind
	m.press("P")
	want := []k8s.ResourceType{k8s.ResourcePods, k8s.ResourceDeployments}
	if !slices.Equal(m.pinned, want) {
		t.Fatalf("pinned %v, want %v", m.pinned, want)
	}
	m.press("D")
	if m.kindsP.InMode() == nil {
		t.Fatal("D did not enter the Drag mode")
	}
	m.press("Space") // the mode's Space menu (tdp K11): Drop, then the mode's own
	if s := m.screen_(); !strings.Contains(s, "Drop") || !strings.Contains(s, "DRAG MODE") {
		t.Errorf("the Drag mode's Space menu:\n%s", s)
	}
	m.press("Esc")
	m.press("j") // Deployments is last: it wraps to the top
	if got := []k8s.ResourceType{k8s.ResourceDeployments, k8s.ResourcePods}; !slices.Equal(m.pinned, got) {
		t.Errorf("after j: %v, want %v", m.pinned, got)
	}
	m.press("Esc")
	if !slices.Equal(m.pinned, want) {
		t.Errorf("Esc kept %v, want back to %v", m.pinned, want)
	}
	m.press("D", "k", "Enter")
	saved, _ := config.LoadKindData()
	if got := saved.PinnedOrdered(); !slices.Equal(got, []string{"deployment", "pod"}) {
		t.Errorf("saved order %v, want deployment, pod", got)
	}
}

// A pinned kind picked up on the filtered panel 1 moves among every pinned
// one: the filter is set aside while it is held, and comes back as it was
// once it is dropped or put back, the cursor on it (tdp panel.8, panel-199).
func TestDragSetsTheFilterAside(t *testing.T) {
	m := newTestModel(t)
	m.press("P") // Pods
	m.kinds.Select(string(k8s.ResourceDeployments))
	m.size(120, 40)
	m.press("P")
	m.press("/", "D", "e", "p", "Enter")
	if !m.kinds.Filtering() {
		t.Fatal("/ Dep did not filter panel 1")
	}
	onDeployments := func(when string) {
		t.Helper()
		if r, ok := m.kinds.Current(); !ok || r.ID != string(k8s.ResourceDeployments) {
			t.Errorf("%s: the cursor is on %+v, want Deployments", when, r)
		}
	}
	for _, c := range []struct {
		end  string
		want []k8s.ResourceType
	}{
		{"Esc", []k8s.ResourceType{k8s.ResourcePods, k8s.ResourceDeployments}},
		{"Enter", []k8s.ResourceType{k8s.ResourceDeployments, k8s.ResourcePods}},
	} {
		m.press("D")
		if m.kindsP.InMode() == nil || m.kinds.Filtering() {
			t.Fatalf("D: in a mode %v, filtering %v; want the Drag mode, the filter set aside", m.kindsP.InMode() != nil, m.kinds.Filtering())
		}
		m.press("k") // past Pods, which the filter hid
		m.press(c.end)
		if !m.kinds.Filtering() {
			t.Errorf("%s: the filter did not come back", c.end)
		}
		onDeployments(c.end)
		if !slices.Equal(m.pinned, c.want) {
			t.Errorf("%s: pinned %v, want %v", c.end, m.pinned, c.want)
		}
	}
}

// tdp L2: the statusbar's context and namespaces have a fixed width, and
// the chips that come and go are after them: nothing moves with them.
func TestStatusbarKeepsItsFields(t *testing.T) {
	m := newTestModel(t)
	bar := plainSpans(m.statusbar(120))
	if i, want := strings.Index(bar, "[N]amespace"), 1+len("[C]ontext: ")+24+2; i != want {
		t.Errorf("[N]amespace at %d, want %d:\n%q", i, want, bar)
	}
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "D")
	bar = plainSpans(m.statusbar(120))
	if i, want := strings.Index(bar, "[N]amespace"), 1+len("[C]ontext: ")+24+2; i != want {
		t.Errorf("in the Diff mode [N]amespace at %d, want %d:\n%q", i, want, bar)
	}
	if d, want := strings.Index(bar, "[D]iff"), strings.Index(bar, "[N]amespace")+len("[N]amespace: ")+16+2+len("[Alt-t]erm")+2; d != want {
		t.Errorf("[D]iff at %d, want %d, after the fields:\n%q", d, want, bar)
	}
}

// D enters the lib's Diff mode (tdp K11, panel.9), the row
// the anchor, drawn whole on Yellow: on the anchor D is off, on another row
// D or Enter opens the DIFF of their YAML and comes back to the mode; Esc
// leaves, the anchor gone. The state keeps no tab: every DIFF opens on
// UNIFIED (tdp note.8).
func TestDiffMode(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "D")
	if md := m.listP.InMode(); md == nil || md.Name != "Diff" || m.list.Anchor == "" {
		t.Fatalf("panel 2 in mode %+v, anchor %q", md, m.list.Anchor)
	}
	if !strings.Contains(m.screen_(), "[D]iff") {
		t.Errorf("no [D]iff on the statusbar:\n%s", m.screen_())
	}
	m.press("D")
	if strings.Contains(m.screen_(), "DIFF") {
		t.Errorf("D on the anchor compared:\n%s", m.screen_())
	}
	m.press("j", "Enter")
	if s := m.screen_(); !strings.Contains(s, "DIFF") || !strings.Contains(s, "demo/a ↔ demo/b") {
		t.Fatalf("Enter on another row opened no DIFF:\n%s", s)
	}
	m.press("l") // SPLIT: 120 wide
	m.press("Esc")
	if m.listP.InMode() == nil || m.list.Anchor == "" {
		t.Fatal("closing the DIFF left the mode")
	}
	m.press("Esc")
	if m.listP.InMode() != nil || m.list.Anchor != "" {
		t.Errorf("Esc kept the mode %v or the anchor %q", m.listP.InMode(), m.list.Anchor)
	}
}

// x asks first; Enter runs kubectl delete with kbu's own context, never a
// real one in a test.
func TestDeleteAsksThenRunsKubectl(t *testing.T) {
	m := newTestModel(t)
	var ran []string
	m.run = func(name string, args ...string) (string, error) {
		ran = append([]string{name}, args...)
		return "pod \"a\" deleted", nil
	}
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "x")
	if ran != nil {
		t.Fatal("x deleted without asking")
	}
	if !strings.Contains(m.screen_(), "Delete pod/a?") {
		t.Fatalf("no confirm:\n%s", m.screen_())
	}
	m.press("Enter")
	want := []string{"kubectl", "delete", "pod", "a", "-n", "demo"}
	if !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
}

// Esc on the confirm deletes nothing.
func TestDeleteEscCancels(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "x", "Esc")
	// m.run fails the test if it runs.
}

// Panel 3's tabs follow the kind.
func TestTabsOfKinds(t *testing.T) {
	cases := map[k8s.ResourceType][]string{
		k8s.ResourcePods:       {"Logs", "Relatives", "Events", "Conditions"},
		k8s.ResourceConfigMaps: {"Relatives", "Events"},
		k8s.ResourceReleases:   {"Relatives", "History"},
		k8s.ResourceContexts:   {"Info"},
		k8s.ResourceNamespaces: {"Events"},
		"":                     {"Logs"},
	}
	for rt, want := range cases {
		if got := tabsOf(rt); !slices.Equal(got, want) {
			t.Errorf("tabsOf(%q) = %v, want %v", rt, got, want)
		}
	}
}

// Logs are streaming content: not dimmed when the panel loses the focus
// (tdp T2). The events are read again with the details, and dim.
func TestStreamingTabsAreNotDimmed(t *testing.T) {
	m := newTestModel(t)
	for i, tab := range m.det.tabs {
		m.detailP.Tab = i
		m.showTab()
		want := tab == "Logs"
		if m.detailP.Stream != want {
			t.Errorf("tab %s: Stream %v, want %v", tab, m.detailP.Stream, want)
		}
	}
}

// The logs follow the newest line until the user scrolls up; G follows again.
func TestLogsFollowUntilScrolledUp(t *testing.T) {
	m := newTestModel(t)
	b := m.det.logBody
	for i := range 100 {
		m.appendLog(k8s.LogLine{Container: "app", Text: "line " + strings.Repeat("x", i%3)})
	}
	b.Draw(80, 10, true)
	if !b.live {
		t.Fatal("not following at the start")
	}
	b.Key("k", keyMsg("k"))
	if b.live {
		t.Error("scrolling up kept following")
	}
	b.Key("G", keyMsg("G"))
	if !b.live {
		t.Error("G did not follow again")
	}
}

// The splash draws docs/icon.svg; the copy go:embed reads is the same (tdp
// E6).
func TestSplashIconIsDocsIcon(t *testing.T) {
	docs, err := os.ReadFile("../../docs/icon.svg")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(icon, docs) {
		t.Error("internal/tui/icon.svg differs from docs/icon.svg")
	}
	if _, err := newSplash("v1.2.3"); err != nil {
		t.Errorf("newSplash: %v", err)
	}
}

// An age sorts by when the object was made, "N/M" by both numbers.
func TestSortValues(t *testing.T) {
	if got := sortValue("Ready", k8s.ResourceItem{}, "2/10"); got != "000000002/000000010" {
		t.Errorf("Ready 2/10 sorts by %q", got)
	}
	if got := sortValue("Age", pod("a", "b"), "1h"); got == "1h" {
		t.Error("Age sorts by what it shows, not by when it was made")
	}
}

// Only what needs a look is coloured.
func TestStatusColor(t *testing.T) {
	for status, want := range map[string]string{"Running": "", "Pending": "p", "CrashLoopBackOff": "e", "Init:0/1": "e", "Completed": ""} {
		if got := statusColor(status, "p", "e"); got != want {
			t.Errorf("statusColor(%q) = %q, want %q", status, got, want)
		}
	}
}

// The session state says where the user is.
func TestSessionState(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "j", "Tab")
	s := m.sessionState()
	if s.Kind != "pod" || s.ObjectName != "b" || s.ObjectNamespace != "demo" || s.Panel != "detail" || s.Tab != "Logs" {
		t.Errorf("state %+v", s)
	}
}

// tdp E3-23: a theme.yaml kbu no longer reads, and km8's directory, are
// noted in the App Log at every start while they are there: not deleted,
// no toast. A file that is not there is not noted.
func TestFilesNoLongerUsedAreNoted(t *testing.T) {
	testDirs(t)
	theme := filepath.Join(config.ConfigDir(), "theme.yaml")
	if err := os.WriteFile(theme, []byte("status: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	km8 := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "km8")
	for range 2 { // every start
		m := startTestModel(t, Options{})
		if s := m.screen_(); strings.Contains(s, "No longer used") {
			t.Fatalf("a toast:\n%s", s)
		}
		m.press("!")
		s := flat(m.screen_())
		if !strings.Contains(s, "No longer used: "+theme+"; delete it if you don't need it") {
			t.Errorf("theme.yaml not noted:\n%s", s)
		}
		if strings.Contains(s, "km8") {
			t.Errorf("km8's directory noted while not there:\n%s", s)
		}
	}
	if _, err := os.Stat(theme); err != nil {
		t.Errorf("theme.yaml went: %v", err)
	}
	if err := os.Mkdir(km8, 0o755); err != nil {
		t.Fatal(err)
	}
	m := startTestModel(t, Options{})
	m.press("!")
	if s := flat(m.screen_()); !strings.Contains(s, "No longer used: "+km8+"; delete it if you don't need it") {
		t.Errorf("km8's directory not noted:\n%s", s)
	}
}

// flat is a screen's words with its rows and the frames between them run
// together, for an entry the App Log folds.
func flat(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(" \n┃│", r) }), " ")
}

// tdp E3: settings, a state and pins and sorts kbu cannot read are asked
// about at the start, MOVE ASIDE: Enter moves each to .bak, the settings
// first, then the state, then the pins and sorts, and kbu goes on as on its
// first start, which writes none of them (tdp E3-18): each is made at its first
// save, the pins and sorts at a pin.
func TestUnreadableFilesMoveAside(t *testing.T) {
	m := newTestModelWith(t, Options{ConfigErr: errors.New("yaml: line 3: bad"), StateErr: errors.New("yaml: line 1: bad"),
		KindsErr: errors.New("yaml: line 2: bad")})
	files := []string{config.ConfigPath(), config.StatePath(), config.KindDataPath()}
	for _, p := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("broken: ["), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"config.yaml", "state.yaml", "kinds.yaml"} {
		if s := m.screen_(); !strings.Contains(s, "MOVE ASIDE") || !strings.Contains(s, name) {
			t.Fatalf("no MOVE ASIDE for %s:\n%s", name, s)
		}
		m.press("Enter")
	}
	for _, p := range files {
		if b, err := os.ReadFile(p + ".bak"); err != nil || string(b) != "broken: [" {
			t.Errorf("%s.bak: %q, %v", p, b, err)
		}
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a new %s before any save: %v", p, err)
		}
	}
	if !m.asideAsked {
		t.Error("Run would not say it was a file kbu could not read")
	}
	m.press("P")
	if saved, err := config.LoadKindData(); err != nil || !saved.IsPinned("pod") {
		t.Errorf("the first pin not saved: %v, %v", saved, err)
	}
}

// tdp K9-6: with a popup open the first q does not leave, it asks; Esc goes
// back to the popup.
func TestQWithAPopupAsks(t *testing.T) {
	m := newTestModel(t)
	m.press("Space", "q")
	if s := m.screen_(); !strings.Contains(s, "Quit kbu?") {
		t.Fatalf("q over the menu did not ask:\n%s", s)
	}
	m.press("Esc")
	if s := m.screen_(); strings.Contains(s, "Quit kbu?") || !strings.Contains(s, "Global operation") {
		t.Errorf("Esc did not go back to the menu:\n%s", s)
	}
}

// A toast is a short text: of kubectl's many lines only the first goes in it,
// the whole in the App Log.
func TestAToastTakesTheFirstLine(t *testing.T) {
	m := newTestModel(t)
	m.drive(m.toastErr("Delete pod/a failed: exit status 1: Error from server (Forbidden)\nmore detail"))
	if s := m.screen_(); strings.Contains(s, `\n`) || !strings.Contains(s, "Error from server (Forbidden)…") {
		t.Errorf("the toast:\n%s", s)
	}
	m.press("!")
	if s := m.screen_(); !strings.Contains(s, "more detail") {
		t.Errorf("the App Log has not the whole of it:\n%s", s)
	}
}

// An error popup of the API's error is short as a toast is: its first
// line, the whole in the App Log (tdp components.5), after kbu's title and
// as it is: x509 is a name (tdp toast.2).
func TestAnErrorPopupTakesTheFirstLine(t *testing.T) {
	m := newTestModel(t)
	m.drive(m.namespacePicker(k8s.AllNamespaces()))
	m.drive(m.onMsg(namespacesMsg{err: errors.New("x509: certificate signed by unknown authority\ntoken expired")}))
	if s := m.screen_(); !strings.Contains(s, "NAMESPACES NOT READ") || strings.Contains(s, `\n`) ||
		!strings.Contains(s, "x509: certificate") || strings.Contains(s, "token expired") {
		t.Errorf("the error popup:\n%s", s)
	}
	m.press("Esc", "Esc", "!")
	if s := m.screen_(); !strings.Contains(s, "token expired") || strings.Contains(s, "X509") {
		t.Errorf("the App Log has not the whole of it as it is:\n%s", s)
	}
}

// tdp toast.2: a toast starts with a capital, but a row starting with a name
// keeps it: what kbu says of a delete, and kubectl's own words as they are.
func TestADeleteToastStartsWithACapital(t *testing.T) {
	for out, want := range map[string]string{"": "Deleted pod/a", `pod "a" deleted`: `pod "a" deleted`} {
		m := newTestModel(t)
		m.drive(m.onCommandDone(commandDoneMsg{what: "delete", target: "pod/a\x00demo", out: out}))
		if s := m.screen_(); !strings.Contains(s, want) {
			t.Errorf("kubectl said %q: no %q:\n%s", out, want, s)
		}
	}
}

// A toast and an App Log entry start with a capital (tdp note-78), a Go
// error from the watch too.
func TestTheWatchErrorStartsWithACapital(t *testing.T) {
	if capital("") != "" || capital("é") != "É" {
		t.Errorf("capital: %q, %q", capital(""), capital("é"))
	}
	m := newTestModel(t)
	m.drive(m.onWatchErr(errors.New("listing pods: forbidden")))
	if s := m.screen_(); strings.Contains(s, "listing pods") || !strings.Contains(s, "Listing pods: forbidden") {
		t.Errorf("the toast:\n%s", s)
	}
}

// kbu's colours go as spans: a colour code left in a cell would show as its
// name, \x1b (tdp components.5). Panel 2 with a status in
// colour and the Diff anchor on Yellow, and the statusbar, show none.
func TestNoColourCodeShows(t *testing.T) {
	m := newTestModel(t)
	bad := pod("demo", "b")
	bad.Row[2] = "CrashLoopBackOff"
	m.feed(pod("demo", "a"), bad)
	m.press("Tab", "D")
	if s := m.screen_(); strings.Contains(s, `\x1b`) || !strings.Contains(s, "CrashLoopBackOff") {
		t.Errorf("a colour code shows:\n%s", s)
	}
}

// A log line is reading content (tdp components.5): colour
// codes colour, a \r leaves its last state, another sequence is its name,
// a \n starts another row.
func TestLogRowsAreReadingContent(t *testing.T) {
	if r := logRows("\x1b[32mok\x1b[0m"); len(r) != 1 || !strings.Contains(r[0], "\x1b[32m") {
		t.Errorf("the colour went: %q", r)
	}
	if r := logRows("10%\r100%"); len(r) != 1 || terminu.StripStyles(r[0]) != "100%" {
		t.Errorf("the progress: %q", r)
	}
	if r := logRows("x\x1b[2Jy"); len(r) != 1 || !strings.Contains(terminu.StripStyles(r[0]), `\x1b`) {
		t.Errorf("a clear-screen is not named: %q", r)
	}
	if r := logRows("a\nb\n"); len(r) != 2 {
		t.Errorf("a \\n is not two rows: %q", r)
	}
}

// A watch that fails is an Error toast and an entry the first time; the same
// failure again says nothing, another one says it again, and a list read
// again is one INFO entry with no toast (tdp note.7, note-105).
func TestWatchFailureSaysOnce(t *testing.T) {
	m := newTestModel(t)
	fail := func(text string) { m.drive(m.onMsg(watchErrMsg{err: errors.New(text)})) }
	fail("listing pods: forbidden")
	fail("listing pods: forbidden")
	fail("listing pods: connection refused")
	m.feed(pod("demo", "a"))
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("!")
	s := m.screen_()
	for want, n := range map[string]int{"ERR  Listing pods: forbidden": 1, "ERR  Listing pods: connection refused": 1,
		"INFO Pods can be read again": 1} {
		if got := strings.Count(s, want); got != n {
			t.Errorf("%q is in the App Log %d times, want %d:\n%s", want, got, n, s)
		}
	}
}

// The details read again with every change say a failure once, as the watch
// does (tdp note.7).
func TestDetailFailureSaysOnce(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"))
	m.det.uid = itemKey(pod("demo", "a"))
	for range 2 {
		m.drive(m.onMsg(detailMsg{uid: m.det.uid, err: errors.New("events is forbidden")}))
	}
	m.drive(m.onMsg(detailMsg{uid: m.det.uid}))
	m.press("!")
	s := m.screen_()
	for want, n := range map[string]int{"ERR  Details: events is forbidden": 1, "INFO The details can be read again": 1} {
		if got := strings.Count(s, want); got != n {
			t.Errorf("%q is in the App Log %d times, want %d:\n%s", want, got, n, s)
		}
	}
}

// Helm has no watch: the poll reads the releases again, and what changed is
// one INFO entry, nothing when nothing did, nothing the first time
// (tdp note.7, note-104).
func TestHelmChanged(t *testing.T) {
	m := newTestModel(t)
	rel := func(name, rev, status string) k8s.ResourceItem {
		return k8s.ResourceItem{Name: name, Namespace: "demo", Raw: &k8s.Release{Name: name, Namespace: "demo", Revision: rev, Status: status}}
	}
	if got := m.helmChanged([]k8s.ResourceItem{rel("web", "1", "deployed"), rel("db", "2", "deployed")}); got != "" {
		t.Errorf("the first read changed %q", got)
	}
	if got := m.helmChanged([]k8s.ResourceItem{rel("web", "1", "deployed"), rel("db", "2", "deployed")}); got != "" {
		t.Errorf("a read with no change changed %q", got)
	}
	got := m.helmChanged([]k8s.ResourceItem{rel("web", "2", "failed"), rel("api", "1", "deployed")})
	if want := "demo/api installed (revision 1, deployed); demo/db uninstalled; demo/web now revision 2, failed"; got != want {
		t.Errorf("changed %q, want %q", got, want)
	}
}

// An action of kbu that changes what a panel lists, or puts its cursor on an
// item, clears the panel filter first, whether it hides that item or not
// (tdp panel.14, panel-180): showing or hiding the Helm-managed rows.
func TestActionsClearTheFilter(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "/", "b", "Enter")
	if !m.list.Filtering() {
		t.Fatal("/ b did not filter panel 2")
	}
	m.press(".")
	if m.list.Filtering() {
		t.Error("[.] kept the filter of panel 2")
	}
}

// [P]in on a kind of the filtered panel 1 moves it to the pinned group and
// back; the filter stays, the cursor with the kind (tdp panel-198).
func TestPinKeepsTheFilter(t *testing.T) {
	m := newTestModel(t)
	m.press("1", "/", "D", "e", "p", "Enter")
	if !m.kinds.Filtering() {
		t.Fatal("/ Dep did not filter panel 1")
	}
	for _, pinned := range []bool{true, false} {
		m.press("P")
		if !m.kinds.Filtering() {
			t.Errorf("[P] (pinned %v) took the filter of panel 1 away", pinned)
		}
		if r, ok := m.kinds.Current(); !ok || r.ID != string(k8s.ResourceDeployments) || m.isPinned(k8s.ResourceDeployments) != pinned {
			t.Errorf("after [P]: the cursor on %+v, Deployments pinned %v; want it on Deployments, pinned %v",
				r, m.isPinned(k8s.ResourceDeployments), pinned)
		}
	}
}

// A row deleted from the filtered list goes when the watch says so; the
// filter stays and the cursor lands on a row it shows (tdp panel-198).
func TestADeletedRowKeepsTheFilter(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "ab1"), pod("demo", "ab2"), pod("demo", "c"))
	m.press("Tab", "/", "a", "b", "Enter")
	if r, ok := m.list.Current(); !ok || r.ID != itemKey(pod("demo", "ab1")) {
		t.Fatalf("the cursor is on %+v, want ab1", r)
	}
	m.feed(pod("demo", "ab2"), pod("demo", "c")) // the watch after deleting ab1
	if !m.list.Filtering() {
		t.Error("the filter went with the deleted row")
	}
	if r, ok := m.list.Current(); !ok || r.ID != itemKey(pod("demo", "ab2")) {
		t.Errorf("the cursor is on %+v, want ab2, the row the filter shows", r)
	}
}

// What one start has to say is a toast each, stacked in the order it
// happened; the App Log an entry each (tdp E3-37). The keys of config.yaml
// kbu does not read are an INFO each (tdp E3).
func TestTheStartsToastsStack(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Unread = map[string]any{"refesh": 5}
	m := newTestModelWith(t, Options{Config: cfg,
		Moved: []config.Moved{{What: "logs", From: "/old/logs", To: "/data/logs"}},
		Start: []terminu.Notice{{Kind: terminu.Warning, Text: `Context "gone" in state.yaml is not in the kubeconfig: using "a"`}}})
	s := m.screen_()
	for _, want := range []string{"INFO", "Moved kbu's logs from", "WARNING", `Context "gone" in state.yaml is not in the kubeconfig: using "a"`} {
		if !strings.Contains(s, want) {
			t.Errorf("the start's toasts have no %q:\n%s", want, s)
		}
	}
	if strings.Index(s, "Moved kbu's logs from") > strings.Index(s, `Context "gone"`) {
		t.Errorf("the toast that came first is not above:\n%s", s)
	}
	m.press("!")
	s = m.screen_()
	for _, want := range []string{"INFO Moved kbu's logs from", `WARN Context "gone" in state.yaml`,
		"INFO Not read: refesh in "} { // the path folds onto the rows after
		if !strings.Contains(s, want) {
			t.Errorf("the App Log has no %q:\n%s", want, s)
		}
	}
}

// A workload's merged logs look for its pods again every few seconds: none
// running is said once, a failure once, as everything kbu reads again on its
// own (tdp note.7).
func TestPodsLookedForAgainSayOnce(t *testing.T) {
	m := newTestModel(t)
	m.det.uid = "uid-demo-web"
	for range 2 {
		m.drive(m.onMsg(podsMsg{uid: m.det.uid}))
	}
	for range 2 {
		m.drive(m.onMsg(podsMsg{uid: m.det.uid, err: errors.New("pods is forbidden")}))
	}
	m.press("!")
	s := m.screen_()
	for want, n := range map[string]int{"INFO Logs: no pods running": 1, "ERR  Logs: pods is forbidden": 1} {
		if got := strings.Count(s, want); got != n {
			t.Errorf("%q is in the App Log %d times, want %d:\n%s", want, got, n, s)
		}
	}
}

// S on a pod asks first, the confirm naming the exact kubectl exec (bash when
// the image has it, else sh); Esc runs nothing. The exec itself runs in a
// terminal popup, never in a test.
func TestShellAsksFirst(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "S")
	s := m.screen_()
	for _, want := range []string{"SHELL", "Exec into app?", "kubectl exec -it a -n demo -c app", "exec bash; else exec sh"} {
		if !strings.Contains(s, want) {
			t.Errorf("the confirm has no %q:\n%s", want, s)
		}
	}
	m.press("Esc")
	if m.tx != nil || strings.Contains(m.screen_(), "Exec into") {
		t.Errorf("Esc did not leave the confirm, or started a shell:\n%s", m.screen_())
	}
}

// Copying is a Success toast saying how many lines went to the clipboard;
// nothing to copy is an error popup (tdp toast.2, F5).
func TestCopyIsTold(t *testing.T) {
	m := newTestModel(t)
	m.drive(copyText("a\nb\n"))
	if s := m.screen_(); !strings.Contains(s, "Copied 2 lines") {
		t.Errorf("no toast for the copy:\n%s", s)
	}
	m.drive(copyText(""))
	if s := m.screen_(); !strings.Contains(s, "COPY FAILED") || !strings.Contains(s, "no content to copy") {
		t.Errorf("no error popup for nothing to copy:\n%s", s)
	}
}

// A kubeconfig kbu cannot read is outside kbu (tdp E3): kbu opens, panel 2
// says why and what to do, an Error toast says it, and what needs the
// cluster is off (tdp M6).
func TestNoKubeconfigStillOpens(t *testing.T) {
	m := newTestModelWith(t, Options{ClientErr: errors.New("loading kubeconfig: open /nope: no such file or directory")})
	s := m.screen_()
	for _, want := range []string{"Loading kubeconfig: open /nope", "Fix the kubeconfig, then start kbu again"} {
		if !strings.Contains(s, want) {
			t.Errorf("no %q:\n%s", want, s)
		}
	}
	m.press("N")
	if strings.Contains(m.screen_(), "NAMESPACES") {
		t.Errorf("[N] opened the namespaces with no cluster:\n%s", m.screen_())
	}
	m.press("1", "j", "j")
	if !strings.Contains(m.screen_(), "Fix the kubeconfig") {
		t.Errorf("another kind lost the reason:\n%s", m.screen_())
	}
}

// state.yaml's context that the kubeconfig no longer has is not written
// back: the next write gives the one kbu uses, so the next start does not
// warn again (tdp E3-41).
func TestTheStateDropsAContextGone(t *testing.T) {
	m := newTestModelWith(t, Options{State: &config.State{Context: "gone"}})
	m.client = testClient(t)
	if got := m.sessionState().Context; got != "a" {
		t.Errorf("the state's context is %q, want the one in use, a", got)
	}
}

// What is in the Relatives tab's chain already is not gone into again:
// Drill in is off there (tdp M6).
func TestDrillInIsOffOnTheChain(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	if !m.inChain(k8s.RefTarget{Type: k8s.ResourcePods, Name: "a", Namespace: "demo"}) {
		t.Error("the row panel 3 shows is not in its chain")
	}
	if m.inChain(k8s.RefTarget{Type: k8s.ResourcePods, Name: "b", Namespace: "demo"}) {
		t.Error("another pod is in the chain")
	}
}

// testClient is a client on a kubeconfig of one context, a, that no test
// ever reaches: for what reads the client and not the cluster.
func testClient(t *testing.T) *k8s.Client {
	t.Helper()
	kubeconfig := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(kubeconfig, []byte("apiVersion: v1\nkind: Config\ncurrent-context: a\n"+
		"clusters:\n- name: c\n  cluster: {server: \"https://127.0.0.1:6443\"}\nusers:\n- name: u\n  user: {token: t}\n"+
		"contexts:\n- name: a\n  context: {cluster: c, user: u}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)
	c, err := k8s.NewClient("")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Recorded namespaces the cluster does not have are left out (tdp E3-41):
// kbu shows the ones it has, all of them when none is there, a Warning
// names those gone, once, and state.yaml no longer gets them.
func TestTheStateDropsNamespacesGone(t *testing.T) {
	m := newTestModel(t)
	m.client = testClient(t)
	m.client.SetSelection(k8s.SelectedNamespaces("a", "gone1", "gone2"))
	cmd := m.onMsg(namespacesMsg{check: true, names: []string{"a", "b"}})
	m.listP.Loading = false // no watch here to end the loading
	m.drive(cmd)
	if s := m.screen_(); !strings.Contains(s, `Namespaces "gone1", "gone2" in state.yaml are not in the cluster: showing [a]`) {
		t.Errorf("no Warning naming the namespaces gone:\n%s", s)
	}
	if got := m.client.Selection().List(); !slices.Equal(got, []string{"a"}) {
		t.Errorf("kbu shows %q, want the one there, a", got)
	}
	if got := m.sessionState().Namespaces; !slices.Equal(got, []string{"a"}) {
		t.Errorf("state.yaml gets %q, want the one there, a", got)
	}

	m = newTestModel(t)
	m.client = testClient(t)
	m.client.SetSelection(k8s.SelectedNamespaces("gone"))
	cmd = m.onMsg(namespacesMsg{check: true, names: []string{"a"}})
	m.listP.Loading = false
	m.drive(cmd)
	if s := m.screen_(); !strings.Contains(s, `Namespace "gone" in state.yaml is not in the cluster: showing`) {
		t.Errorf("no Warning naming the namespace gone:\n%s", s)
	}
	if !m.client.Selection().IsAll() || len(m.sessionState().Namespaces) != 0 {
		t.Errorf("none there: shows %v, state gets %q; want all, gone left out", m.client.Selection(), m.sessionState().Namespaces)
	}
}

// A recorded kind found only after start (a CRD) is not missing until
// discovery is done (tdp E3-42): kbu starts on Pods and keeps it, warns only
// when discovery ends without it, then no longer writes it (tdp E3-41), and
// stays where it is.
func TestTheStateKeepsAKindNotFoundYet(t *testing.T) {
	m := newTestModelWith(t, Options{State: &config.State{Kind: "crontabs.stable.example.com", ObjectName: "x", Tab: "Events"}})
	if m.current != k8s.ResourcePods {
		t.Fatalf("started on %q, want Pods", m.current)
	}
	if s := m.screen_(); strings.Contains(s, "crontabs") {
		t.Errorf("warned before discovery was done:\n%s", s)
	}
	if s := m.sessionState(); s.Kind != "crontabs.stable.example.com" || s.ObjectName != "x" || s.Tab != "Events" {
		t.Errorf("state.yaml gets %+v, want the recorded kind, row and tab kept", s)
	}
	m.drive(m.onMsg(crdsMsg{}))
	if s := m.screen_(); !strings.Contains(s, `Kind "crontabs.stable.example.com" in state.yaml is not in the cluster: showing Pods`) {
		t.Errorf("no Warning after discovery:\n%s", s)
	}
	if m.current != k8s.ResourcePods || m.sessionState().Kind != k8s.ResourcePods.KubectlName() {
		t.Errorf("after discovery: on %q, state gets %q; want Pods for both", m.current, m.sessionState().Kind)
	}

	// Picked before discovery is done: the user's kind, not the recorded.
	m = newTestModelWith(t, Options{State: &config.State{Kind: "crontabs.stable.example.com"}})
	m.press("j")
	if m.sessionState().Kind == "crontabs.stable.example.com" {
		t.Error("the user picked another kind, and state.yaml still gets the recorded one")
	}
}

// Details read again that fail keep what the tab had: only the toast says
// it (tdp panel.15); a tab that had nothing yet says why it is empty.
func TestDetailReadAgainKeepsWhatItHas(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"))
	m.det.uid = itemKey(pod("demo", "a"))
	ev := []k8s.EventItem{{Type: "Normal", Reason: "Pulled", Message: "pulled"}}
	m.drive(m.onMsg(detailMsg{uid: m.det.uid, events: ev}))
	m.drive(m.onMsg(detailMsg{uid: m.det.uid, err: errors.New("events is forbidden")}))
	if len(m.det.events) != 1 || m.detailP.Fail != nil {
		t.Errorf("read again and failed: events %v, Fail %v; want the events kept, no Fail", m.det.events, m.detailP.Fail)
	}
	m = newTestModel(t)
	m.feed(pod("demo", "a"))
	m.det.uid = itemKey(pod("demo", "a"))
	m.drive(m.onMsg(detailMsg{uid: m.det.uid, err: errors.New("events is forbidden")}))
	if m.detailP.Fail == nil {
		t.Error("the first read failed and the tab does not say why")
	}
}

// CRD discovery that fails leaves panel 1 its built-in kinds and says why in
// an Error toast (tdp panel.15, F5).
func TestCRDDiscoveryFailureIsTold(t *testing.T) {
	m := newTestModel(t)
	m.drive(m.onMsg(crdsMsg{err: errors.New("the server could not find the requested resource")}))
	if s := m.screen_(); !strings.Contains(s, "CRD discovery: the server could not find the requested resource") || !strings.Contains(s, "Pods") {
		t.Errorf("no Error toast, or the built-in kinds gone:\n%s", s)
	}
}

// Esc on the Relatives tab goes one level up with the cursor on the relative
// it came out of (tdp panel.10).
func TestRelativesBackPutsTheCursorWhereItCameFrom(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"))
	node := k8s.RefTarget{Type: k8s.ResourceNodes, Name: "node-1"}
	m.det.detail = k8s.ResourceDetail{PodRelatives: &k8s.PodRelativesData{
		Owner: &k8s.RefTarget{Type: k8s.ResourceDeployments, Name: "a", Namespace: "demo"}, Node: &node}}
	m.refreshRelatives()
	m.det.chain = []relFrame{{ref: node}}
	m.relBack()
	if r, ok := m.relList.Current(); !ok || r.ID != "1" {
		t.Errorf("the cursor is on %+v, want the node's row, 1", r)
	}
}

// A state that cannot be written on leaving does not keep the user from
// leaving; it is Run's error, for main to write to stderr after "kbu: "
// (tdp F5, E1).
func TestAStateNotSavedIsTold(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a folder Windows does not keep from being written to")
	}
	m := newTestModel(t)
	dir := os.Getenv("KBU__STATE")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := m.shutdown()
	if err == nil {
		t.Fatal("the state was written to a folder that cannot be written to")
	}
	if s := (NotSaved{err}).Error(); !strings.Contains(s, "state.yaml could not be written: ") {
		t.Errorf("NotSaved says %q", s)
	}
}

// A Warning that comes after the start, once the cluster has said which
// namespaces it has, is a toast of its own, stacked under the start's while
// that one still shows (tdp E3-37, toast.2).
func TestALateWarningIsAToastOfItsOwn(t *testing.T) {
	m := newTestModelWith(t, Options{Moved: []config.Moved{{What: "logs", From: "/old/logs", To: "/data/logs"}}})
	m.client = testClient(t)
	m.client.SetSelection(k8s.SelectedNamespaces("gone"))
	cmd := m.onMsg(namespacesMsg{check: true, names: []string{"a"}})
	m.listP.Loading = false // no watch here to end the loading
	m.drive(cmd)
	s := m.screen_()
	for _, want := range []string{"INFO", "Moved kbu's logs from", "WARNING", `Namespace "gone" in state.yaml is not in the cluster`} {
		if !strings.Contains(s, want) {
			t.Errorf("the toasts have no %q:\n%s", want, s)
		}
	}
}

// A kubeconfig kbu cannot read is a toast of the start, after what moved:
// stacked, so neither replaces the other unread (tdp E3-37, toast.2).
func TestNoKubeconfigIsAStartToast(t *testing.T) {
	m := newTestModelWith(t, Options{Moved: []config.Moved{{What: "logs", From: "/old/logs", To: "/data/logs"}},
		ClientErr: errors.New("loading kubeconfig: open /nope: no such file or directory")})
	s := m.screen_()
	for _, want := range []string{"INFO", "Moved kbu's logs from", "ERROR", "Loading kubeconfig: open /nope"} {
		if !strings.Contains(s, want) {
			t.Errorf("the start's toasts have no %q:\n%s", want, s)
		}
	}
}

// The Relatives tab says how deep the chain is in two cells, one after its
// glyph: one digit keeps a blank after it, the same width from 1 to 99
// (tdp L2, L2-2, components.2, panel.4).
func TestTheRelativesTabSpacesItsDepth(t *testing.T) {
	m := newTestModel(t)
	m.det.tabs = []string{"Details", "Relatives"}
	node := relFrame{ref: k8s.RefTarget{Type: k8s.ResourceNodes, Name: "node-1"}}
	for _, c := range []struct {
		deep int
		want string
	}{{0, "1 "}, {1, "2 "}, {8, "9 "}, {9, "10"}, {98, "99"}} {
		m.det.chain = slices.Repeat([]relFrame{node}, c.deep)
		m.setTabLabels()
		if want := "Relatives " + glyphLink + " " + c.want; len(m.detailP.Tabs) != 1 || m.detailP.Tabs[0] != want {
			t.Errorf("%d deep, the tabs are %q, want [%q]", c.deep, m.detailP.Tabs, want)
		}
	}
	// The tab as drawn keeps the blank: its seam comes after it.
	m = newTestModel(t)
	m.feed(pod("demo", "a"))
	seam := regexp.MustCompile(`RELATIVES ` + glyphLink + ` 1 [\x{E0B0}\x{E0BA}\x{E0BB}\x{E0BC}]`)
	if s := m.screen_(); !seam.MatchString(s) {
		t.Errorf("the Relatives tab lost the blank after its depth:\n%s", s)
	}
}

// Alterm runs $KBU__ALTERM_SHELL, alterm_shell or $SHELL, else /bin/sh; on
// Windows PowerShell, pwsh when it is there, and a $SHELL Windows cannot run
// (Git Bash's /usr/bin/bash) does not count (tdp terminal, P5).
func TestAltermShell(t *testing.T) {
	found := map[string]bool{}
	orig := lookPath
	lookPath = func(name string) (string, error) {
		if found[name] {
			return name, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = orig })
	t.Setenv("KBU__ALTERM_SHELL", "")
	t.Setenv("KBU__ALTERM_LOGIN_SHELL", "")
	for _, c := range []struct {
		goos, env, cfg, shell string
		login                 bool
		found                 []string
		want                  []string
	}{
		{"darwin", "", "", "/bin/zsh", false, nil, []string{"/bin/zsh"}},
		{"linux", "", "", "", true, nil, []string{"/bin/sh", "-l"}},
		{"linux", "fish", "bash", "/bin/zsh", false, nil, []string{"fish"}},
		{"darwin", "", "bash", "/bin/zsh", false, nil, []string{"bash"}},
		{"windows", "", "", "", false, []string{"pwsh"}, []string{"pwsh"}},
		{"windows", "", "", "", false, nil, []string{"powershell.exe"}},
		{"windows", "", "", "/usr/bin/bash", false, nil, []string{"powershell.exe"}},
		{"windows", "", "", "bash", false, []string{"bash"}, []string{"bash"}},
		{"windows", "", "cmd.exe", "", false, nil, []string{"cmd.exe"}},
	} {
		found = map[string]bool{}
		for _, f := range c.found {
			found[f] = true
		}
		t.Setenv("KBU__ALTERM_SHELL", c.env)
		t.Setenv("SHELL", c.shell)
		if got := altermCommand(c.goos, c.cfg, c.login); !slices.Equal(got, c.want) {
			t.Errorf("%s, KBU__ALTERM_SHELL %q, alterm_shell %q, $SHELL %q, on PATH %v: %q, want %q",
				c.goos, c.env, c.cfg, c.shell, c.found, got, c.want)
		}
	}
}
