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

// openNamespacePicker opens the picker via the OpenLoading + SetNamespaces
// async flow, then finalizes the animation so the picker becomes
// interactive. Matches what app.go does when the user presses N:
// open the popup immediately, swap the placeholder for the real
// list when fetchNamespaces returns.
func openNamespacePicker(m *NamespacePickerModel) {
	m.OpenLoading()
	m.SetNamespaces(testNamespaces)
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

	// Re-open via the async flow.
	m.OpenLoading()
	m.SetNamespaces(testNamespaces)
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

func TestNamespacePickerModel_CloseOnUppercaseN(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('N'))
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("N must also close the namespace picker (alias)")
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

// ── Search ────────────────────────────────────────────────────────────────

func TestNamespacePickerModel_SlashEntersSearchMode(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	if !m.searching {
		t.Error("'/' must enter search mode")
	}
}

func TestNamespacePickerModel_SearchFiltersList(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("mon")})

	items := m.filtered()
	if len(items) != 1 || items[0] != "monitoring" {
		t.Errorf("expected only 'monitoring', got %v", items)
	}
}

func TestNamespacePickerModel_EnterInSearchReleasesFocusOnly(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("default")})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.searching {
		t.Error("Enter in search mode must release search focus")
	}
	if m.searchQuery != "default" {
		t.Errorf("Enter must keep filter, got query %q", m.searchQuery)
	}
	if !m.IsActive() {
		t.Error("Enter in search mode must not close popup")
	}
	if cmd != nil {
		if msg := runBatchForMsg[NamespaceChangedMsg](cmd); msg != nil {
			t.Error("Enter in search mode must NOT emit NamespaceChangedMsg")
		}
	}
}

func TestNamespacePickerModel_JKInSearchModeAreTyped(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('k'))

	if m.cursor != 0 {
		t.Errorf("j/k in search must not move cursor, got %d", m.cursor)
	}
	if m.searchQuery != "jk" {
		t.Errorf("j/k in search must be typed, got query %q", m.searchQuery)
	}
}

func TestNamespacePickerModel_ArrowsInSearchModeNavigate(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	if m.cursor != 1 {
		t.Errorf("Down arrow in search must move cursor, got %d", m.cursor)
	}
}

func TestNamespacePickerModel_JNavigatesAfterEnterReleasesFocus(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}) // matches All Namespaces & default
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})                     // exit search

	if m.searching {
		t.Fatal("expected search released after Enter")
	}
	m, _ = m.Update(keyMsg('j'))
	if m.cursor != 1 {
		t.Errorf("j after Enter must navigate filtered list, got cursor %d", m.cursor)
	}
}

func TestNamespacePickerModel_EscInSearchClearsFilter(t *testing.T) {
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("xyz")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if m.searching {
		t.Error("Esc in search must exit search mode")
	}
	if m.searchQuery != "" {
		t.Errorf("Esc must clear query, got %q", m.searchQuery)
	}
	if !m.IsActive() {
		t.Error("Esc in search must NOT close popup")
	}
}

func TestNamespacePickerModel_NSearchableInSearchMode(t *testing.T) {
	// In search mode, 'n' should be a typed character, not close popup.
	m := newTestNamespacePicker()
	openNamespacePicker(&m)

	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(keyMsg('n'))

	if !m.IsActive() {
		t.Error("'n' in search mode must not close popup")
	}
	if m.searchQuery != "n" {
		t.Errorf("'n' in search must be typed; got query %q", m.searchQuery)
	}
}

// ── Async loading flow ─────────────────────────────────────────────────────

func TestNamespacePickerModel_OpenLoading_ActivatesInLoadingState(t *testing.T) {
	// OpenLoading must make the popup IsActive immediately so the user
	// gets visual feedback before fetchNamespaces returns.
	m := newTestNamespacePicker()
	m.OpenLoading()
	m.animator.Finalize()

	if !m.IsActive() {
		t.Error("OpenLoading must activate the picker")
	}
	if !m.loading {
		t.Error("OpenLoading must set loading=true")
	}
	if len(m.namespaces) != 0 {
		t.Errorf("OpenLoading must not seed any namespaces, got %v", m.namespaces)
	}
}

func TestNamespacePickerModel_LoadingState_NavKeysAreNoOp(t *testing.T) {
	// In loading state, j/k/Enter/search must be ignored so the user
	// can't act on the empty placeholder. Only the close set responds.
	m := newTestNamespacePicker()
	m.OpenLoading()
	m.animator.Finalize()

	for _, k := range []rune{'j', 'k', '/'} {
		m, _ = m.Update(keyMsg(k))
	}
	if m.cursor != 0 {
		t.Errorf("loading state must ignore j/k, cursor moved to %d", m.cursor)
	}
	if m.searching {
		t.Error("loading state must ignore /, but search mode turned on")
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		if msg := runBatchForMsg[NamespaceChangedMsg](cmd); msg != nil {
			t.Error("Enter in loading state must NOT emit NamespaceChangedMsg")
		}
	}
}

func TestNamespacePickerModel_LoadingState_EscCloses(t *testing.T) {
	// Esc on a loading popup must dismiss it — otherwise a failed
	// fetch leaves the user stuck (paired with the Err-handling
	// path in app.go's NamespaceListMsg handler).
	m := newTestNamespacePicker()
	m.OpenLoading()
	m.animator.Finalize()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("Esc must close a loading popup")
	}
}

func TestNamespacePickerModel_SetNamespaces_SwapsListInPlace(t *testing.T) {
	// SetNamespaces lands while the popup is open in loading state →
	// loading flips off, list is prepended with "All Namespaces",
	// cursor resets to 0, animator stays open (no re-animation, no
	// flicker).
	m := newTestNamespacePicker()
	m.OpenLoading()
	m.animator.Finalize()
	stateBefore := m.animator.State

	m.SetNamespaces([]string{"default", "kube-system"})

	if m.loading {
		t.Error("SetNamespaces must clear the loading flag")
	}
	if m.animator.State != stateBefore {
		t.Errorf("animator state changed %v → %v, want stable", stateBefore, m.animator.State)
	}
	if len(m.namespaces) != 3 || m.namespaces[0] != "All Namespaces" {
		t.Errorf("SetNamespaces should prepend 'All Namespaces', got %v", m.namespaces)
	}
	if m.cursor != 0 {
		t.Errorf("SetNamespaces must reset cursor to 0, got %d", m.cursor)
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
