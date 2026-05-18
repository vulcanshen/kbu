package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestNamespacePicker() NamespacePickerModel {
	t := theme.DefaultTheme()
	return NewNamespacePickerModel(t)
}

var testNamespaces = []string{"default", "kube-system", "monitoring"}

// openNamespacePicker opens the picker with testNamespaces and finalizes animation.
func openNamespacePicker(m *NamespacePickerModel) {
	m.Open(testNamespaces)
	m.animator.Finalize()
}

// ── Initial state ──────────────────────────────────────────────────────────

func TestNamespacePickerModel_InitialState(t *testing.T) {
	m := newTestNamespacePicker()

	if m.IsActive() {
		t.Error("namespace picker should be inactive initially")
	}
}

// ── Open ───────────────────────────────────────────────────────────────────

func TestNamespacePickerModel_Open_Activates(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	if !m.IsActive() {
		t.Error("Open() must activate the picker")
	}
	if !m.IsInteractive() {
		t.Error("picker must be interactive after animation")
	}
}

func TestNamespacePickerModel_Open_PrependsAllNamespaces(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	if len(m.namespaces) != len(testNamespaces)+1 {
		t.Fatalf("expected %d namespaces (with 'All Namespaces'), got %d",
			len(testNamespaces)+1, len(m.namespaces))
	}
	if m.namespaces[0] != "All Namespaces" {
		t.Errorf("first entry must be 'All Namespaces', got %q", m.namespaces[0])
	}
}

func TestNamespacePickerModel_Open_ResetsCursor(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))

	// Re-open.
	m.Open(testNamespaces)
	m.animator.Finalize()

	if m.cursor != 0 {
		t.Errorf("re-opening must reset cursor to 0, got %d", m.cursor)
	}
}

// ── Navigation ─────────────────────────────────────────────────────────────

func TestNamespacePickerModel_NavigateDown(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}
}

func TestNamespacePickerModel_NavigateUp(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after j,k, got %d", m.cursor)
	}
}

func TestNamespacePickerModel_NavigateUp_ClampAtZero(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('k'))
	if m.cursor != 0 {
		t.Errorf("k at top must stay at 0, got %d", m.cursor)
	}
}

func TestNamespacePickerModel_NavigateDown_ClampAtEnd(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	for i := 0; i < 20; i++ {
		m, _ = m.Update(keyMsg('j'))
	}
	if m.cursor != len(m.namespaces)-1 {
		t.Errorf("cursor must clamp at last item (%d), got %d",
			len(m.namespaces)-1, m.cursor)
	}
}

// ── Select ─────────────────────────────────────────────────────────────────

func TestNamespacePickerModel_SelectAllNamespaces_SendsEmptyNS(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)
	// cursor=0 = "All Namespaces"

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must return a cmd")
	}
	// Execute the batch and look for NamespaceChangedMsg.
	msg := runBatchForMsg[NamespaceChangedMsg](cmd)
	if msg == nil {
		t.Fatal("Enter must emit NamespaceChangedMsg")
	}
	if msg.Namespace != "" {
		t.Errorf("selecting 'All Namespaces' must send empty namespace, got %q", msg.Namespace)
	}
}

func TestNamespacePickerModel_SelectSpecific_SendsNamespace(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('j')) // cursor = 1 = "default"
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msg := runBatchForMsg[NamespaceChangedMsg](cmd)
	if msg == nil {
		t.Fatal("Enter must emit NamespaceChangedMsg")
	}
	if msg.Namespace != "default" {
		t.Errorf("expected namespace %q, got %q", "default", msg.Namespace)
	}
}

// ── Close ─────────────────────────────────────────────────────────────────

func TestNamespacePickerModel_CloseOnEsc(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("Esc must close the namespace picker")
	}
}

func TestNamespacePickerModel_CloseOnN(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('n'))
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("n must close the namespace picker")
	}
}

func TestNamespacePickerModel_InactiveIgnoresKeys(t *testing.T) {
	m := newTestNamespacePicker()
	// Do NOT call Open.

	m, cmd := m.Update(keyMsg('j'))
	if m.cursor != 0 || cmd != nil {
		t.Error("inactive picker must ignore key messages")
	}
}

// ── Helper ────────────────────────────────────────────────────────────────

// runBatchForMsg executes cmds from a tea.Batch until it finds one that
// returns a message of type T, or returns nil if none is found.
// Only looks one level deep (does not recurse into nested batches).
func runBatchForMsg[T any](cmd tea.Cmd) *T {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch m := msg.(type) {
	case T:
		return &m
	case tea.BatchMsg:
		for _, c := range m {
			if result := runBatchForMsg[T](c); result != nil {
				return result
			}
		}
	}
	return nil
}
