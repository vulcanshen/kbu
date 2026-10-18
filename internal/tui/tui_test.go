package tui

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// Rules L4: every row is exactly as wide as the terminal, at every size, with
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
		{"Tab", "D"},            // a confirm
		{"Tab", "C", "j", "C"},  // the compare viewer
		{"!"},                   // the App Log
	}
	sizes := [][2]int{{120, 40}, {80, 40}, {59, 20}, {200, 60}, {24, 40}, {200, 7}, {16, 6}}
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

// Namespace tells same-named objects apart: while Name can give up the room
// panel 2 lacks and keep its MinWidth, Name gives it and Namespace stays
// whole (the old table kept Namespace at 15 cells or more).
func TestNamespaceKeepsItsWidth(t *testing.T) {
	for _, w := range []int{120, 100} {
		m := newTestModel(t)
		m.size(w, 30)
		m.feed(pod("kube-system", "prometheus-kube-prometheus-operator-6644ffcf4d-x2b9q"), pod("demo", "web-1"))
		if s := m.screen_(); !strings.Contains(s, "kube-system") {
			t.Errorf("at %d columns Namespace is cut:\n%s", w, s)
		}
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
	saved, err := config.Load()
	if err != nil || !saved.IsPinned("pod") {
		t.Errorf("pin not saved: %v %v", err, saved.PinnedOrdered())
	}
	m.press("P")
	if m.kinds.Rows()[0].Group == pinnedGroup {
		t.Errorf("Unpin left the Pinned group: %+v", m.kinds.Rows()[0])
	}
}

// D on a pinned kind is the Drag mode (Rules K11): j moves it, past the end
// it wraps to the top (Rules K12), Enter keeps the order; Esc puts it back.
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
	m.press("j") // Deployments is last: it wraps to the top
	if got := []k8s.ResourceType{k8s.ResourceDeployments, k8s.ResourcePods}; !slices.Equal(m.pinned, got) {
		t.Errorf("after j: %v, want %v", m.pinned, got)
	}
	m.press("Esc")
	if !slices.Equal(m.pinned, want) {
		t.Errorf("Esc kept %v, want back to %v", m.pinned, want)
	}
	m.press("D", "k", "Enter")
	saved, _ := config.Load()
	if got := saved.PinnedOrdered(); !slices.Equal(got, []string{"deployment", "pod"}) {
		t.Errorf("saved order %v, want deployment, pod", got)
	}
}

// C marks the compare anchor, C on another row compares (a viewer opens),
// Esc on the panel drops the anchor first (Rules K4).
func TestCompareAnchor(t *testing.T) {
	m := newTestModel(t)
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "C")
	if m.anchor == nil || m.anchor.item.Name != "a" {
		t.Fatalf("anchor %+v, want on a", m.anchor)
	}
	if !strings.Contains(m.screen_(), "[C]ompare") {
		t.Errorf("the statusbar has no [C]ompare:\n%s", m.screen_())
	}
	m.press("j", "C")
	if !strings.Contains(m.screen_(), "COMPARE") {
		t.Fatalf("C on another row opened no compare:\n%s", m.screen_())
	}
	m.press("Esc", "Esc")
	if m.anchor != nil {
		t.Errorf("Esc kept the anchor")
	}
}

// D asks first; Enter runs kubectl delete with kbu's own context, never a
// real one in a test.
func TestDeleteAsksThenRunsKubectl(t *testing.T) {
	m := newTestModel(t)
	var ran []string
	m.run = func(name string, args ...string) (string, error) {
		ran = append([]string{name}, args...)
		return "pod \"a\" deleted", nil
	}
	m.feed(pod("demo", "a"), pod("demo", "b"))
	m.press("Tab", "D")
	if ran != nil {
		t.Fatal("D deleted without asking")
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
	m.press("Tab", "D", "Esc")
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

// Logs and Events are streaming content: not dimmed when the panel loses the
// focus (Rules T2).
func TestStreamingTabsAreNotDimmed(t *testing.T) {
	m := newTestModel(t)
	for i, tab := range m.det.tabs {
		m.detailP.Tab = i
		m.showTab()
		want := tab == "Logs" || tab == "Events"
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
