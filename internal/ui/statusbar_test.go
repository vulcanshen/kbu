package ui

import (
	"strings"
	"testing"

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

func TestStatusBarModel_View_ContainsContextAndCluster(t *testing.T) {
	m := newTestStatusBar()
	view := m.View()

	if !strings.Contains(view, "test-ctx") {
		t.Error("status bar must show context name")
	}
	if !strings.Contains(view, "test-cluster") {
		t.Error("status bar must show cluster name")
	}
}

func TestStatusBarModel_View_ContainsNamespace(t *testing.T) {
	m := newTestStatusBar()
	m.SetNamespace("staging")
	view := m.View()

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

// ── PtyMarker ──────────────────────────────────────────────────────────────

func TestStatusBarModel_ViewFull_NoPtyMarker(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(0, "", nil)
	if strings.Contains(v, "attached") || strings.Contains(v, "KM8erm") {
		t.Error("no PTY marker requested — bar must not render one")
	}
}

func TestStatusBarModel_ViewFull_AttachedMarker(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(0, "", &PtyMarker{Visible: true, Label: " attached"})
	if !strings.Contains(v, "attached") {
		t.Error("visible PTY marker must surface 'attached' label")
	}
}

func TestStatusBarModel_ViewFull_HiddenMarker(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(0, "", &PtyMarker{Visible: false, Label: " KM8erm"})
	if !strings.Contains(v, "KM8erm") {
		t.Error("hidden PTY marker must surface 'KM8erm' label")
	}
}

func TestStatusBarModel_ViewFull_MarkerCoexistsWithErrorBadge(t *testing.T) {
	m := newTestStatusBar()
	v := m.ViewFull(3, "", &PtyMarker{Visible: false, Label: " KM8erm"})
	if !strings.Contains(v, "KM8erm") {
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
