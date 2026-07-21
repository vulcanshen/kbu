package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
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
		shellPty:        NewPtyView("ptyview_shell"),
		txPty:           NewPtyView("ptyview_tx"),
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

func TestTableModel_SetCursorAtScreenY_AccountsForSearchBox(t *testing.T) {
	// When the table is in search mode, the rendered layout is
	// [border][search box 3 lines][header][data]. Before the fix
	// the cursor calc treated it as [border][header][data] — every
	// click landed 3 rows above where the user pointed. Verify the
	// post-fix math.
	th := theme.DefaultTheme()
	tbl := NewTableModel(th)
	tbl.SetRows([][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}})
	tbl.searching = true
	tbl.searchQuery = ""

	// screenY=5 (border + 3-line search box + header) = first data row
	tbl.SetCursorAtScreenY(5)
	if tbl.cursor != 0 {
		t.Errorf("screenY=5 in search mode should select row 0, cursor=%d", tbl.cursor)
	}
	tbl.SetCursorAtScreenY(7)
	if tbl.cursor != 2 {
		t.Errorf("screenY=7 in search mode should select row 2, cursor=%d", tbl.cursor)
	}
	// Clicks inside the search-box rows must not move cursor.
	tbl.cursor = 0
	tbl.SetCursorAtScreenY(2) // inside the search box
	if tbl.cursor != 0 {
		t.Error("click inside the search box must not move cursor")
	}
	tbl.SetCursorAtScreenY(4) // header row
	if tbl.cursor != 0 {
		t.Error("click on header in search mode must not move cursor")
	}
}

func TestSidebarModel_SetCursorAtScreenY_AccountsForSearchBox(t *testing.T) {
	// Same off-by-2 fix as the table — sidebar's pre-fix code only
	// stepped over 1 search-box line instead of 3.
	th := theme.DefaultTheme()
	sb := NewSidebarModel(th)
	sb.searching = true
	sb.searchQuery = ""

	// Pre-fix this would have selected the wrong row. After fix the
	// math expects screenY = 1 (border) + 3 (search box) + 0 (first
	// item) = 4 for the first content row. We can't assert which
	// resource type it lands on without a fixture, but at least make
	// sure clicks inside the search box don't move the cursor.
	prev := sb.cursor
	sb.SetCursorAtScreenY(2) // search box row
	if sb.cursor != prev {
		t.Error("click inside the sidebar search box must not move cursor")
	}
	sb.SetCursorAtScreenY(3) // last search box row
	if sb.cursor != prev {
		t.Error("click on the search box's bottom border must not move cursor")
	}
}

func TestIsMenuPopupActive_GatesWheelSynth(t *testing.T) {
	// AppModel.isMenuPopupActive backs the "ignore wheel when a
	// menu popup is open" gate. Verify each menu popup flips it on,
	// and that the viewer popups (yamlPopup, comparePopup, appLog,
	// help) do NOT — they keep wheel scroll.
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	if m.isMenuPopupActive() {
		t.Fatal("no popup open → isMenuPopupActive should be false")
	}

	// Manually flip a menu popup's animator to PopupOpen so IsActive
	// reports true without driving the open animation.
	m.panel2Menu.animator.State = PopupOpen
	if !m.isMenuPopupActive() {
		t.Error("panel2Menu open → isMenuPopupActive should be true")
	}
	m.panel2Menu.animator.State = PopupClosed

	// Viewer popups must NOT count as menu popups.
	m.yamlPopup.animator.State = PopupOpen
	if m.isMenuPopupActive() {
		t.Error("yamlPopup (viewer) must not count as menu popup — wheel should still work")
	}
}

func TestHandleMousePress_SettingsEscapeHatchWhenMouseDisabled(t *testing.T) {
	// When mouse is disabled, the Settings popup itself must still
	// accept mouse — otherwise users who toggle Mouse OFF can't
	// turn it back on without keyboard.
	cfg := config.DefaultConfig()
	cfg.SetMouseEnabled(false)
	m := appWithSizeAndCfg(t, 120, 40, cfg)
	m.settingsPopup.animator.State = PopupOpen
	m.settingsPopup.items = []SettingsItem{{Key: "mouse", Label: "Mouse", ValueText: "OFF"}}

	// Drive the real dispatcher path via Update.
	// Click somewhere inside the rendered popup region — exact
	// coords don't matter for this guard; we only need to prove the
	// dispatcher reached settingsPopup.HandleMouse despite the
	// MouseEnabled gate.
	popup := m.settingsPopup.renderFullPopup()
	lines := strings.Split(popup, "\n")
	w := lipgloss.Width(lines[0])
	h := len(lines)
	px := (120 - w) / 2
	py := (40 - h) / 2

	clickMsg := tea.MouseMsg{
		X:      px + 5,
		Y:      py + 2, // first item row
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	updated, cmd := m.Update(clickMsg)
	mAfter := updated.(AppModel)
	_ = mAfter
	// Expect a SettingsToggleMsg cmd queued.
	if cmd == nil {
		t.Fatal("settings popup mouse must still emit a cmd when MouseEnabled=false")
	}
	got := cmd()
	if _, ok := got.(SettingsToggleMsg); !ok {
		t.Errorf("expected SettingsToggleMsg from escape-hatch path, got %T", got)
	}
}
