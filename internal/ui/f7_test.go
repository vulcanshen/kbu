package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

// popupBox is a popup's rendered width (its top border, in display
// cells) and height.
func popupBox(s string) (w, h int) {
	lines := strings.Split(s, "\n")
	return ansi.StringWidth(lines[0]), len(lines)
}

// everyPopup renders each popup at w×h with short content — nothing in
// it is wide enough to push the width.
func everyPopup(t *testing.T, w, h int) map[string]string {
	t.Helper()
	th := theme.DefaultTheme()
	out := map[string]string{}

	menu := NewSpaceMenuModel(th)
	menu.SetSize(w, h)
	_ = menu.Open(menuTitle(menuTitleGlyph, "[2] pods/a"), []menuItem{{label: "Copy", key: "y", hint: "name"}}, k8s.ResourcePods, k8s.ResourceItem{Name: "a"})
	out["space menu"] = menu.renderFullPopup()

	lp := NewListPickerModel(th)
	lp.SetSize(w, h)
	_ = lp.Open("sort", "Sort", []ListPickerItem{{Label: "Name"}})
	out["list picker"] = lp.renderFullPopup()

	sp := NewSettingsPopupModel(th)
	sp.SetSize(w, h)
	_ = sp.Open([]SettingsItem{{Key: "mouse", Label: "Mouse", ValueText: "ON"}})
	out["settings"] = sp.renderFullPopup()

	ns := NewNamespacePickerModel(th)
	ns.SetSize(w, h)
	_ = ns.OpenLoading()
	ns.SetNamespaces([]string{"default"})
	out["namespace picker"] = ns.renderFullPopup()

	cp := NewContextPickerModel(th)
	cp.SetSize(w, h)
	_ = cp.Open([]string{"dev"}, "dev")
	out["context picker"] = cp.renderFullPopup()

	al := NewAppLogModel(th)
	al.SetSize(w, h)
	al.Info("hi")
	_ = al.Toggle()
	out["app log"] = al.renderFullPopup()

	bc := NewBreadcrumbPopupModel(th)
	bc.SetSize(w, h)
	_ = bc.Open([]k8s.RefTarget{{Type: k8s.ResourcePods, Name: "a"}})
	out["breadcrumb"] = bc.renderFullPopup()

	yp := NewYamlPopupModel(th)
	yp.SetSize(w, h)
	_ = yp.Open("a: 1\n", k8s.ResourcePods, k8s.ResourceItem{Name: "a"}, "dev")
	out["yaml"] = yp.renderFullPopup()

	cmp := NewCompareYamlPopupModel(th)
	cmp.SetSize(w, h)
	_ = cmp.Open("a: 1\n", "a: 2\n", "x", "y")
	out["compare"] = cmp.renderFrame()

	cf := NewConfirmModel(th)
	cf.SetSize(w, h)
	_ = cf.Show(ConfirmEdit, "Edit a?", "", nil)
	out["confirm"] = cf.renderFullPopup()

	hp := NewHelpModel(th)
	hp.SetSize(w, h)
	_ = hp.Open("keys", []helpRow{{key: "j", desc: "down"}})
	out["key reference"] = hp.renderFullPopup()

	ts := NewToastModel(th)
	ts.SetSize(w)
	_ = ts.Show("saved")
	ts.animator.Finalize()
	out["toast"] = ts.RenderPopup()
	return out
}

// tdp F7: every popup is min(W − 2, 120) wide, whatever it shows — 78 on
// an 80-column terminal, 120 on a 200-column one (the cap).
func TestF7_EveryPopupIsOneWidth(t *testing.T) {
	for _, c := range []struct{ w, want int }{{80, 78}, {200, 120}} {
		for name, s := range everyPopup(t, c.w, 40) {
			if got, _ := popupBox(s); got != c.want {
				t.Errorf("%s at W=%d: %d wide, want %d", name, c.w, got, c.want)
			}
		}
	}
}

// tdp F7: the terminals are the exception — they fill the screen, W − 2
// × H − 2, and the 120 cap does not apply.
func TestF7_TerminalFillsTheScreen(t *testing.T) {
	for _, c := range []struct{ w, h, cols int }{{80, 24, 76}, {200, 60, 196}} {
		v := PtyView{hostW: c.w, hostH: c.h}
		if cols, _ := v.ptyDims(); cols != c.cols {
			t.Errorf("terminal content at %d×%d: %d columns, want %d (frame W − 2)", c.w, c.h, cols, c.cols)
		}
	}
}

// tdp F7: a note is as tall as its content, capped by the screen less a
// row above and below.
func TestF7_HeightFollowsContent(t *testing.T) {
	th := theme.DefaultTheme()
	yaml := func(lines int) int {
		yp := NewYamlPopupModel(th)
		yp.SetSize(120, 40)
		var b strings.Builder
		for i := 0; i < lines; i++ {
			fmt.Fprintf(&b, "k%d: v\n", i)
		}
		_ = yp.Open(b.String(), k8s.ResourcePods, k8s.ResourceItem{Name: "a"}, "dev")
		_, h := popupBox(yp.renderFullPopup())
		return h
	}
	if got := yaml(15); got != 17 {
		t.Errorf("15-line YAML: popup %d tall, want 17 (lines + 2 borders)", got)
	}
	if got := yaml(3); got != 10 {
		t.Errorf("3-line YAML: popup %d tall, want the 10-row floor", got)
	}
	if got := yaml(200); got != 38 {
		t.Errorf("200-line YAML: popup %d tall, want 38 (H − 2, then it scrolls)", got)
	}

	hp := NewHelpModel(th)
	hp.SetSize(120, 40)
	_ = hp.Open("keys", []helpRow{{key: "j", desc: "down"}, {key: "k", desc: "up"}})
	if _, h := popupBox(hp.renderFullPopup()); h != 6 {
		t.Errorf("2-row key reference: %d tall, want 6 (rows + padding + borders)", h)
	}
	many := make([]helpRow, 100)
	for i := range many {
		many[i] = helpRow{key: "x", desc: "y"}
	}
	_ = hp.Open("keys", many)
	if _, h := popupBox(hp.renderFullPopup()); h != 38 {
		t.Errorf("100-row key reference: %d tall, want 38 (H − 2, then it scrolls)", h)
	}
}

// tdp F7: the height is fixed when the popup opens — log entries that
// arrive while the App log is open scroll in the box, not grow it.
func TestF7_AppLogHeightFixedAtOpen(t *testing.T) {
	al := NewAppLogModel(theme.DefaultTheme())
	al.SetSize(120, 40)
	al.Info("one")
	al.Info("two")
	_ = al.Toggle()
	al.animator.Finalize()
	_, before := popupBox(al.renderFullPopup())
	for i := 0; i < 10; i++ {
		al.Info(fmt.Sprintf("later %d", i))
	}
	if _, after := popupBox(al.renderFullPopup()); after != before {
		t.Errorf("App log grew from %d to %d rows while open", before, after)
	}
	if before >= 36 {
		t.Errorf("a 2-entry App log is %d rows tall — its height should follow its content", before)
	}
}

// tdp F7: the toast sits at the bottom of the screen, just above the
// footer, not in the middle.
func TestF7_ToastAtTheBottom(t *testing.T) {
	m := screenAt(t, 120, 40)
	footer := strings.Split(m.View(), "\n")[39]
	_ = m.toast.Show("saved the thing")
	m.toast.animator.Finalize()
	lines := strings.Split(m.View(), "\n")
	row := -1
	for i, l := range lines {
		if strings.Contains(ansi.Strip(l), "saved the thing") {
			row = i
		}
	}
	// border, padding row, message, padding row, border: the box ends on
	// row 38, so the message is on row 36.
	if row != 36 {
		t.Errorf("toast message on row %d, want 36 (box ends on row 38, above the footer)", row)
	}
	if lines[39] != footer {
		t.Errorf("the toast covered the footer")
	}
}

// A click lands on the row that is drawn there. The overlay centers a
// popup at W/2 − w/2, H/2 − h/2; the hit test used (H − h)/2, one row off
// when the screen height is even and the popup's odd.
func TestF7_ClickHitsTheDrawnRow(t *testing.T) {
	m := appWithSizeAndCfg(t, 120, 40, config.DefaultConfig())
	m.ready = true
	_ = m.settingsPopup.Open([]SettingsItem{
		{Key: "first", Label: "First", ValueText: "ON"},
		{Key: "second", Label: "Second", ValueText: "ON"},
		{Key: "third", Label: "Third", ValueText: "ON"},
	})
	m.settingsPopup.animator.Finalize()
	if _, h := popupBox(m.settingsPopup.renderFullPopup()); h%2 == 0 {
		t.Fatalf("test needs an odd-height popup, got %d", h)
	}
	for i, l := range strings.Split(m.View(), "\n") {
		plain := ansi.Strip(l)
		col := strings.Index(plain, "Second")
		if col < 0 {
			continue
		}
		_, cmd := m.Update(tea.MouseMsg{X: lipgloss.Width(plain[:col]), Y: i, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		if cmd == nil {
			t.Fatalf("click on the drawn Second row (screen row %d) did nothing", i)
		}
		if got, ok := cmd().(SettingsToggleMsg); !ok || got.Key != "second" {
			t.Errorf("click on the drawn Second row committed %#v, want second", got)
		}
		return
	}
	t.Fatal("Second row not on screen")
}

// The Space menu title shows its glyph once.
func TestF7_MenuTitleGlyphOnce(t *testing.T) {
	m := stackTestApp(t)
	m.activePanel = TablePanel
	_ = m.openSpaceMenu()
	top := strings.Split(m.spaceMenu.renderFullPopup(), "\n")[0]
	if n := strings.Count(top, menuTitleGlyph); n != 1 {
		t.Errorf("menu title has the glyph %d times: %q", n, ansi.Strip(top))
	}
}
