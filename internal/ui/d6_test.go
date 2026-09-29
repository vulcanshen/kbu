package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	overlay "github.com/rmhubbert/bubbletea-overlay"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// wideIcon is a Nerd Font icon: one cell on a normal font, two where the
// probe finds icons two cells wide.
var wideIcon = string(rune(0xf015))

func restoreIconCells(v int) { iconCells = v }

// d6App is the whole app at w × h with icons on screen before any popup:
// the helm mark on a panel 2 row, the live glyph on panel 3's Logs tab.
func d6App(t *testing.T, w, h int) AppModel {
	t.Helper()
	m := stackTestApp(t)
	cols := ColumnsForResource(k8s.ResourcePods)
	row := make([]string, len(cols))
	for i := range row {
		row[i] = "web"
	}
	row[0] = "\U000f0833" // the helm mark
	m.currentResource = k8s.ResourcePods
	m.items = []k8s.ResourceItem{podItem("web", helmLabels)}
	m.table.SetColumns(cols)
	m.table.SetRows([][]string{row})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(AppModel)
}

// d6Popups opens each kind of popup on the app and returns the box as it is
// drawn alone.
var d6Popups = []struct {
	name string
	open func(m *AppModel) string
}{
	{"Space menu", func(m *AppModel) string {
		openTestSpaceMenu(m)
		return m.spaceMenu.renderFullPopup()
	}},
	{"sort", func(m *AppModel) string {
		m.listPicker.SetSize(m.width, m.height)
		_ = m.listPicker.Open("sort", "Sort", []ListPickerItem{{Key: "name", Label: "Name " + wideIcon}})
		m.listPicker.animator.Finalize()
		return m.listPicker.renderFullPopup()
	}},
	{"Settings", func(m *AppModel) string {
		m.settingsPopup.SetSize(m.width, m.height)
		_ = m.settingsPopup.Open([]SettingsItem{{Key: "scroll", Label: "Scroll"}})
		m.settingsPopup.animator.Finalize()
		return m.settingsPopup.renderFullPopup()
	}},
	{"namespace picker, loading", func(m *AppModel) string {
		m.namespacePicker.SetSize(m.width, m.height)
		_ = m.namespacePicker.OpenLoading()
		m.namespacePicker.animator.Finalize()
		return m.namespacePicker.renderFullPopup()
	}},
	{"namespace picker", func(m *AppModel) string {
		m.namespacePicker.SetSize(m.width, m.height)
		_ = m.namespacePicker.OpenLoading()
		m.namespacePicker.SetNamespaces([]string{"default", "kube-system"})
		m.namespacePicker.animator.Finalize()
		return m.namespacePicker.renderFullPopup()
	}},
	{"context picker", func(m *AppModel) string {
		m.contextPicker.SetSize(m.width, m.height)
		_ = m.contextPicker.Open([]string{"orbstack", "prod"}, "orbstack")
		m.contextPicker.animator.Finalize()
		return m.contextPicker.renderFullPopup()
	}},
	{"App log", func(m *AppModel) string {
		m.appLog.SetSize(m.width, m.height)
		m.appLog.Info("pinned " + wideIcon + " Pods")
		_ = m.appLog.Toggle()
		m.appLog.animator.Finalize()
		return m.appLog.renderFullPopup()
	}},
	{"breadcrumb", func(m *AppModel) string {
		m.breadcrumbPopup.SetSize(m.width, m.height)
		_ = m.breadcrumbPopup.Open([]k8s.RefTarget{{Type: k8s.ResourcePods, Name: "a"}, {Type: k8s.ResourceDeployments, Name: "b"}})
		m.breadcrumbPopup.animator.Finalize()
		return m.breadcrumbPopup.renderFullPopup()
	}},
	{"YAML", func(m *AppModel) string {
		m.yamlPopup.SetSize(m.width, m.height)
		_ = m.yamlPopup.Open("kind: Pod\nmetadata:\n  name: web "+wideIcon+"\n", k8s.ResourcePods, podItem("web", nil))
		m.yamlPopup.animator.Finalize()
		return m.yamlPopup.renderFullPopup()
	}},
	{"YAML selecting", func(m *AppModel) string {
		m.yamlPopup.SetSize(m.width, m.height)
		_ = m.yamlPopup.Open("kind: Pod\n", k8s.ResourcePods, podItem("web", nil))
		m.yamlPopup.animator.Finalize()
		m.yamlPopup, _ = m.yamlPopup.Update(key("v"))
		return m.yamlPopup.renderFullPopup()
	}},
	{"Compare", func(m *AppModel) string {
		m.comparePopup.SetSize(m.width, m.height)
		_ = m.comparePopup.Open("a: 1\n", "a: 2 "+wideIcon+"\n", "l", "r")
		m.comparePopup.animator.Finalize()
		return m.comparePopup.renderFrame()
	}},
	{"Alterm", func(m *AppModel) string {
		m.shellPty = hookedPtyView(PtyKindShell)
		m.shellPty.SetLayer(1)
		m.shellPty.SetSize(m.width, m.height)
		m.shellPty.term.Resize(m.shellPty.ptyDims()) // SetSize resizes only a live PTY
		return m.shellPty.RenderPopup()
	}},
	{"confirm", func(m *AppModel) string {
		m.confirm.SetSize(m.width, m.height)
		_ = m.confirm.Show(ConfirmDelete, "Delete pod/web "+wideIcon+"?", "kubectl delete pod web", nil)
		m.confirm.animator.Finalize()
		return m.confirm.renderFullPopup()
	}},
	{"key reference", func(m *AppModel) string {
		_ = m.openKeyRef()
		m.help.animator.Finalize()
		return m.help.renderFullPopup()
	}},
	{"toast", func(m *AppModel) string {
		_ = m.toast.Show("Copied " + wideIcon + " path")
		m.toast.animator.Finalize()
		return m.toast.RenderPopup()
	}},
}

// tdp D6, L4: with icons one cell wide and two, every popup — alone, and
// laid over the screen — keeps every line exactly as wide as it should be:
// the box its own width, the screen the terminal's. At 80 × 40 (L1) and
// wider.
func TestD6_EveryPopupEveryLineExact(t *testing.T) {
	defer restoreIconCells(iconCells)
	for _, cells := range []int{1, 2} {
		iconCells = cells
		for _, size := range [][2]int{{120, 40}, {80, 40}} {
			base := d6App(t, size[0], size[1])
			for r, line := range strings.Split(base.View(), "\n") {
				if got := dispWidth(line); got != size[0] {
					t.Errorf("icons %d, %dx%d, no popup: row %d is %d wide\n  %q", cells, size[0], size[1], r, got, ansi.Strip(line))
				}
			}
			for _, p := range d6Popups {
				m := d6App(t, size[0], size[1])
				box := strings.Split(p.open(&m), "\n")
				want := dispWidth(box[0])
				for r, line := range box {
					if got := dispWidth(line); got != want {
						t.Errorf("icons %d, %dx%d, %s alone: row %d is %d wide, row 0 is %d\n  %q", cells, size[0], size[1], p.name, r, got, want, ansi.Strip(line))
					}
				}
				for r, line := range strings.Split(m.View(), "\n") {
					if got := dispWidth(line); got != size[0] {
						t.Errorf("icons %d, %dx%d, %s on screen: row %d is %d wide\n  %q", cells, size[0], size[1], p.name, r, got, ansi.Strip(line))
					}
				}
			}
		}
	}
}

// tdp D6: the splash's pixels are two cells whatever the icon width — the
// glyph and a space, or the glyph alone where it is drawn two cells wide —
// so the logo rows line up and the screen keeps its width.
func TestD6_SplashPixelsTwoCells(t *testing.T) {
	defer restoreIconCells(iconCells)
	for _, cells := range []int{1, 2} {
		iconCells = cells
		s := NewSplashModel()
		_ = s.Show()
		s.revealedCount = len(s.pixelOrder) / 2 // lit and unlit cells side by side
		for r, line := range strings.Split(s.Render(100, 40), "\n") {
			if got := dispWidth(line); got != 100 {
				t.Errorf("icons %d: splash row %d is %d wide, want 100", cells, r, got)
			}
		}
		logoW := len(logoPixels[0]) * 2
		lit := 0
		for _, line := range strings.Split(s.Render(100, 40), "\n") {
			if strings.Contains(line, pixelGlyph(0)) {
				lit++
				if got := dispWidth(strings.TrimSpace(ansi.Strip(line))); got > logoW {
					t.Errorf("icons %d: a logo row is %d wide, the logo %d", cells, got, logoW)
				}
			}
		}
		if lit == 0 {
			t.Fatalf("icons %d: no pixel drawn", cells)
		}
	}
}

// compositeDisp lays fg over bg by display width: a popup row with an icon,
// an icon under the popup, an icon cut by the popup's left or right edge
// (the half left outside becomes a space). Placement matches overlay's.
func TestD6_CompositeDisp(t *testing.T) {
	defer restoreIconCells(iconCells)
	iconCells = 2
	I := wideIcon
	for _, c := range []struct {
		name, fg, bg string
		x            int
		want         string
	}{
		{"icon in the popup", "a" + I + "b", "0123456789", 2, "01a" + I + "b6789"},
		{"icon under the popup", "XY", "ab" + I + "cdefg", 4, "ab" + I + "XYefg"},
		{"icon cut by the left edge", "XY", "a" + I + "bcdef", 2, "a XYcdef"},
		{"icon cut by the right edge", "XY", "abc" + I + "de", 2, "abXY de"},
	} {
		got := compositeDisp(c.fg, c.bg, overlay.Left, overlay.Top, c.x, 0)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		if dispWidth(got) != dispWidth(c.bg) {
			t.Errorf("%s: %d wide, the screen is %d", c.name, dispWidth(got), dispWidth(c.bg))
		}
	}
	bg := strings.Repeat(strings.Repeat(".", 10)+"\n", 4) + strings.Repeat(".", 10)
	got := strings.Split(compositeDisp("AB\nCD", bg, overlay.Center, overlay.Center, 0, 0), "\n")
	if got[1] != "....AB...." || got[2] != "....CD...." {
		t.Errorf("centred box landed wrong:\n%s", strings.Join(got, "\n"))
	}
	got = strings.Split(compositeDisp("AB", bg, overlay.Center, overlay.Bottom, 0, -1), "\n")
	if got[3] != "....AB...." {
		t.Errorf("bottom box one row up landed wrong:\n%s", strings.Join(got, "\n"))
	}

	// A box wider or taller than the screen — the frame a resize lands in —
	// is cut at the screen's edge instead of panicking.
	wide := strings.Repeat("W", 14) + "\n" + strings.Repeat("W", 14)
	tall := strings.Repeat("T\n", 7) + "T"
	for name, fg := range map[string]string{"wider": wide, "taller": tall} {
		for _, pos := range []overlay.Position{overlay.Center, overlay.Bottom} {
			rows := strings.Split(compositeDisp(fg, bg, overlay.Center, pos, 0, -1), "\n")
			if len(rows) != 5 {
				t.Errorf("a %s box made %d rows, the screen has 5", name, len(rows))
			}
			for r, row := range rows {
				if dispWidth(row) != 10 {
					t.Errorf("a %s box: row %d is %d wide, the screen 10: %q", name, r, dispWidth(row), row)
				}
			}
		}
	}
}

// The width functions themselves, with icons two cells wide; and centerDisp
// centres like lipgloss.Place when there are no icons.
func TestD6_WidthFunctions(t *testing.T) {
	defer restoreIconCells(iconCells)
	iconCells = 2
	for _, c := range []struct {
		s    string
		n    int
		want string
	}{
		{"abcdef", 2, "cdef"},
		{"a" + wideIcon + "bc", 3, "bc"},
		{"a" + wideIcon + "bc", 2, " bc"},
		{"ab", 5, ""},
	} {
		if got := dispCutLeft(c.s, c.n); got != c.want {
			t.Errorf("dispCutLeft(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
	for _, c := range []struct{ got, want string }{
		{dispClip("abc"+wideIcon+"de", 2), "ab"},
		{dispClip("a"+wideIcon+"bcd", 4), "a" + wideIcon + "b"},
		{truncate("abcdef"+wideIcon, 4), "abc…"},
		{truncate(wideIcon+"abcdef", 5), wideIcon + "ab…"},
		{truncateSidebarLabel("a"+wideIcon+"bcd", 7), "a" + wideIcon + "bcd"}, // 8 bytes, 6 cells: it fits
		{truncateSidebarLabel("a"+wideIcon+"bcd", 5), "a" + wideIcon + "b…"},
		{ansiTruncate("a"+wideIcon+"bcd", 4), "a" + wideIcon + "b\x1b[0m"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	if got := dispWidth(" " + wideIcon + " name"); got != 8 {
		t.Errorf("dispWidth with a wide icon = %d, want 8", got)
	}
	if got := dispWidth(joinH(padDisp(" "+wideIcon+" x", 5), padDisp("abc", 3))); got != 8 {
		t.Errorf("joinH total = %d, want 8", got)
	}
	iconCells = 1
	if got := dispWidth(" " + wideIcon + " name"); got != 7 {
		t.Errorf("dispWidth on a normal font = %d, want 7", got)
	}
	for _, s := range []string{"ab", "abc\nd", "x\nlonger"} {
		for _, wh := range [][2]int{{7, 5}, {8, 4}, {3, 1}} {
			if got, want := centerDisp(wh[0], wh[1], s), lipgloss.Place(wh[0], wh[1], lipgloss.Center, lipgloss.Center, s); got != want {
				t.Errorf("centerDisp(%d, %d, %q) = %q, lipgloss.Place gives %q", wh[0], wh[1], s, got, want)
			}
		}
	}
}

// tdp D6 in panel 2: a row carrying the helm mark keeps every column under
// its header when icons are two cells wide — the cell is padded by the
// mark's display width, not its measured one.
func TestD6_TableColumnsLineUpPastTheHelmMark(t *testing.T) {
	defer restoreIconCells(iconCells)
	for _, cells := range []int{1, 2} {
		iconCells = cells
		cols := ColumnsForResource(k8s.ResourcePods)
		row := make([]string, len(cols))
		for i := range row {
			row[i] = "v" + string(rune('a'+i))
		}
		row[0] = "\U000f0833" // the helm mark
		tbl := NewTableModel(stackTestApp(t).theme)
		tbl.SetColumns(cols)
		tbl.SetRows([][]string{row})
		tbl.SetSize(120, 5)
		lines := strings.Split(ansi.Strip(tbl.View()), "\n")
		header, body := lines[0], lines[1]
		checked := 0
		for i := 1; i < len(cols); i++ {
			if cols[i].Title == "" {
				continue
			}
			v := strings.Index(body, row[i])
			if v < 0 {
				t.Fatalf("icons %d: the %q value is not drawn: %q", cells, cols[i].Title, body)
			}
			// The header is plain ASCII, so its cell c is its byte c.
			if vc := dispWidth(body[:v]); vc >= len(header) || !strings.HasPrefix(header[vc:], cols[i].Title) {
				t.Errorf("icons %d: the %q value starts at cell %d, not under its header:\n%s\n%s", cells, cols[i].Title, vc, header, body)
			}
			checked++
		}
		if checked < 3 {
			t.Fatalf("icons %d: only %d columns checked", cells, checked)
		}
	}
}

// isWideIcon: Nerd Font icons count, powerline caps and box drawing do not.
func TestD6_IsWideIcon(t *testing.T) {
	for _, c := range []struct {
		r    rune
		want bool
	}{
		{0xf015, true}, {0xf0833, true}, {0xf0a9e, true}, {0xe0b6, false}, {0xe0b4, false},
		{0x2502, false}, {'a', false}, {0x4e2d, false},
	} {
		if got := isWideIcon(c.r); got != c.want {
			t.Errorf("isWideIcon(U+%X) = %v, want %v", c.r, got, c.want)
		}
	}
}

// KBU__ICON_WIDTH overrides the probe with 1 or 2; anything else is ignored.
func TestD6_IconWidthOverride(t *testing.T) {
	for _, c := range []struct {
		v    string
		n    int
		isOK bool
	}{{"2", 2, true}, {" 1 ", 1, true}, {"3", 0, false}, {"", 0, false}, {"x", 0, false}} {
		t.Setenv("KBU__ICON_WIDTH", c.v)
		if n, ok := iconWidthOverride(); n != c.n || ok != c.isOK {
			t.Errorf("KBU__ICON_WIDTH=%q: got (%d, %v), want (%d, %v)", c.v, n, ok, c.n, c.isOK)
		}
	}
}
