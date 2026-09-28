package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/vulcanshen/kbu/internal/k8s"
)

// yamlOpenFor is the app with the YAML viewer open on item.
func yamlOpenFor(t *testing.T, rt k8s.ResourceType, item k8s.ResourceItem) AppModel {
	t.Helper()
	m := stackTestApp(t)
	m.yamlPopup.SetSize(m.width, m.height)
	_ = m.yamlPopup.Open("kind: Pod\nmetadata:\n  name: "+item.Name+"\n", rt, item)
	m.yamlPopup.animator.Finalize()
	return m
}

// pressE sends E and feeds whatever it asks for back into the app.
func pressE(t *testing.T, m AppModel) AppModel {
	t.Helper()
	updated, cmd := m.Update(key("E"))
	m = updated.(AppModel)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(AppModel)
		}
	}
	m.confirm.animator.Finalize()
	return m
}

// tdp F6: an action that confirms confirms every time. E in the YAML
// viewer used to start kubectl edit at once, where E on the panel asks;
// now both ask, and the confirm stacks on the viewer (F4): Esc on it
// lands back in the YAML.
func TestF6_YamlEditConfirmsLikeThePanel(t *testing.T) {
	m := yamlOpenFor(t, k8s.ResourcePods, k8s.ResourceItem{Name: "nginx", Namespace: "default"})
	m = pressE(t, m)
	if !m.confirm.animator.Owns() || m.confirm.action != ConfirmEdit {
		t.Fatal("E in the YAML viewer must open the edit confirm")
	}
	if !strings.Contains(m.confirm.detail, "kubectl edit pod/nginx -n default") {
		t.Errorf("confirm detail %q, want the kubectl edit command", m.confirm.detail)
	}
	if !m.yamlPopup.animator.Owns() {
		t.Fatal("the YAML viewer must stay under the confirm")
	}
	if m.txPty != nil && m.txPty.IsActive() {
		t.Fatal("kubectl edit started before the confirm was answered")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(AppModel)
	if m.topLayer() != &m.yamlPopup {
		t.Errorf("Esc on the confirm should return to the YAML viewer, top is %T", m.topLayer())
	}
}

// Rule A (dev-remarks): a helm-managed object is read-only in kbu — its
// Edit row is dimmed. E in its YAML viewer does nothing either, and the
// hint and ? don't offer it.
func TestF6_YamlEditOffForHelmManaged(t *testing.T) {
	item := podItem("web", map[string]string{"app.kubernetes.io/managed-by": "Helm"})
	if !k8s.IsHelmManaged(item) {
		t.Fatal("fixture is not helm-managed")
	}
	m := yamlOpenFor(t, k8s.ResourcePods, item)
	m = pressE(t, m)
	if m.confirm.animator.Owns() {
		t.Error("E on a helm-managed object's YAML opened a confirm")
	}
	assertNoEditOffered(t, m)
}

// A Helm release document is not a resource kubectl edit can open.
func TestF6_YamlEditOffForReleaseDocs(t *testing.T) {
	m := yamlOpenFor(t, k8s.ResourceReleases, k8s.ResourceItem{Name: "web", Namespace: "default"})
	m = pressE(t, m)
	if m.confirm.animator.Owns() {
		t.Error("E on a Helm release document opened a confirm")
	}
	assertNoEditOffered(t, m)
}

func assertNoEditOffered(t *testing.T, m AppModel) {
	t.Helper()
	lines := strings.Split(m.yamlPopup.renderFullPopup(), "\n")
	if bottom := ansi.Strip(lines[len(lines)-1]); strings.Contains(bottom, "E:edit") {
		t.Errorf("hint offers E where it does nothing: %q", bottom)
	}
	_, rows := m.keyRef()
	for _, r := range rows {
		if r.key == "E" {
			t.Errorf("? lists E where it does nothing: %q", r.desc)
		}
	}
}
