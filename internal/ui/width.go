package ui

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

// Every width in internal/ui is measured here (tdp D6, from filu's
// width.go): padding, cutting, centring, joining, frames and laying popups
// over the screen all go through dispWidth, so a font that draws a Nerd
// Font icon two cells wide cannot push a border out of line.

// IconCells reports the detected Nerd Font icon cell width (1 or 2) — exposed
// for the `kbu iconwidth` debug command.
func IconCells() int { return iconCells }

// iconCells is how many cells the cursor actually moves past a Nerd Font
// icon. On a normal Nerd Font it is 1; some fonts (CJK "full-width icon"
// fonts) move it 2, while lipgloss/x-ansi still measure 1 — that mismatch is
// what breaks the borders. A font that draws the icon wider but moves the
// cursor one cell (the glyph overflows) is 1. DetectIconWidth (CPR probe,
// startup) sets this; the default of 1 means "no adjustment", so nothing
// changes on a normal font.
var iconCells = 1

// iconWidthOverride reads KBU__ICON_WIDTH (1 or 2), the manual override for a
// terminal whose CPR reply is missing or wrong, and the only way to set it
// where the probe does not run (Windows).
func iconWidthOverride() (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("KBU__ICON_WIDTH")))
	if err != nil || n < 1 || n > 2 {
		return 0, false
	}
	return n, true
}

// isWideIcon reports whether r is a Nerd Font file-type glyph that a CJK icon
// font renders double-width. The powerline caps (U+E0A0–E0D7, the tab-bar
// triangles/rounds) live in the PUA too but render single-width, so they are
// excluded — only file-type icons get the +1 treatment.
func isWideIcon(r rune) bool {
	if r >= 0x2160 && r <= 0x2164 {
		return true // Ⅰ..Ⅴ tab numerals: ambiguous width, drawn wide on CJK fonts
	}
	if r >= 0xe0a0 && r <= 0xe0d7 {
		return false // powerline caps — single-width even on CJK icon fonts
	}
	// BMP Private Use Area + supplementary PUA-A (Material Design icons).
	return (r >= 0xe000 && r <= 0xf8ff) || (r >= 0xf0000 && r <= 0xffffd)
}

// iconCount counts wide-icon runes in s (ANSI stripped). Fast-pathed to 0 when
// icons are single-width, so dispWidth == ansi.StringWidth on a normal font.
func iconCount(s string) int {
	if iconCells == 1 {
		return 0
	}
	n := 0
	for _, r := range ansi.Strip(s) {
		if isWideIcon(r) {
			n++
		}
	}
	return n
}

// dispWidth is the on-screen width of s: the measured width plus the extra cell
// each file-type icon eats on a CJK icon font.
func dispWidth(s string) int {
	return ansi.StringWidth(s) + iconCount(s)*(iconCells-1)
}

// dispClip trims s to display width w (ANSI- and wide-icon-aware), no ellipsis.
// The first w measured cells are at least w display cells; each icon among them
// takes one more, so step back until they fit — an icon anywhere in the line,
// not only at its start (a row under a popup has icons past the cut).
func dispClip(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispWidth(s) <= w {
		return s
	}
	target := w
	for target > 0 {
		out := ansi.Truncate(s, target, "")
		if dispWidth(out) <= w {
			return out
		}
		target--
	}
	return ""
}

// padDisp clips then space-pads s to exactly display width w.
func padDisp(s string, w int) string {
	s = dispClip(s, w)
	if d := w - dispWidth(s); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

// padDispRight clips then right-aligns s to display width w (left-padding with
// spaces) — for the numeric Size column.
func padDispRight(s string, w int) string {
	s = dispClip(s, w)
	if d := w - dispWidth(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}

// truncate clips s to display width w, appending "…" when it had to cut. Like
// dispClip it starts from w measured cells and steps back one per icon kept, so
// icons past the cut do not cost room.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispWidth(s) <= w {
		return s
	}
	target := w
	for {
		out := ansi.Truncate(s, target, "…")
		if dispWidth(out) <= w || target <= 1 {
			return out
		}
		target--
	}
}

// truncPathLeft clips s to display width w from the LEFT, keeping the tail
// visible and prepending "…" — the right choice for paths, where the end matters
// more than the root, and for a value being typed, where the cursor is at the end.
func truncPathLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispWidth(s) <= w {
		return s
	}
	return "…" + dispCutLeft(s, dispWidth(s)-(w-1))
}

// dispCutLeft drops the first n display cells of s and returns the rest, ANSI
// styles kept. A wide icon or character cut in half is replaced by spaces, so
// the result is always dispWidth(s) − n wide.
func dispCutLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	total := dispWidth(s)
	if n >= total {
		return ""
	}
	// m measured cells hold at least n display cells; start where they would
	// if every icon so far were narrow, and step up past a wide one.
	m := max(n-iconCount(s)*(iconCells-1), 0)
	for dispWidth(ansi.Truncate(s, m, "")) < n {
		m++
	}
	rest := ansi.TruncateLeft(s, m, "")
	return strings.Repeat(" ", max(total-n-dispWidth(rest), 0)) + rest
}

// compositeDisp draws fg over bg — overlay.Composite, but every width is the
// display width, so a wide icon in the popup or in what it covers cannot push
// a line past the screen (tdp D6, L4). Placement is the same: Left / Top at 0,
// Center at half the background less half the foreground (each halved on its
// own), Right / Bottom flush, then moved by the offsets and kept on screen.
func compositeDisp(fg, bg string, xPos, yPos overlay.Position, xOff, yOff int) string {
	if fg == "" {
		return bg
	}
	if bg == "" {
		return fg
	}
	fgLines, bgLines := strings.Split(fg, "\n"), strings.Split(bg, "\n")
	fgW, bgW := blockWidth(fgLines), blockWidth(bgLines)
	fgH, bgH := len(fgLines), len(bgLines)
	// A box wider or taller than the screen (drawn at the old size for the
	// frame a resize lands in) starts at 0 and is cut at the screen's edge,
	// larger both ways too: clampSpan alone would give a negative start, and a
	// negative start panics below (overlay.Composite let the row run past the
	// screen). A box the size of the screen (a PTY) simply covers it.
	x := max(clampSpan(placeOffset(xPos, bgW, fgW)+xOff, bgW-fgW), 0)
	y := max(clampSpan(placeOffset(yPos, bgH, fgH)+yOff, bgH-fgH), 0)
	for i, line := range fgLines {
		if y+i >= bgH {
			break
		}
		line = dispClip(line, bgW-x)
		row := bgLines[y+i]
		left := dispClip(row, x)
		left += strings.Repeat(" ", x-dispWidth(left)) // a wide icon cut at x, or a short row
		right := dispCutLeft(row, x+dispWidth(line))
		bgLines[y+i] = left + line + right
	}
	return strings.Join(bgLines, "\n")
}

// centerDisp centres s in a w × h area by display width — lipgloss.Place(w, h,
// Center, Center, s), which measures an icon as one cell: the smaller half of
// the gap goes left and on top. In a direction s already fills, it is left as
// it is; h 0 centres across only.
func centerDisp(w, h int, s string) string {
	lines := strings.Split(s, "\n")
	width := blockWidth(lines)
	if w > width {
		for i, l := range lines {
			gap := w - dispWidth(l)
			lines[i] = strings.Repeat(" ", gap/2) + l + strings.Repeat(" ", gap-gap/2)
		}
		width = w
	}
	if gap := h - len(lines); gap > 0 {
		blank := strings.Repeat(" ", width)
		out := make([]string, 0, h)
		for range gap / 2 {
			out = append(out, blank)
		}
		out = append(out, lines...)
		for len(out) < h {
			out = append(out, blank)
		}
		lines = out
	}
	return strings.Join(lines, "\n")
}

// blockWidth is the display width of the widest line.
func blockWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, dispWidth(l))
	}
	return w
}

// placeOffset is where a span of size fg starts in one of size bg.
func placeOffset(p overlay.Position, bg, fg int) int {
	switch p {
	case overlay.Center:
		return bg/2 - fg/2
	case overlay.Right, overlay.Bottom:
		return bg - fg
	}
	return 0
}

// clampSpan keeps v between 0 and hi (either way round, as overlay does).
func clampSpan(v, hi int) int {
	lo := 0
	if lo > hi {
		lo, hi = hi, lo
	}
	return min(max(v, lo), hi)
}

// joinH lays multi-line blocks side by side. Each block's lines are padded to
// that block's own display width, so a wide icon in one column never shoves the
// next column left. Replaces lipgloss.JoinHorizontal, whose width maths is
// icon-blind.
func joinH(blocks ...string) string {
	rows := make([][]string, len(blocks))
	widths := make([]int, len(blocks))
	maxRows := 0
	for i, b := range blocks {
		rows[i] = strings.Split(b, "\n")
		for _, ln := range rows[i] {
			if wd := dispWidth(ln); wd > widths[i] {
				widths[i] = wd
			}
		}
		if len(rows[i]) > maxRows {
			maxRows = len(rows[i])
		}
	}
	var out strings.Builder
	for r := 0; r < maxRows; r++ {
		for i := range rows {
			if r < len(rows[i]) {
				out.WriteString(padDisp(rows[i][r], widths[i]))
			} else {
				out.WriteString(strings.Repeat(" ", widths[i]))
			}
		}
		if r < maxRows-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// joinV stacks blocks, left-aligned, every line padded to the widest display
// width. Replaces lipgloss.JoinVertical (icon-blind).
func joinV(blocks ...string) string {
	var lines []string
	maxW := 0
	for _, b := range blocks {
		for _, ln := range strings.Split(b, "\n") {
			if wd := dispWidth(ln); wd > maxW {
				maxW = wd
			}
			lines = append(lines, ln)
		}
	}
	for i := range lines {
		lines[i] = padDisp(lines[i], maxW)
	}
	return strings.Join(lines, "\n")
}

// cellsBefore is where rune i of a line starts, in the line's own cells — the
// positions ansi.Cut takes (the YAML viewer keeps its cursor and selection
// as rune indexes). A CJK character is one rune but two cells, so a rune index
// handed to ansi.Cut as is lands on the wrong character (tdp L4). Measured as
// ansi.Cut measures, not dispWidth: it cuts the string's own cells.
func cellsBefore(pr []rune, i int) int { return ansi.StringWidth(string(pr[:i])) }
