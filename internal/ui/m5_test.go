package ui

import (
	"regexp"
	"strings"
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

// keyRefRows collects the key reference of every surface kbu has.
func keyRefRows(t *testing.T) map[string][]helpRow {
	t.Helper()
	out := map[string][]helpRow{}
	m := stackTestApp(t)
	m.currentResource = k8s.ResourcePods
	m.detail.SetResourceType(k8s.ResourcePods)
	for name, p := range map[string]Panel{"panel 1": SidebarPanel, "panel 2": TablePanel, "panel 3": DetailPanel} {
		m.activePanel = p
		_, out[name] = m.keyRef()
	}
	_ = m.confirm.Show(ConfirmDelete, "Delete?", "", nil)
	m.confirm.animator.Finalize()
	_, out["confirm"] = m.keyRef()

	m = stackTestApp(t)
	m.appLog.SetSize(m.width, m.height)
	_ = m.appLog.Toggle()
	m.appLog.animator.Finalize()
	_, out["app log"] = m.keyRef()

	m = stackTestApp(t)
	_ = m.comparePopup.Open("a: 1\n", "a: 2\n", "l", "r")
	m.comparePopup.animator.Finalize()
	_, out["compare"] = m.keyRef()

	_, out["drag"] = dragKeyRef()
	out["yaml"] = yamlRows(true, true)
	out["selection"] = yamlVisualRows()
	out["picker"] = pickerRows("pick", "close")
	out["filter picker"] = filterPickerRows("pick", false)
	out["menu"] = menuMoveRows("run", "close", true)
	return out
}

// tdp M5 in the key reference: keys that do the same thing share a row,
// joined with / (j/k, gg/G, Enter/y), a range with – (1–3); a key with a
// description of its own gets its own row (Tab, Shift-Tab). No key cell
// is a space-separated list.
func TestM5_KeyReferenceJoinsKeysWithSlash(t *testing.T) {
	all := keyRefRows(t)
	for surface, rows := range all {
		for _, r := range rows {
			if !r.header && strings.Contains(r.key, " ") {
				t.Errorf("%s: key cell %q lists keys with spaces", surface, r.key)
			}
		}
	}
	want := map[string][]string{
		"panel 1":       {"j/k", "u/d", "gg/G", "Tab", "Shift-Tab", "1–3", "Ctrl-C"},
		"panel 3":       {"h/[", "l/]"},
		"confirm":       {"Enter/y", "Esc/n"},
		"app log":       {"j/k", "u/d", "g/G"},
		"compare":       {"j/k", "u/d", "gg/G"},
		"drag":          {"Enter/D", "q/Ctrl-C"},
		"yaml":          {"h/j/k/l", "w/b/e", "0/$"},
		"selection":     {"v/Esc", "q/Ctrl-C"},
		"filter picker": {"↑/↓"},
	}
	for surface, keys := range want {
		got := refKeys(all[surface])
		for _, k := range keys {
			if !contains(got, k) {
				t.Errorf("%s: no %q row, got %v", surface, k, got)
			}
		}
	}
}
