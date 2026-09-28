package ui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/vulcanshen/kbu/internal/theme"
)

// unset marks a cell with no background of its own (the terminal's).
var unset = [3]int{-1, -1, -1}

// textRGB is the default foreground, as dimANSI takes it.
var textRGB = [3]int{0xcd, 0xd6, 0xf4}

// cell is one screen cell as a truecolor terminal draws it.
type cell struct {
	r      rune
	fg, bg [3]int
}

// screenCells parses a styled screen into cells: the colours each cell is
// drawn with (fg unset = the default text colour, bg unset = none).
func screenCells(view string) [][]cell {
	var out [][]cell
	for _, line := range strings.Split(view, "\n") {
		var row []cell
		fg, bg := textRGB, unset
		for i := 0; i < len(line); {
			if strings.HasPrefix(line[i:], "\x1b[") {
				end := i + 2
				for end < len(line) && (line[end] < 0x40 || line[end] > 0x7e) {
					end++
				}
				if end < len(line) && line[end] == 'm' {
					ps := strings.Split(line[i+2:end], ";")
					for k := 0; k < len(ps); k++ {
						switch ps[k] {
						case "", "0":
							fg, bg = textRGB, unset
						case "39":
							fg = textRGB
						case "49":
							bg = unset
						case "38", "48":
							if k+4 < len(ps) && ps[k+1] == "2" {
								var c [3]int
								for j := 0; j < 3; j++ {
									c[j], _ = strconv.Atoi(ps[k+2+j])
								}
								if ps[k] == "38" {
									fg = c
								} else {
									bg = c
								}
								k += 4
							}
						}
					}
				}
				i = end + 1
				continue
			}
			var r rune
			size := 1
			for _, rr := range line[i:] {
				r, size = rr, len(string(rr))
				break
			}
			for w := ansi.StringWidth(string(r)); w > 0; w-- {
				row = append(row, cell{r, fg, bg})
			}
			i += size
		}
		out = append(out, row)
	}
	return out
}

// dimmedBG is what a background becomes under the dim: faded, or still
// none when it had none.
func dimmedBG(c [3]int) [3]int {
	if c == unset {
		return unset
	}
	return dimRGB(c)
}

// truecolor draws styles as 24-bit SGR for the test, as a real terminal gets.
func truecolor(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

func hexRGB(s string) [3]int {
	s = strings.TrimPrefix(s, "#")
	var out [3]int
	for k := 0; k < 3; k++ {
		v, _ := strconv.ParseUint(s[2*k:2*k+2], 16, 8)
		out[k] = int(v)
	}
	return out
}

// near compares colours allowing the rounding termenv does from hex to RGB.
func near(a, b [3]int) bool {
	for k := range a {
		if d := a[k] - b[k]; d > 2 || d < -2 {
			return false
		}
	}
	return true
}

// inBox reports whether (x, y) is inside a popup drawn centred on the screen.
func inBox(x, y int, popup string, w, h int) bool {
	pw, ph := popupBox(popup)
	px, py := popupOrigin(pw, ph, w, h)
	return x >= px && x < px+pw && y >= py && y < py+ph
}

// withConfirm opens a confirm over whatever is open, fully drawn.
func withConfirm(m *AppModel) {
	m.confirm.SetSize(m.width, m.height)
	m.confirm.SetLayer(m.popupDepth() + 1)
	_ = m.confirm.Show(ConfirmEdit, "Edit pod/nginx?", "", nil)
	m.confirm.animator.Finalize()
}

// tdp F8, D2: with a popup open, every cell outside it is the same cell
// dimmed — foreground and background each faded toward the base, the
// text and layout untouched. Nothing is left at full strength, nothing
// loses its background.
func TestF8_EverythingBelowTheTopIsDimmed(t *testing.T) {
	truecolor(t)
	m := screenAt(t, 120, 40)
	before := screenCells(m.View())
	withConfirm(&m)
	after := screenCells(m.View())
	popup := m.confirm.renderFullPopup()

	checked := 0
	for y := range before {
		for x := 0; x < len(before[y]) && x < len(after[y]); x++ {
			if inBox(x, y, popup, 120, 40) {
				continue
			}
			b, a := before[y][x], after[y][x]
			if a.r != b.r {
				t.Fatalf("cell (%d,%d) changed from %q to %q under the confirm", x, y, b.r, a.r)
			}
			if a.fg != dimRGB(b.fg) || a.bg != dimmedBG(b.bg) {
				t.Fatalf("cell (%d,%d) %q is fg %v bg %v under the confirm, want the dimmed fg %v bg %v (was %v / %v)",
					x, y, b.r, a.fg, a.bg, dimRGB(b.fg), dimmedBG(b.bg), b.fg, b.bg)
			}
			checked++
		}
	}
	_, ph := popupBox(popup)
	if checked < 120*(40-ph) {
		t.Errorf("only %d cells compared", checked)
	}
}

// tdp F8: the elements drawn with a background — the error badge, the
// focused panel's capsule, the compare anchor row, the sidebar cursor,
// panel 3's tab capsules — are their own colour faded under a popup, not
// gone. Expected values are D2's formula worked by hand on the colours
// termenv emits (dim(c) = c × 0.45 + #1e1e2e × 0.55, never lighter).
func TestF8_BackgroundsFadeNotVanish(t *testing.T) {
	truecolor(t)
	m := screenAt(t, 120, 40)
	withConfirm(&m)
	after := screenCells(m.View())
	for _, c := range []struct {
		what string
		x, y int
		was  [3]int
		want [3]int
	}{
		{"error badge (red)", 110, 0, [3]int{243, 139, 168}, [3]int{126, 79, 101}},
		{"[1] capsule of the focused panel (blue)", 6, 1, [3]int{137, 179, 250}, [3]int{78, 97, 138}},
		{"compare anchor row (lavender)", 40, 3, [3]int{179, 190, 254}, [3]int{97, 102, 140}},
		{"sidebar cursor (lavender)", 5, 9, [3]int{179, 190, 254}, [3]int{97, 102, 140}},
		{"panel 3 active tab capsule (surface2)", 30, 25, [3]int{88, 91, 112}, [3]int{56, 57, 76}},
		{"panel 3 inactive tab (crust, darker than the base: kept)", 40, 25, [3]int{17, 17, 27}, [3]int{17, 17, 27}},
	} {
		if got := after[c.y][c.x].bg; got != c.want {
			t.Errorf("%s at (%d,%d): background %v under the confirm, want %v (dim of %v)", c.what, c.x, c.y, got, c.want, c.was)
		}
		if dimRGB(c.was) != c.want {
			t.Errorf("%s: dimRGB(%v) = %v, hand-worked %v", c.what, c.was, dimRGB(c.was), c.want)
		}
	}
}

// tdp F8: only the top popup is bright. The Space menu beneath a confirm
// dims, and its border stays its own layer colour, faded (D2) — the
// confirm on top keeps its layer-2 colour at full strength.
func TestF8_LowerPopupBorderKeepsItsLayerColour(t *testing.T) {
	truecolor(t)
	m := screenAt(t, 120, 40)
	m.activePanel = TablePanel
	_ = m.openSpaceMenu()
	m.spaceMenu.SetSize(m.width, m.height)
	m.spaceMenu.animator.Finalize()
	withConfirm(&m)
	cells := screenCells(m.View())

	menuW, menuH := popupBox(m.spaceMenu.renderFullPopup())
	mx, my := popupOrigin(menuW, menuH, 120, 40)
	if got, want := cells[my][mx].fg, [3]int{90, 103, 138}; !near(got, want) {
		t.Errorf("Space menu corner under the confirm is %v, want dim(Lavenphire25) %v", got, want)
	}
	cw, ch := popupBox(m.confirm.renderFullPopup())
	cx, cy := popupOrigin(cw, ch, 120, 40)
	if got, want := cells[cy][cx].fg, hexRGB(theme.Lavenphire50); !near(got, want) {
		t.Errorf("confirm corner on top is %v, want its bright layer-2 colour %v", got, want)
	}
	// Two popups deep, the panels are still dimmed once, not twice.
	if got, want := cells[0][110].bg, [3]int{126, 79, 101}; got != want {
		t.Errorf("error badge under two popups is %v, want dim(red) %v — dimmed once", got, want)
	}
}

// tdp F8: the moment the top popup starts to close it is no longer the
// top — the popup beneath is bright again while the closing one fades
// out (the animation is not run here).
func TestF8_ClosingTopLightsTheOneBeneath(t *testing.T) {
	truecolor(t)
	m := screenAt(t, 120, 40)
	m.activePanel = TablePanel
	_ = m.openSpaceMenu()
	m.spaceMenu.SetSize(m.width, m.height)
	m.spaceMenu.animator.Finalize()
	withConfirm(&m)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(AppModel)
	if !m.confirm.IsActive() || m.confirm.animator.Owns() {
		t.Fatalf("test needs the confirm mid-close (active %v, owns %v)", m.confirm.IsActive(), m.confirm.animator.Owns())
	}
	cells := screenCells(m.View())
	menuW, menuH := popupBox(m.spaceMenu.renderFullPopup())
	mx, my := popupOrigin(menuW, menuH, 120, 40)
	if got, want := cells[my][mx].fg, hexRGB(theme.Lavenphire25); !near(got, want) {
		t.Errorf("Space menu corner with the confirm closing is %v, want its bright colour %v", got, want)
	}
}

// tdp F8: a toast is not a layer — it dims nothing.
func TestF8_ToastDimsNothing(t *testing.T) {
	truecolor(t)
	m := screenAt(t, 120, 40)
	before := screenCells(m.View())
	_ = m.toast.Show("Copied!")
	m.toast.animator.Finalize()
	after := screenCells(m.View())
	for y := 0; y < 30; y++ { // the toast sits at the bottom
		for x := range before[y] {
			if before[y][x] != after[y][x] {
				t.Fatalf("cell (%d,%d) changed with only a toast up: %v → %v", x, y, before[y][x], after[y][x])
			}
		}
	}
}

// D2's formula on the SGR forms kbu never writes itself but a streamed
// log or a remote session may: 16 colours, 256 colours, a reset.
func TestF8_DimSGR(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"38;2;180;190;254", "38;2;98;102;140"}, // lavender
		{"48;2;0;0;0", "48;2;0;0;0"},            // black: fading would lighten it, kept
		{"31", "38;2;109;0;0"},                  // 16-colour red (205,0,0)
		{"38;5;196", "38;2;131;0;0"},            // 256-colour red (255,0,0)
		{"0", "0;38;2;109;113;135"},             // reset: the text after it stays dim
		{"1;7", "1;7"},                          // bold, reverse untouched
	} {
		if got := dimSGR(c.in); got != c.want {
			t.Errorf("dimSGR(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
