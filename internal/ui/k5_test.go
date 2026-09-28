package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vulcanshen/kbu/internal/k8s"
)

var spaceKey = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}

// tdp K5: Space only opens and closes the Space menu it opened. On a
// popup opened by Enter or a hotkey (confirm, pickers, viewers, the key
// reference) it does nothing — closing a confirm on Space would make it
// a second Esc (F6). Each case opens one popup, presses Space through
// the app, and checks the popup is still up.
func TestK5_SpaceDoesNotClosePopupsThatAreNotTheSpaceMenu(t *testing.T) {
	cases := []struct {
		name string
		open func(m *AppModel)
		up   func(m *AppModel) bool
	}{
		{"confirm", func(m *AppModel) {
			_ = m.confirm.Show(ConfirmDelete, "⚠ Delete resource?", "kubectl delete pods nginx", func() tea.Msg { return confirmedMsg{} })
			m.confirm.animator.Finalize()
		}, func(m *AppModel) bool { return m.confirm.owns() }},
		{"yaml", func(m *AppModel) {
			_ = m.yamlPopup.Open("kind: Pod\n", k8s.ResourcePods, k8s.ResourceItem{Name: "nginx"})
			m.yamlPopup.animator.Finalize()
		}, func(m *AppModel) bool { return m.yamlPopup.owns() }},
		{"breadcrumb", func(m *AppModel) {
			_ = m.breadcrumbPopup.Open([]k8s.RefTarget{
				{Type: k8s.ResourcePods, Name: "a", Namespace: "ns"},
				{Type: k8s.ResourceDeployments, Name: "b", Namespace: "ns"},
			})
			m.breadcrumbPopup.animator.Finalize()
		}, func(m *AppModel) bool { return m.breadcrumbPopup.owns() }},
		{"namespace picker, loading", func(m *AppModel) {
			_ = m.namespacePicker.OpenLoading()
			m.namespacePicker.animator.Finalize()
		}, func(m *AppModel) bool { return m.namespacePicker.owns() }},
		{"namespace picker, loaded", func(m *AppModel) {
			_ = m.namespacePicker.OpenLoading()
			m.namespacePicker.SetNamespaces([]string{"default", "kube-system"})
			m.namespacePicker.animator.Finalize()
		}, func(m *AppModel) bool { return m.namespacePicker.owns() }},
		{"context picker", func(m *AppModel) {
			_ = m.contextPicker.Open([]string{"dev", "prod"}, "dev")
			m.contextPicker.animator.Finalize()
		}, func(m *AppModel) bool { return m.contextPicker.owns() }},
		{"sort picker", func(m *AppModel) {
			_ = m.listPicker.Open("sort:column", "Sort Pods by…", []ListPickerItem{{Key: "Name", Label: "Name"}})
			m.listPicker.animator.Finalize()
		}, func(m *AppModel) bool { return m.listPicker.owns() }},
		{"settings", func(m *AppModel) {
			_ = m.settingsPopup.Open(m.buildSettingsItems())
			m.settingsPopup.animator.Finalize()
		}, func(m *AppModel) bool { return m.settingsPopup.owns() }},
		{"app log", func(m *AppModel) {
			_ = m.appLog.Toggle()
			m.appLog.animator.Finalize()
		}, func(m *AppModel) bool { return m.appLog.owns() }},
		{"key reference", func(m *AppModel) {
			_ = m.openKeyRef()
			m.help.animator.Finalize()
		}, func(m *AppModel) bool { return m.help.owns() }},
		{"compare", func(m *AppModel) {
			_ = m.comparePopup.Open("a: 1\n", "a: 2\n", "l", "r")
			m.comparePopup.animator.Finalize()
		}, func(m *AppModel) bool { return m.comparePopup.owns() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := stackTestApp(t)
			c.open(&m)
			if !c.up(&m) {
				t.Fatal("setup: popup must be open")
			}
			updated, cmd := m.Update(spaceKey)
			got := updated.(AppModel)
			if !c.up(&got) {
				t.Errorf("Space closed the %s", c.name)
			}
			for _, msg := range drainCmd(cmd) {
				if _, ok := msg.(confirmedMsg); ok {
					t.Errorf("Space accepted the %s", c.name)
				}
			}
		})
	}
}

// tdp K7: the hotkey that opened the App log (!) or Settings (>) is not a
// second Esc on it.
func TestK7_OpeningHotkeyDoesNotClose(t *testing.T) {
	m := stackTestApp(t)
	_ = m.appLog.Toggle()
	m.appLog.animator.Finalize()
	updated, _ := m.Update(key("!"))
	if got := updated.(AppModel); !got.appLog.owns() {
		t.Error("! must not close the App log")
	}

	m = stackTestApp(t)
	_ = m.settingsPopup.Open(m.buildSettingsItems())
	m.settingsPopup.animator.Finalize()
	updated, _ = m.Update(key(">"))
	if got := updated.(AppModel); !got.settingsPopup.owns() {
		t.Error("> must not close Settings")
	}
}

// tdp F6 + D3: the confirm's hint says what Enter will do, not "OK".
func TestConfirm_HintNamesTheAction(t *testing.T) {
	cases := map[ConfirmAction]string{
		ConfirmDelete:    " Enter delete · Esc cancel ",
		ConfirmEdit:      " Enter edit · Esc cancel ",
		ConfirmShellExec: " Enter exec · Esc cancel ",
		ConfirmRollback:  " Enter rollback · Esc cancel ",
		ConfirmSwitch:    " Enter switch · Esc cancel ",
	}
	for action, want := range cases {
		m := stackTestApp(t)
		_ = m.confirm.Show(action, "Do it?", "detail", nil)
		m.confirm.animator.Finalize()
		if popup := m.confirm.renderFullPopup(); !strings.Contains(popup, want) {
			t.Errorf("action %d: hint must read %q", action, want)
		}
	}
}
