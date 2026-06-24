package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	w := lipgloss.Width(lines[0])
	px := (screenW - w) / 2
	py := (screenH - h) / 2
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
	w := lipgloss.Width(lines[0])
	// Centered overlay positioning (mirrors overlay.Composite with
	// Center/Center anchors).
	px := (screenW - w) / 2
	py := (screenH - h) / 2
	if msg.X < px || msg.X >= px+w || msg.Y < py || msg.Y >= py+h {
		return -1
	}
	contentY := msg.Y - py - itemsStartLine
	if contentY < 0 || contentY >= numItems {
		return -1
	}
	return contentY
}
