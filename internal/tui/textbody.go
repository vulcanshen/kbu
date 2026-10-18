package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// textBody is a panel body of lines with no cursor (components/layout/panel:
// a log, a table of events): content laid out for the panel's width, a
// header that does not scroll, and — when it follows — the view sticking to
// the end while new lines come in, until the user scrolls up.
//
// render lays the content out for a width: the header rows, then the rows.
// It runs again only when the width or the content changes (version).
type textBody struct {
	render func(w int) (header, rows []string)
	// empty and hint are what it says with nothing to show: a fact and a
	// hint naming a key (components/layout/screen, empty states).
	empty, hint string
	follow      bool   // it follows the end when it was at the end
	live        bool   // following now: at the end, not scrolled away
	onLive      func() // runs when live changes (the tab's live / paused glyph)

	version int // bumped by changed()
	cacheW  int
	cacheV  int
	header  []string
	rows    []string
	top     int
	height  int // rows of content the last Draw had room for
	scroll  string
}

func newTextBody(render func(w int) (header, rows []string), empty string, follow bool) *textBody {
	return &textBody{render: render, empty: empty, hint: "Press [Space] to see what you can do here",
		follow: follow, live: follow, cacheW: -1}
}

// changed says the content changed: it is laid out again at the next Draw.
func (b *textBody) changed() { b.version++ }

// reset goes back to the top (or, following, to the end) for new content.
func (b *textBody) reset() {
	b.top = 0
	b.setLive(b.follow)
	b.changed()
}

func (b *textBody) setLive(v bool) {
	if b.live == v {
		return
	}
	b.live = v
	if b.onLive != nil {
		b.onLive()
	}
}

func (b *textBody) layout(w int) {
	if w == b.cacheW && b.version == b.cacheV {
		return
	}
	b.header, b.rows = b.render(w)
	b.cacheW, b.cacheV = w, b.version
}

// Draw draws the header, then the rows seen from top; following, the last
// rows.
func (b *textBody) Draw(w, h int, focused bool) []string {
	b.layout(max(w-1, 1))
	if len(b.rows) == 0 {
		b.scroll = ""
		out := make([]string, 0, h)
		for _, l := range b.header {
			if len(out) < h {
				out = append(out, " "+terminu.Pad(l, w-1))
			}
		}
		return append(out, terminu.EmptyState(w, h-len(out), b.empty, b.hint)...)
	}
	out := make([]string, 0, h)
	for _, l := range b.header {
		if len(out) < h {
			out = append(out, " "+terminu.Pad(l, w-1))
		}
	}
	rows := h - len(out)
	b.height = rows
	maxTop := max(len(b.rows)-rows, 0)
	if b.live {
		b.top = maxTop
	}
	b.top = min(max(b.top, 0), maxTop)
	for i := 0; i < rows; i++ {
		if r := b.top + i; r < len(b.rows) {
			out = append(out, " "+terminu.Pad(b.rows[r], w-1))
		} else {
			out = append(out, terminu.Repeat(" ", w))
		}
	}
	b.scroll = ""
	if len(b.rows) > rows && rows > 0 {
		last := min(b.top+rows, len(b.rows))
		b.scroll = fmt.Sprintf("%d-%d of %d", b.top+1, last, len(b.rows))
	}
	return out
}

// Key scrolls: a line, half a page, the top, the end. Scrolling up leaves the
// end; G goes back to it and, following, follows again.
func (b *textBody) Key(key string, _ tea.KeyMsg) (bool, tea.Cmd) {
	half := max(b.height/2, 1)
	maxTop := max(len(b.rows)-b.height, 0)
	switch key {
	case "j", "↓":
		b.top = min(b.top+1, maxTop)
	case "k", "↑":
		b.top = max(b.top-1, 0)
	case "d", "PgDn":
		b.top = min(b.top+half, maxTop)
	case "u", "PgUp":
		b.top = max(b.top-half, 0)
	case "gg":
		b.top = 0
	case "G":
		b.top = maxTop
	default:
		return false, nil
	}
	b.setLive(b.follow && b.top >= maxTop)
	return true, nil
}

// HasItem: a text has no items; Enter works on the whole panel (Rules K3).
func (b *textBody) HasItem() bool { return false }

// Scroll is "N-M of T" when the rows do not all fit (layout/panel).
func (b *textBody) Scroll() string { return b.scroll }

// MoveKeys are the scrolling keys.
func (b *textBody) MoveKeys() []terminu.KeyRow {
	end := "to the top, the end"
	if b.follow {
		end = "to the top, the end and follow it"
	}
	return []terminu.KeyRow{
		{Key: "j|k", Desc: "scroll a line"},
		{Key: "u|d", Desc: "half a page up, down"},
		{Key: "gg|G", Desc: end},
	}
}

// text is everything in it, plain, for copying.
func (b *textBody) text() string {
	b.layout(max(b.cacheW, 80))
	var s string
	for _, l := range append(append([]string(nil), b.header...), b.rows...) {
		s += terminu.StripStyles(l) + "\n"
	}
	return s
}
