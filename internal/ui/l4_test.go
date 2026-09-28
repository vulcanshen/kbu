package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

// screenAt is a whole app at w×h with a long EKS-style context name, a
// namespace, and the Alterm + compare chips and an error badge on the
// statusbar — the worst case for the top row.
func screenAt(t *testing.T, w, h int) AppModel {
	t.Helper()
	m := appWithSizeAndCfg(t, w, h, config.DefaultConfig())
	m.ready = true
	m.statusBar = NewStatusBarModel(m.theme, k8s.ClusterInfo{
		ContextName: "arn:aws:eks:ap-northeast-1:123456789012:cluster/production-platform-main",
	})
	m.statusBar.SetNamespace("kube-system")
	m.statusBar.SetWidth(w)
	m.items = []k8s.ResourceItem{podItem("nginx", nil), podItem("web", nil)}
	m.table.SetRows([][]string{{"default", "nginx", ""}, {"default", "web", ""}})
	m.currentResource = k8s.ResourcePods
	m.setCompareLock(m.items[0], k8s.ResourcePods)
	m.shellPty = fakeAlivePtyView(PtyKindShell, true) // hidden → [Alt-t]erm chip
	for i := 0; i < 3; i++ {
		m.appLog.Error(fmt.Sprintf("boom %d", i))
	}
	return m
}

// assertScreen checks tdp L4 (every row exactly the terminal width) and
// L3 (the chrome keeps its row count: the screen is exactly h rows).
func assertScreen(t *testing.T, name string, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Errorf("%s at %d×%d: %d rows, want %d", name, w, h, len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Errorf("%s at %d×%d: row %d is %d wide: %q", name, w, h, i, got, ansi.Strip(l))
		}
	}
}

// The statusbar fields take their names' width — no padding hole after
// a short context — and a long name is cut in the middle at the cap
// (a deviation from tdp L2, ruled 2026-09-28; see dev-remarks).
func TestStatusbar_FieldsTakeTheirNamesWidth(t *testing.T) {
	row := func(ctx, ns string) string {
		m := screenAt(t, 160, 40)
		m.statusBar.SetClusterInfo(k8s.ClusterInfo{ContextName: ctx})
		m.statusBar.SetNamespace(ns)
		return ansi.Strip(m.statusBar.ViewFull(0, 0, "", &PtyMarker{}, nil))
	}
	if got := row("orbstack", "default"); !strings.HasPrefix(got, " [C]ontext: orbstack  [N]amespace: default  [Alt-t]erm") {
		t.Errorf("short names should sit side by side with no hole: %q", got)
	}
	long := row("arn:aws:eks:ap-northeast-1:123456789012:cluster/production-platform-main", "a-very-long-namespace-name")
	if !strings.Contains(long, "[C]ontext: arn:aws:eks:") || !strings.Contains(long, "…") {
		t.Errorf("a long context should be cut in the middle: %q", long)
	}
	ctxW := ansi.StringWidth(long[strings.Index(long, "ontext: ")+len("ontext: ") : strings.Index(long, "  [N]")])
	if ctxW != 24 {
		t.Errorf("a long context takes the %d-cell cap, got %d", 24, ctxW)
	}
}

// tdp L2, L3, L4 (D6's cross-size test): at every size, with or without
// a popup on top, every row of the screen is exactly the terminal width
// and the screen is exactly the terminal height — the statusbar never
// wraps onto a second row, however long the context name.
func TestL4_EveryRowIsTheTerminalWidth(t *testing.T) {
	for _, size := range [][2]int{{80, 40}, {100, 30}, {120, 40}, {160, 48}, {200, 60}} {
		w, h := size[0], size[1]
		m := screenAt(t, w, h)
		assertScreen(t, "panels", m.View(), w, h)

		m.activePanel = TablePanel
		_ = m.openSpaceMenu()
		m.spaceMenu.animator.Finalize()
		assertScreen(t, "Space menu", m.View(), w, h)

		_ = m.openKeyRef()
		m.help.animator.Finalize()
		assertScreen(t, "key reference", m.View(), w, h)
	}
}
