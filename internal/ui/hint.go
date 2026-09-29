package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vulcanshen/kbu/internal/theme"
)

// Every hint in kbu — the popups' bottom borders, the terminals, the
// panel borders, the footer — is a list of keyHint drawn one way (tdp
// M5): key:description, no space around the colon, entries one space
// apart (j/k:move Enter:run Esc:close). The key is one colour and the
// colon and description another (D2), so the entries read apart
// without wider gaps.

// keyHint is one entry of a hint: a key and what it does.
type keyHint struct {
	key  string
	desc string
}

// hintText is a hint line uncoloured: what its width is measured on.
func hintText(hs []keyHint) string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = h.key + ":" + h.desc
	}
	return strings.Join(parts, " ")
}

// hintColours is the pair a hint is drawn in: the key, then the colon
// and description.
type hintColours struct {
	key  lipgloss.Style
	desc lipgloss.Style
}

// brightHint is the family pair (D2): the key Blue, bold like the key
// reference's keys; the colon and description Overlay0.
func brightHint() hintColours {
	return hintColours{
		key:  lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Blue)).Bold(true),
		desc: lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Overlay0)),
	}
}

// recededHint is the darker pair on an unfocused panel's border: the key
// Overlay0, the colon and description Surface2 — the border's own colour
// — so the hint recedes with the panel and stays two colours.
func recededHint() hintColours {
	return hintColours{
		key:  lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Overlay0)).Bold(true),
		desc: lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Surface2)),
	}
}

// renderHints draws hs in c.
func renderHints(hs []keyHint, c hintColours) string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = c.key.Render(h.key) + c.desc.Render(":"+h.desc)
	}
	return strings.Join(parts, " ")
}

// popupHint is a popup's bottom-border hint, a space either side.
func popupHint(hs ...keyHint) string {
	if len(hs) == 0 {
		return ""
	}
	return " " + renderHints(hs, brightHint()) + " "
}

// fitPopupHint is a popup's bottom-border hint fitted to room cells, its
// spaces included: entries that don't fit drop whole from the end, never
// half of one (tdp D3). "" when not even the first fits.
func fitPopupHint(room int, hs ...keyHint) string {
	return popupHint(fitHints(hs, room-2)...)
}

// fitHints drops entries from the end until the line fits in w cells —
// a whole entry at a time, never half of one (tdp D1).
func fitHints(hs []keyHint, w int) []keyHint {
	for len(hs) > 0 && dispWidth(hintText(hs)) > w {
		hs = hs[:len(hs)-1]
	}
	return hs
}
