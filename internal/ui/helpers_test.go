package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// key builds the KeyMsg for a key name ("enter", "esc", " ", or runes).
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// expectMsg runs cmd and passes when match accepts its message or any
// message inside a tea.Batch. A nil cmd passes (callers that need a cmd
// check that first).
func expectMsg(t *testing.T, cmd tea.Cmd, match func(tea.Msg) bool) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if match(msg) {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if match(c()) {
				return
			}
		}
	}
}

// itemKeys lists the hotkeys of a menu's selectable rows, chrome skipped.
func itemKeys(items []menuItem) []string {
	var out []string
	for _, it := range items {
		if !it.selectable() {
			continue
		}
		out = append(out, it.key)
	}
	return out
}

func contains(s []string, want string) bool {
	for _, x := range s {
		if x == want {
			return true
		}
	}
	return false
}

// openPodMenu opens menu as the Space menu of a panel 2 row, the way the
// app builds it, and settles its open animation.
func openPodMenu(menu *MenuPopupModel, rt k8s.ResourceType, item k8s.ResourceItem, compare panel2CompareCtx) {
	items := groupedMenu(panel2ItemOps(rt, item, k8s.IsHelmManaged(item), compare), nil)
	_ = menu.Open(menuTitle(menuTitleGlyph, "[2] "+rt.KubectlName()+"/"+item.Name), items, rt, item)
	menu.animator.Finalize()
}

// menuRow finds the row with the given action, or fails the test.
func menuRow(t *testing.T, items []menuItem, action string) menuItem {
	t.Helper()
	for _, it := range items {
		if it.selectable() && it.actionName() == action {
			return it
		}
	}
	t.Fatalf("no row with action %q in %v", action, labels(items))
	return menuItem{}
}

func labels(items []menuItem) []string {
	var out []string
	for _, it := range items {
		switch {
		case it.separator:
			out = append(out, "──")
		case it.header:
			out = append(out, "#"+it.label)
		default:
			out = append(out, bracketHotkey(it.label, it.key))
		}
	}
	return out
}
