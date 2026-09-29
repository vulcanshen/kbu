package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// popupRowAt is the common hit-test for centered popups. Given a
// fully-rendered popup string + screen size + the popup's items-area
// layout (where item rows START inside the popup, counted from the
// popup's own top line; and how many such rows exist), returns the
// 0-indexed row under the mouse click, or -1 if the click is outside
// the popup OR lands on a border / padding / hint line rather than a
// row.
//
// Layout assumed by `itemsStartLine` (counting from the popup's top
// line as 0):
//
//	0:                        ╭─ title ──╮
//	1:                        │ padding  │
//	itemsStartLine .. +N-1:   │ items    │   ← these are the rows
//	itemsStartLine+N:         │ padding  │
//	h-1:                      ╰─ hint  ──╯
//
// itemsStartLine = 2 for the standard panel2menu / listpicker /
// settings render shape; other popups override when they prepend
// extra header rows (search box, separator, ...).
// popupContains tests whether the mouse click landed anywhere
// inside the given centered popup's rendered bounds. Used by
// scroll-only popups (yamlpopup, comparepopup, help, appLog,
// confirm) where no specific row matters — the only mouse gesture
// is "right-click inside to close".
func popupContains(popup string, msg tea.MouseMsg, screenW, screenH int) bool {
	if popup == "" {
		return false
	}
	lines := strings.Split(popup, "\n")
	h := len(lines)
	if h == 0 {
		return false
	}
	w := dispWidth(lines[0])
	px, py := popupOrigin(w, h, screenW, screenH)
	return msg.X >= px && msg.X < px+w && msg.Y >= py && msg.Y < py+h
}

func popupRowAt(popup string, msg tea.MouseMsg, screenW, screenH, itemsStartLine, numItems int) int {
	if popup == "" || numItems <= 0 {
		return -1
	}
	lines := strings.Split(popup, "\n")
	h := len(lines)
	if h == 0 {
		return -1
	}
	w := dispWidth(lines[0])
	px, py := popupOrigin(w, h, screenW, screenH)
	if msg.X < px || msg.X >= px+w || msg.Y < py || msg.Y >= py+h {
		return -1
	}
	contentY := msg.Y - py - itemsStartLine
	if contentY < 0 || contentY >= numItems {
		return -1
	}
	return contentY
}

// popupOrigin is where a w×h popup's top-left corner lands on a screenW ×
// screenH screen when composited centered — the same arithmetic
// compositeDisp uses (half the screen minus half the popup, each
// halved on its own), so a click hits the row that is drawn there. The
// shorter (screen − popup) / 2 is off by one when the two sizes differ in
// parity.
func popupOrigin(w, h, screenW, screenH int) (x, y int) {
	x = screenW/2 - w/2
	y = screenH/2 - h/2
	return max(x, 0), max(y, 0)
}

// popupOuterWidth is every popup's width, borders included (tdp F7): the
// terminal width less a column each side, at most 120 — the same for
// every popup, so a popup's size never depends on what it shows. The
// PTY terminals are the one exception (they fill the screen).
func popupOuterWidth(screenW int) int {
	if screenW <= 0 {
		screenW = 80
	}
	return max(min(screenW-2, 120), 24)
}

// popupInnerWidth is the width inside a popup's two side borders.
func popupInnerWidth(screenW int) int { return popupOuterWidth(screenW) - 2 }
