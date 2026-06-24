package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
)

func appWithSizeAndCfg(t *testing.T, w, h int, cfg *config.Config) AppModel {
	t.Helper()
	th := theme.DefaultTheme()
	tbl := NewTableModel(th)
	d := NewDetailModel(th)
	d.SetResourceType(k8s.ResourcePods)
	m := AppModel{
		table:           tbl,
		detail:          d,
		sidebar:         NewSidebarModel(th),
		theme:           th,
		shellPty:        NewPtyView(),
		txPty:           NewPtyView(),
		toast:           NewToastModel(th),
		breadcrumbPopup: NewBreadcrumbPopupModel(th),
		appLog:          NewAppLogModel(th),
		listPicker:      NewListPickerModel(th),
		settingsPopup:   NewSettingsPopupModel(th),
		statusLine:      NewStatusLineModel(th),
		statusBar:       NewStatusBarModel(th, k8s.ClusterInfo{}),
		cfg:             cfg,
		width:           w,
		height:          h,
	}
	m.statusLine.SetWidth(w)
	m.statusBar.SetWidth(w)
	return m
}

func TestPanelAt_SidebarClickHitsSidebar(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	// Sidebar occupies x in [1, 25) (panelHMargin + sw=24). Click well
	// inside x=10, y=5 → sidebar.
	panel, ok := m.panelAt(10, 5)
	if !ok || panel != SidebarPanel {
		t.Errorf("panelAt(10,5) = (%v, %v), want (SidebarPanel, true)", panel, ok)
	}
}

func TestPanelAt_TableClickHitsTable(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	// Right of sidebar, top half → table. Width=120, sw=24, hMargin=1
	// → table starts at x=25. y=2 is well inside upperH (≈ 24).
	panel, ok := m.panelAt(50, 2)
	if !ok || panel != TablePanel {
		t.Errorf("panelAt(50,2) = (%v, %v), want (TablePanel, true)", panel, ok)
	}
}

func TestPanelAt_DetailClickHitsDetail(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	// y near the bottom of the right side → detail. height=40, statusBar=1,
	// statusLine=1, detailH=14. middle ends at y=38; detail top ≈ y=24.
	panel, ok := m.panelAt(50, 30)
	if !ok || panel != DetailPanel {
		t.Errorf("panelAt(50,30) = (%v, %v), want (DetailPanel, true)", panel, ok)
	}
}

func TestPanelAt_OutsideAllPanelsIsMiss(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	// y=0 is the status bar.
	if _, ok := m.panelAt(50, 0); ok {
		t.Error("status bar click should not hit any panel")
	}
}

func TestHandleMousePress_DisabledIsNoOp(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.SetMouseEnabled(false)
	m := appWithSizeAndCfg(t, 120, 40, cfg)
	cmd := m.handleMousePress(tea.MouseMsg{X: 10, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if cmd != nil {
		t.Error("MouseEnabled=false must short-circuit dispatcher")
	}
}

func TestHandleMousePress_LeftClickSwitchesFocus(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	m.activePanel = SidebarPanel
	// Click into table area.
	m.handleMousePress(tea.MouseMsg{X: 50, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.activePanel != TablePanel {
		t.Errorf("left-click on table area must focus TablePanel, got %v", m.activePanel)
	}
}

func TestHandleMousePress_RightClickSynthesizesSpace(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	cmd := m.handleMousePress(tea.MouseMsg{X: 50, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonRight})
	if cmd == nil {
		t.Fatal("right-click on panel must return a cmd batch")
	}
	// The space synthesis is the second cmd in the batch; reach it by
	// invoking the returned cmd.
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		spaceFound := false
		for _, c := range batch {
			if c == nil {
				continue
			}
			if k, ok := c().(tea.KeyMsg); ok && k.Type == tea.KeySpace {
				spaceFound = true
			}
		}
		if !spaceFound {
			t.Error("right-click batch missing synthesized KeySpace")
		}
	}
}

func TestHandleMousePress_DoubleClickSynthesizesEnter(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	// First click — no Enter, just focus + cursor.
	first := m.handleMousePress(tea.MouseMsg{X: 50, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if cmd := first; cmd != nil {
		if msg, ok := cmd().(tea.KeyMsg); ok && msg.Type == tea.KeyEnter {
			t.Error("first click must not synthesize Enter")
		}
	}
	// Second click, same cell, same panel — should fire Enter.
	second := m.handleMousePress(tea.MouseMsg{X: 50, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if second == nil {
		t.Fatal("second click in double window must return a cmd")
	}
	enterFound := false
	if msg, ok := second().(tea.BatchMsg); ok {
		for _, c := range msg {
			if c == nil {
				continue
			}
			if k, ok := c().(tea.KeyMsg); ok && k.Type == tea.KeyEnter {
				enterFound = true
			}
		}
	} else if k, ok := second().(tea.KeyMsg); ok && k.Type == tea.KeyEnter {
		enterFound = true
	}
	if !enterFound {
		t.Error("double-click must synthesize Enter")
	}
}

func TestTableModel_SetCursorAtScreenY_SkipsHeaderAndBorder(t *testing.T) {
	th := theme.DefaultTheme()
	tbl := NewTableModel(th)
	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}})
	// screenY=0 = top border, screenY=1 = header, screenY=2 = first row.
	// Note SetCursorAtScreenY subtracts the status bar inside AppModel —
	// here we call it with the panel-relative value already.
	tbl.SetCursorAtScreenY(2)
	if tbl.cursor != 0 {
		t.Errorf("screenY=2 should select first data row, cursor=%d", tbl.cursor)
	}
	tbl.SetCursorAtScreenY(1)
	if tbl.cursor != 0 {
		t.Error("click on header must not move cursor")
	}
	tbl.SetCursorAtScreenY(0)
	if tbl.cursor != 0 {
		t.Error("click on border must not move cursor")
	}
	tbl.SetCursorAtScreenY(3)
	if tbl.cursor != 1 {
		t.Errorf("screenY=3 should select row 1, cursor=%d", tbl.cursor)
	}
}
