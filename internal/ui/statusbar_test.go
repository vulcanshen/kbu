package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestStatusBar() StatusBarModel {
	t := theme.DefaultTheme()
	info := k8s.ClusterInfo{ContextName: "test-ctx", ClusterName: "test-cluster"}
	m := NewStatusBarModel(t, info)
	m.SetWidth(120)
	return m
}

// ── SetNamespace ───────────────────────────────────────────────────────────

func TestStatusBarModel_SetNamespace_Empty_ShowsAllNamespaces(t *testing.T) {
	m := newTestStatusBar()
	m.SetNamespace("")

	if m.namespace != "All Namespaces" {
		t.Errorf("empty namespace must show 'All Namespaces', got %q", m.namespace)
	}
}

func TestStatusBarModel_SetNamespace_Stores(t *testing.T) {
	m := newTestStatusBar()
	m.SetNamespace("kube-system")

	if m.namespace != "kube-system" {
		t.Errorf("expected namespace %q, got %q", "kube-system", m.namespace)
	}
}

// ── View content ───────────────────────────────────────────────────────────

func TestStatusBarModel_View_ContainsContextLabelAndName(t *testing.T) {
	m := newTestStatusBar()
	view := m.View()

	if !strings.Contains(view, "[C]ontext:") {
		t.Errorf("status bar must show bracket-hotkey context label, got %q", view)
	}
	if !strings.Contains(view, "test-ctx") {
		t.Error("status bar must show context name")
	}
}

func TestStatusBarModel_View_BracketMarkersPresentOnAllPanels(t *testing.T) {
	// Multi-segment colored rendering keeps the bracket markers literal
	// on every panel — width stays fixed, no jitter. Per-panel signal
	// is conveyed by COLOR (green active vs grey inactive) of the hotkey
	// letter, not by adding/removing the bracket frame.
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusBar()
		m.SetActivePanel(p)
		view := m.View()
		if !strings.Contains(view, "[") || !strings.Contains(view, "C") || !strings.Contains(view, "]") {
			t.Errorf("panel %v: status bar must contain `[`, `C`, `]` literals, got %q", p, view)
		}
		if !strings.Contains(view, "N") || !strings.Contains(view, "amespace:") {
			t.Errorf("panel %v: status bar must contain `N`, `amespace:` literals, got %q", p, view)
		}
		if !strings.Contains(view, "test-ctx") {
			t.Errorf("panel %v: status bar must contain context value, got %q", p, view)
		}
	}
}

func TestStatusBarModel_View_WidthInvariantAcrossPanels(t *testing.T) {
	// The user-facing invariant we care about: switching focus between
	// panels must NOT cause statusbar text to jitter horizontally. The
	// panel-aware C-letter coloring (green ↔ grey) must keep the same
	// visual width on every panel. Width-only assertion is stable
	// regardless of ANSI rendering profile (test env strips colors).
	widths := map[Panel]int{}
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusBar()
		m.SetActivePanel(p)
		widths[p] = lipgloss.Width(m.View())
	}
	want := widths[SidebarPanel]
	for p, w := range widths {
		if w != want {
			t.Errorf("status bar width must be panel-invariant (no jitter), got widths=%v", widths)
			break
		}
		_ = p
	}
}

func TestStatusBarModel_View_NamespaceHotkeyAlwaysActive(t *testing.T) {
	// `N` has no panel-specific override (no panel hijacks N for
	// something else). All three panels must render the N hotkey
	// identically — which means the namespace SEGMENT bytes are
	// identical across panels.
	views := make(map[Panel]string)
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m := newTestStatusBar()
		m.SetActivePanel(p)
		views[p] = m.View()
	}
	// Same namespace token must appear in all views — proxy for "same
	// rendering" since N is global.
	for p, v := range views {
		if !strings.Contains(v, "amespace:") || !strings.Contains(v, "All Namespaces") {
			t.Errorf("panel %v: namespace segment must render with N letter + value, got %q", p, v)
		}
	}
}

func TestStatusBarModel_View_DoesNotShowCluster(t *testing.T) {
	// v1.7: cluster slot dropped (context binds cluster+user, surfacing
	// both was duplicating the same signal).
	m := newTestStatusBar()
	view := m.View()

	if strings.Contains(view, "test-cluster") {
		t.Errorf("v1.7 status bar must NOT surface cluster name, got %q", view)
	}
}

func TestStatusBarModel_View_ContainsNamespaceLabelAndName(t *testing.T) {
	m := newTestStatusBar()
	m.SetNamespace("staging")
	view := m.View()

	if !strings.Contains(view, "[N]amespace:") {
		t.Errorf("status bar must show bracket-hotkey namespace label, got %q", view)
	}
	if !strings.Contains(view, "staging") {
		t.Errorf("status bar must show current namespace, got %q", view)
	}
}

// ── Badge ──────────────────────────────────────────────────────────────────

func TestStatusBarModel_ViewWithBadge_ErrorBadge(t *testing.T) {
	m := newTestStatusBar()
	view := m.ViewWithBadge(3, "")

	if !strings.Contains(view, "3") {
		t.Error("error badge must show error count")
	}
	if !strings.Contains(view, "!") {
		t.Error("error badge must contain '!' indicator")
	}
}

func TestStatusBarModel_ViewWithBadge_SuccessBadge(t *testing.T) {
	m := newTestStatusBar()
	view := m.ViewWithBadge(0, "applied")

	if !strings.Contains(view, "applied") {
		t.Error("success badge must contain the notice text")
	}
	if !strings.Contains(view, "✓") {
		t.Error("success badge must contain '✓'")
	}
}

func TestStatusBarModel_ViewWithBadge_ErrorTakesPriorityOverSuccess(t *testing.T) {
	m := newTestStatusBar()
	view := m.ViewWithBadge(2, "applied")

	if !strings.Contains(view, "!") {
		t.Error("error badge must appear when both error and success are present")
	}
	// Success notice should NOT appear when there are unread errors.
	if strings.Contains(view, "✓") {
		t.Error("success badge must not appear when errors are unread")
	}
}

func TestStatusBarModel_ViewWithBadge_NoBadge(t *testing.T) {
	m := newTestStatusBar()
	view := m.ViewWithBadge(0, "")

	if strings.Contains(view, "!") || strings.Contains(view, "✓") {
		t.Error("no badge should appear with 0 errors and no success notice")
	}
}

// TestStatusBarModel_WarnBadge_DistinctFromError pins the v1.7.5 split:
// warn fires the peach ` N warnings` badge, NOT the red `! N errors`
// badge. Error precedence is preserved when both are present.
func TestStatusBarModel_WarnBadge_DistinctFromError(t *testing.T) {
	m := newTestStatusBar()

	// warn-only — peach warnings badge, no error badge
	v := m.ViewFull(0, 2, "", nil, nil)
	if !strings.Contains(v, "") {
		t.Error("warn-only badge must contain Nerd Font U+F071 warning glyph")
	}
	if !strings.Contains(v, "2 warnings") {
		t.Errorf("warn-only badge must show count + 'warnings', got %q", v)
	}
	if strings.Contains(v, "errors") {
		t.Error("warn-only badge must NOT use 'errors' wording")
	}

	// Singular form
	v1 := m.ViewFull(0, 1, "", nil, nil)
	if !strings.Contains(v1, "1 warning ") {
		t.Errorf("singular warn badge must render 'warning' (not 'warnings'), got %q", v1)
	}

	// Error precedence — both set, error wins
	vBoth := m.ViewFull(3, 5, "", nil, nil)
	if !strings.Contains(vBoth, "3 errors") {
		t.Errorf("error must take precedence over warn, got %q", vBoth)
	}
	if strings.Contains(vBoth, "") {
		t.Error("warn badge must not coexist with error badge")
	}
}

// ── PtyMarker ──────────────────────────────────────────────────────────────

func TestStatusBarModel_ViewFull_NoPtyMarker(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(0, 0, "", nil, nil)
	if strings.Contains(v, "attached") || strings.Contains(v, "Alterm") {
		t.Error("no PTY marker requested — bar must not render one")
	}
}

func TestStatusBarModel_ViewFull_AttachedMarker(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(0, 0, "", &PtyMarker{Visible: true, Label: " attached"}, nil)
	if !strings.Contains(v, "attached") {
		t.Error("visible PTY marker must surface 'attached' label")
	}
}

func TestStatusBarModel_ViewFull_HiddenMarker(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(0, 0, "", &PtyMarker{Visible: false, Label: " Alterm"}, nil)
	if !strings.Contains(v, "Alterm") {
		t.Error("hidden PTY marker must surface 'Alterm' label")
	}
}

func TestStatusBarModel_ViewFull_MarkerCoexistsWithErrorBadge(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(3, 0, "", &PtyMarker{Visible: false, Label: " Alterm"}, nil)
	if !strings.Contains(v, "Alterm") {
		t.Error("marker must survive when an error badge is also present")
	}
	if !strings.Contains(v, "3 errors") {
		t.Error("error badge must still render alongside marker")
	}
}

// ── Height ─────────────────────────────────────────────────────────────────

func TestStatusBarModel_Height_IsOne(t *testing.T) {
	m := newTestStatusBar()
	if h := m.Height(); h != 1 {
		t.Errorf("expected height=1, got %d", h)
	}
}
