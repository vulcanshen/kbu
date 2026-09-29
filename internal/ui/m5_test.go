package ui

import (
	"regexp"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// plusModifier is a modifier written with "+" (Alt+t, Ctrl+C, Shift+Tab).
var plusModifier = regexp.MustCompile(`\b(Alt|Ctrl|Shift)\+`)

// tdp M5: one hotkey notation on every surface — the menus and the
// statusbar write [Alt-t]erm and [Alt-S]ort, so the key references and
// the PTY hint write Alt-t, Alt-S, Ctrl-C, Shift-Tab too.
func TestM5_OneModifierNotation(t *testing.T) {
	var shown []string
	m := stackTestApp(t)
	m.currentResource = k8s.ResourcePods // panel 2 has the Alt-S sort row
	for _, p := range []Panel{SidebarPanel, TablePanel, DetailPanel} {
		m.activePanel = p
		_, rows := m.keyRef()
		for _, r := range rows {
			shown = append(shown, r.key, r.desc)
		}
	}
	for _, rows := range [][]helpRow{yamlVisualRows(), yamlRows(true, true), menuRows(globalActions)} {
		for _, r := range rows {
			shown = append(shown, r.key, r.desc)
		}
	}
	_, drag := dragKeyRef()
	for _, r := range drag {
		shown = append(shown, r.key, r.desc)
	}
	for _, kind := range []PtyKind{PtyKindShell, PtyKindEdit} {
		shown = append(shown, ansi.Strip(hookedPtyView(kind).RenderPopup()))
	}
	if len(shown) < 40 {
		t.Fatalf("only %d strings collected", len(shown))
	}
	for _, s := range shown {
		if plusModifier.MatchString(s) {
			t.Errorf("%q writes a modifier with +; the menus write it with -", s)
		}
	}
}
