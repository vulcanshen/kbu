package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
)

func pressEnter(t *testing.T, m AppModel) (AppModel, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(key("enter"))
	return updated.(AppModel), cmd
}

// tdp K3 (2026-09-28 ruling): Enter on a panel 1 kind moves focus to
// panel 2.
func TestK3_Panel1EnterFocusesPanel2(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.activePanel = SidebarPanel
	m.sidebar.SnapCursorToKind(k8s.ResourcePods)
	got, _ := pressEnter(t, m)
	if got.activePanel != TablePanel {
		t.Errorf("Enter on a kind must focus panel 2, focus is on %v", got.activePanel)
	}
}

// …and a double-click on panel 1 only selects: it doesn't synthesize
// Enter and pull focus away from where the user clicked.
func TestK3_DoubleClickOnPanel1OnlySelects(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	m.activePanel = SidebarPanel
	click := leftClick(10, 5)
	updated, _ := m.Update(click)
	updated, cmd := updated.(AppModel).Update(click)
	for _, msg := range drainCmd(cmd) {
		if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyEnter {
			t.Error("a double-click on panel 1 must not synthesize Enter")
		}
	}
	if got := updated.(AppModel); got.activePanel != SidebarPanel {
		t.Error("a double-click on panel 1 must keep focus on panel 1")
	}
}

// Enter on a kind that doesn't drill opens its YAML — the same path as Y.
func TestK3_Panel2EnterOnANonDrillingKindOpensYaml(t *testing.T) {
	svc := k8s.ResourceItem{Name: "web", Namespace: "default", UID: "uid-web", Row: []string{"web"}}
	m := menuFixture(t, []k8s.ResourceItem{svc}, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourceServices
	m.detail.SetDetail(k8s.ResourceDetail{YAML: "kind: Service\n"}, nil)
	got, _ := pressEnter(t, m)
	if !got.yamlPopup.owns() {
		t.Error("Enter on a Service row must open its YAML")
	}
}

// Enter on a KubeConfig context asks before switching kbu to it; on the
// context kbu is already on it does nothing (and its menu row is dimmed).
func TestK3_Panel2EnterOnAContextConfirmsTheSwitch(t *testing.T) {
	ctxs := []k8s.ResourceItem{{Name: "prod", UID: "uid-prod", Row: []string{"prod"}}}
	m := menuFixture(t, ctxs, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourceContexts
	got, _ := pressEnter(t, m)
	if !got.confirm.owns() || got.confirm.action != ConfirmContextSwitch {
		t.Fatal("Enter on a context must open the switch confirm")
	}
	if !strings.Contains(got.confirm.message, "prod") {
		t.Errorf("the confirm must name the context, got %q", got.confirm.message)
	}
	if !strings.Contains(got.confirm.renderFullPopup(), "Enter switch") {
		t.Error("the confirm hint must read Enter switch")
	}
	for _, msg := range drainCmd(got.confirm.onConfirm) {
		if c, ok := msg.(ContextChangedMsg); !ok || c.Context != "prod" {
			t.Errorf("accepting must switch to prod, got %#v", msg)
		}
	}

	_, rows := openedMenu(t, m)
	if row := menuRow(t, rows, "enter"); row.disabled {
		t.Error("switching to another context must not be dimmed")
	}
}

// Enter on a container row shells into it — the same path as S.
func TestK3_Panel2EnterOnAContainerShells(t *testing.T) {
	pod := podItem("web", nil)
	m := menuFixture(t, []k8s.ResourceItem{pod}, 0)
	m.activePanel = TablePanel
	m.currentResource = k8s.ResourcePods
	m.drillDownPod = &pod
	m.drillDownContainers = []k8s.ContainerInfo{{Name: "app"}}
	m.table.SetRows(containerRows(m.drillDownContainers))
	got, _ := pressEnter(t, m)
	if !got.confirm.owns() || got.confirm.action != ConfirmShellExec {
		t.Error("Enter on a container must open the exec confirm")
	}
}

// Enter on a content tab (no item to act on) full-screens panel 3.
func TestK3_Panel3EnterOnAContentTabZooms(t *testing.T) {
	for _, tab := range []string{"Logs", "Events"} {
		m := menuFixture(t, nil, 0)
		m.activePanel = DetailPanel
		m.detail.SetResourceType(k8s.ResourcePods)
		m.detail.SwitchToTabByName(tab)
		got, _ := pressEnter(t, m)
		if !got.detailExpanded {
			t.Errorf("Enter on %s must full-screen panel 3", tab)
		}
	}
}

// Enter on a History revision asks before rolling back; on the deployed
// revision it does nothing.
func TestK3_Panel3EnterOnHistoryConfirmsRollback(t *testing.T) {
	m := menuFixture(t, nil, 0)
	m.activePanel = DetailPanel
	m.detail.SetResourceType(k8s.ResourceReleases)
	m.detail.SwitchToTabByName("History")
	m.detail.SetDetail(k8s.ResourceDetail{Name: "web", ReleaseHistory: []k8s.ReleaseRevision{
		{Revision: 1, Status: "superseded"},
		{Revision: 2, Status: "deployed"},
	}}, nil)
	m.detail.SwitchToTabByName("History")

	m.detail.historyCursor = 1 // deployed
	if got, _ := pressEnter(t, m); got.confirm.owns() {
		t.Error("Enter on the deployed revision must do nothing")
	}
	m.detail.historyCursor = 0
	if got, _ := pressEnter(t, m); !got.confirm.owns() || got.confirm.action != ConfirmRollback {
		t.Error("Enter on an older revision must open the rollback confirm")
	}
}
