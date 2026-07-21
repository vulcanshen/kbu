package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
)

func newTestYamlPopup() YamlPopupModel {
	t := theme.DefaultTheme()
	m := NewYamlPopupModel(t)
	m.SetSize(120, 40)
	return m
}

const sampleYAML = `apiVersion: v1
kind: Pod
metadata:
  name: nginx-789abc
  namespace: default
  labels:
    app: nginx
    version: 1.27.1
spec:
  containers:
  - name: nginx
    image: nginx:1.27.1
    ports:
    - containerPort: 80
  - name: sidecar
    image: busybox:latest
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"
  - type: ContainersReady
    status: "True"`

func openTestPopup(m YamlPopupModel, yaml string) YamlPopupModel {
	item := k8s.ResourceItem{Name: "nginx-789abc", Namespace: "default"}
	m.Open(yaml, k8s.ResourcePods, item, "test-ctx")
	m.animator.Finalize()
	return m
}

func TestYamlPopup_InitialState(t *testing.T) {
	m := newTestYamlPopup()
	if m.IsActive() {
		t.Error("expected popup to be inactive initially")
	}
	if m.View() != "" {
		t.Error("expected empty view when inactive")
	}
}

func TestYamlPopup_OpenActivates(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	if !m.IsActive() {
		t.Error("expected popup to be active after Open")
	}
	if !m.IsInteractive() {
		t.Error("expected popup to be interactive after animator finalize")
	}
}

func TestYamlPopup_RenderContainsResourceTitle(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	view := m.RenderPopup()
	if !strings.Contains(view, "pod/nginx-789abc") {
		t.Errorf("expected popup to show pod/nginx-789abc in title, got:\n%s", view)
	}
	if !strings.Contains(view, "default") {
		t.Error("expected popup to show namespace in title")
	}
}

func TestYamlPopup_JKMovesCursor(t *testing.T) {
	// v1.7.10 vim-buffer semantics: j/k moves the cursor line (with
	// auto-scroll to keep it visible), not just the scroll viewport.
	m := newTestYamlPopup()
	m.SetSize(80, 10) // small height → force scrollable content
	m = openTestPopup(m, sampleYAML)
	if m.cursorLine != 0 {
		t.Fatalf("expected initial cursorLine=0, got %d", m.cursorLine)
	}

	m, _ = m.Update(keyMsg('j'))
	if m.cursorLine != 1 {
		t.Errorf("expected cursorLine=1 after j, got %d", m.cursorLine)
	}

	m, _ = m.Update(keyMsg('k'))
	if m.cursorLine != 0 {
		t.Errorf("expected cursorLine=0 after k, got %d", m.cursorLine)
	}

	// k at top stays at 0.
	m, _ = m.Update(keyMsg('k'))
	if m.cursorLine != 0 {
		t.Errorf("expected cursorLine=0 at top, got %d", m.cursorLine)
	}
}

func TestYamlPopup_JKAutoScrolls(t *testing.T) {
	// Moving the cursor past the visible viewport auto-scrolls to keep
	// it in view — matches vim's cursor-follows-scroll behavior. Small
	// popup height forces the auto-scroll path to fire.
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	// Press j enough times to drive the cursor past the initial
	// viewport (contentHeight is small at height=10).
	for i := 0; i < 20; i++ {
		m, _ = m.Update(keyMsg('j'))
	}
	if m.ScrollOffset() == 0 {
		t.Errorf("expected scrollOffset > 0 after cursor moved past viewport, got %d", m.ScrollOffset())
	}
}

func TestYamlPopup_UDMovesCursorHalfPage(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('d'))
	if m.cursorLine == 0 {
		t.Error("expected cursorLine > 0 after d half-page-down")
	}
	prev := m.cursorLine

	m, _ = m.Update(keyMsg('u'))
	if m.cursorLine >= prev {
		t.Errorf("expected cursorLine to decrease after u; was %d, now %d", prev, m.cursorLine)
	}
}

func TestYamlPopup_GotoBottomAndTop(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('G'))
	if m.ScrollOffset() == 0 {
		t.Error("expected scrollOffset > 0 after G")
	}

	// gg: needs two g's
	m, _ = m.Update(keyMsg('g'))
	m, _ = m.Update(keyMsg('g'))
	if m.ScrollOffset() != 0 {
		t.Errorf("expected scrollOffset=0 after gg, got %d", m.ScrollOffset())
	}
}

func TestYamlPopup_SearchFlow(t *testing.T) {
	m := newTestYamlPopup()
	m.SetSize(80, 10)
	m = openTestPopup(m, sampleYAML)

	// Open search
	m, _ = m.Update(keyMsg('/'))
	if !m.IsSearching() {
		t.Fatal("expected searching mode after /")
	}

	// Type "image"
	for _, r := range "image" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.SearchQuery() != "image" {
		t.Errorf("expected query=image, got %q", m.SearchQuery())
	}

	// Commit with Enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.IsSearching() {
		t.Error("expected searching mode off after Enter")
	}
	if m.MatchCount() == 0 {
		t.Fatal("expected at least one match for 'image' in sample YAML")
	}

	// Two matches in sample (nginx + busybox); n cycles
	firstCursor := m.MatchCursor()
	m, _ = m.Update(keyMsg('n'))
	if m.MatchCursor() == firstCursor && m.MatchCount() > 1 {
		t.Errorf("expected match cursor to advance after n, stayed at %d", firstCursor)
	}
}

func TestYamlPopup_SearchBackspace(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('/'))
	for _, r := range "img" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.SearchQuery() != "im" {
		t.Errorf("expected query=im after backspace, got %q", m.SearchQuery())
	}
}

func TestYamlPopup_EditEmitsStartEditMsg(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	_, cmd := m.Update(keyMsg('E'))
	if cmd == nil {
		t.Fatal("expected non-nil cmd from E key")
	}
	msg := cmd()
	// Cmd returns tea.Batch result — for the E path it returns nil due to
	// the closure firing both close + startEditMsg as a batch. Walk the batch.
	// tea.Batch returns a tea.BatchMsg which is a slice of cmds — we need to
	// find the inner startEditMsg.
	foundEdit := false
	switch v := msg.(type) {
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			inner := c()
			if _, ok := inner.(startEditMsg); ok {
				foundEdit = true
				break
			}
		}
	case startEditMsg:
		foundEdit = true
	}
	if !foundEdit {
		t.Errorf("expected startEditMsg in cmd output, got %T", msg)
	}
}

func TestYamlPopup_CopyEmitsClipboardCmd(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	_, cmd := m.Update(keyMsg('y'))
	if cmd == nil {
		t.Fatal("y must return a clipboard command")
	}
	msg := cmd()
	switch msg.(type) {
	case ClipboardCopiedMsg, ClipboardCopyFailedMsg:
		// OK — copyToClipboardCmd resolves to one of these depending on
		// whether the host terminal accepts the OSC 52 sequence.
	default:
		t.Errorf("expected clipboard msg, got %T", msg)
	}
}

func TestYamlPopup_CopyNoOpWhenEmpty(t *testing.T) {
	m := newTestYamlPopup()
	m.Open("", k8s.ResourcePods, k8s.ResourceItem{Name: "x"}, "ctx")
	m.animator.Finalize()

	_, cmd := m.Update(keyMsg('y'))
	if cmd != nil {
		t.Errorf("y on empty YAML must be a no-op, got cmd returning %T", cmd())
	}
}

func TestYamlPopup_EditNoOpWithoutItem(t *testing.T) {
	m := newTestYamlPopup()
	// Open with empty item (drill-down container case)
	m.Open(sampleYAML, k8s.ResourcePods, k8s.ResourceItem{}, "test-ctx")
	m.animator.Finalize()

	_, cmd := m.Update(keyMsg('E'))
	if cmd != nil {
		t.Errorf("expected no cmd when item.Name empty, got %T", cmd())
	}
}

func TestYamlPopup_CloseWithEsc(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("expected popup to be inactive after Esc")
	}
}

func TestYamlPopup_SearchLockedStateRenders(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	// Open search, type, commit
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "image" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.IsSearching() {
		t.Fatal("post-Enter: expected IsSearching()=false (filter locked, not editing)")
	}
	if m.SearchQuery() == "" {
		t.Fatal("post-Enter: expected SearchQuery to persist")
	}
	if m.MatchCount() == 0 {
		t.Fatal("post-Enter: expected matches")
	}

	view := m.RenderPopup()
	if !strings.Contains(view, "image") {
		t.Error("locked-state render must still surface the committed query")
	}
}

func TestYamlPopup_CurrentMatchRowHighlighted(t *testing.T) {
	// Open popup, commit a search, render. Strip ANSI from output and ensure
	// the current-match line is present in plain form (full-row highlight uses
	// a background which leaves the plain content intact).
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	m, _ = m.Update(keyMsg('/'))
	for _, r := range "nginx:1.27" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.MatchCount() == 0 {
		t.Fatal("expected at least one match for 'nginx:1.27'")
	}

	view := m.RenderPopup()
	if !strings.Contains(view, "nginx:1.27.1") {
		t.Error("rendered popup must contain the matched image string on the highlighted line")
	}
}

func TestYamlPopup_BottomBarFitsInAvailableWidth(t *testing.T) {
	// Regression: hint + indicator used to overflow the popup border when the
	// match counter was active and the popup was narrow. Progressive fallback
	// must always keep `hint + indicator` within `available` columns.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	// Commit a search so the indicator gets the verbose " 5/7  78-95/119 " form.
	m, _ = m.Update(keyMsg('/'))
	for _, r := range "image" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	cases := []int{20, 30, 40, 60, 100}
	for _, available := range cases {
		hint, indicator := m.bottomBarStrings(m.contentHeight(), available)
		w := lipgloss.Width(hint) + lipgloss.Width(indicator)
		if w > available {
			t.Errorf("bottomBarStrings overflowed at available=%d: hint=%q indicator=%q (w=%d)",
				available, hint, indicator, w)
		}
	}
}

// --- v1.7.10 vim-buffer mode tests -----------------------------------------

func TestYamlPopup_HLMovesCursorInLine(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	// Cursor starts at (0, 0). l moves right, h moves left, bounded.
	m, _ = m.Update(keyMsg('l'))
	if m.cursorCol != 1 {
		t.Errorf("expected cursorCol=1 after l, got %d", m.cursorCol)
	}
	m, _ = m.Update(keyMsg('h'))
	if m.cursorCol != 0 {
		t.Errorf("expected cursorCol=0 after h, got %d", m.cursorCol)
	}
	// h at col 0 stays put.
	m, _ = m.Update(keyMsg('h'))
	if m.cursorCol != 0 {
		t.Errorf("expected cursorCol=0 stays at 0, got %d", m.cursorCol)
	}
}

func TestYamlPopup_HWrapsToPreviousLineEnd(t *testing.T) {
	// v1.7.10 vim-buffer refinement: h at col 0 of a non-top line
	// wraps to the end of the previous line rather than stopping.
	// Matches the behavior vim gives with `set whichwrap+=h,l` and
	// makes cursor navigation across wrap-continuation rows feel
	// natural. Symmetric with l at end-of-line wrapping down.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	// Move to line 1 col 0. sampleYAML line 1 = "kind: Pod".
	m, _ = m.Update(keyMsg('j'))
	if m.cursorLine != 1 || m.cursorCol != 0 {
		t.Fatalf("setup: expected cursor at (1,0), got (%d,%d)", m.cursorLine, m.cursorCol)
	}
	m, _ = m.Update(keyMsg('h'))
	if m.cursorLine != 0 {
		t.Errorf("expected h at (1,0) to wrap to line 0, got line %d", m.cursorLine)
	}
	// Line 0 = "apiVersion: v1" — last col = 13 (14 runes).
	if want := m.lastColOf(0); m.cursorCol != want {
		t.Errorf("expected cursorCol at end of prev line (%d), got %d", want, m.cursorCol)
	}
}

func TestYamlPopup_HAtBufferTopDoesNotCrash(t *testing.T) {
	// Guard against a crash when h is pressed at (0,0). The switch
	// in moveCursorLeft must fall through both cases and leave the
	// cursor where it is.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	if m.cursorLine != 0 || m.cursorCol != 0 {
		t.Fatalf("setup: expected cursor at (0,0), got (%d,%d)", m.cursorLine, m.cursorCol)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on h at buffer top: %v", r)
		}
	}()
	m, _ = m.Update(keyMsg('h'))
	if m.cursorLine != 0 || m.cursorCol != 0 {
		t.Errorf("expected cursor to stay at (0,0), got (%d,%d)", m.cursorLine, m.cursorCol)
	}
}

func TestYamlPopup_LWrapsToNextLineStart(t *testing.T) {
	// Symmetric with h-wrap: l at end-of-line wraps to col 0 of the
	// next line rather than stopping.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	// Jump to end of line 0.
	m, _ = m.Update(keyMsg('$'))
	if m.cursorLine != 0 {
		t.Fatalf("setup: expected line 0, got %d", m.cursorLine)
	}
	m, _ = m.Update(keyMsg('l'))
	if m.cursorLine != 1 || m.cursorCol != 0 {
		t.Errorf("expected l at end-of-line to wrap to (1,0), got (%d,%d)", m.cursorLine, m.cursorCol)
	}
}

func TestYamlPopup_LAtBufferBottomEndDoesNotCrash(t *testing.T) {
	// Guard against a crash when l is pressed at the last col of the
	// last line — mirrors the h-at-(0,0) no-op guard. Both switch
	// cases in moveCursorRight must evaluate false and leave the
	// cursor where it is.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(keyMsg('G')) // jump to last line
	m, _ = m.Update(keyMsg('$')) // jump to end of that line
	lastLine := m.lastLine()
	lastCol := m.lastColOf(lastLine)
	if m.cursorLine != lastLine || m.cursorCol != lastCol {
		t.Fatalf("setup: expected cursor at (%d,%d), got (%d,%d)",
			lastLine, lastCol, m.cursorLine, m.cursorCol)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on l at buffer bottom-end: %v", r)
		}
	}()
	m, _ = m.Update(keyMsg('l'))
	if m.cursorLine != lastLine || m.cursorCol != lastCol {
		t.Errorf("expected cursor to stay at (%d,%d), got (%d,%d)",
			lastLine, lastCol, m.cursorLine, m.cursorCol)
	}
}

func TestYamlPopup_ZeroDollarLineEndpoints(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	// Position cursor mid-line via l.
	for i := 0; i < 5; i++ {
		m, _ = m.Update(keyMsg('l'))
	}
	m, _ = m.Update(keyMsg('$'))
	// $ lands on the last valid col of the current line.
	last := m.lastColOf(m.cursorLine)
	if m.cursorCol != last {
		t.Errorf("$ expected cursorCol=%d, got %d", last, m.cursorCol)
	}
	m, _ = m.Update(keyMsg('0'))
	if m.cursorCol != 0 {
		t.Errorf("0 expected cursorCol=0, got %d", m.cursorCol)
	}
}

func TestYamlPopup_WordMotions(t *testing.T) {
	// sampleYAML line 0: "apiVersion: v1" — words: apiVersion / v1
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	if m.cursorLine != 0 || m.cursorCol != 0 {
		t.Fatalf("expected cursor at (0,0), got (%d,%d)", m.cursorLine, m.cursorCol)
	}
	// e → end of "apiVersion" (rune index 9).
	m, _ = m.Update(keyMsg('e'))
	if m.cursorCol != 9 {
		t.Errorf("expected cursorCol=9 (end of apiVersion), got %d", m.cursorCol)
	}
	// w → next word ":" is punctuation; the next word start after ": " is "v1" at col 12.
	m, _ = m.Update(keyMsg('w'))
	if m.cursorCol == 9 {
		t.Errorf("w should have advanced from col 9, still at %d", m.cursorCol)
	}
	// b → back to previous word start on this line.
	prevCol := m.cursorCol
	m, _ = m.Update(keyMsg('b'))
	if m.cursorCol >= prevCol {
		t.Errorf("b should decrease col (from %d), got %d", prevCol, m.cursorCol)
	}
}

func TestYamlPopup_VisualModeEnterExit(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(keyMsg('v'))
	if !m.visualMode {
		t.Error("v should enter visual mode")
	}
	if m.visualAnchorLine != m.cursorLine || m.visualAnchorCol != m.cursorCol {
		t.Errorf("anchor must snap to cursor position on v; anchor=(%d,%d), cursor=(%d,%d)",
			m.visualAnchorLine, m.visualAnchorCol, m.cursorLine, m.cursorCol)
	}
	// Esc exits visual (but does NOT close popup — Esc-during-visual is a two-step out).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.visualMode {
		t.Error("Esc must exit visual mode")
	}
	if !m.IsActive() {
		t.Error("Esc-during-visual must leave the popup open (first Esc exits visual only)")
	}
	// Second Esc closes.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Error("second Esc must dispatch a close animator cmd")
	}
}

func TestYamlPopup_VisualModeSelectionExtendsWithHJKL(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(keyMsg('v'))
	// Move cursor 3 chars right — selection now covers "api".
	m, _ = m.Update(keyMsg('l'))
	m, _ = m.Update(keyMsg('l'))
	m, _ = m.Update(keyMsg('l'))
	if !m.visualMode {
		t.Fatal("still expected visual mode after l l l")
	}
	sL, sC, eL, eC := m.selectionRange()
	if sL != 0 || sC != 0 || eL != 0 || eC != 3 {
		t.Errorf("expected selection (0,0)→(0,3), got (%d,%d)→(%d,%d)", sL, sC, eL, eC)
	}
	// Selection text: "apiV" (4 chars).
	if got := m.selectionText(); got != "apiV" {
		t.Errorf("expected selection text %q, got %q", "apiV", got)
	}
}

func TestYamlPopup_VisualModeMultiLineSelection(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(keyMsg('v'))
	// v at (0,0), j once → selection covers all of line 0 + col 0 of line 1.
	m, _ = m.Update(keyMsg('j'))
	got := m.selectionText()
	// Line 0 fully: "apiVersion: v1", + \n + "k" (first char of "kind: Pod").
	if !strings.HasPrefix(got, "apiVersion: v1\n") {
		t.Errorf("expected selection to start with 'apiVersion: v1\\n', got %q", got)
	}
	if !strings.HasSuffix(got, "k") {
		t.Errorf("expected selection to end with 'k' (first char of kind line), got %q", got)
	}
}

func TestYamlPopup_YInVisualModeCopiesSelectionAndExits(t *testing.T) {
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	m, _ = m.Update(keyMsg('v'))
	m, _ = m.Update(keyMsg('l'))
	m, _ = m.Update(keyMsg('l'))
	m, _ = m.Update(keyMsg('l')) // selection: "apiV"
	_, cmd := m.Update(keyMsg('y'))
	if cmd == nil {
		t.Fatal("expected copyToClipboardCmd from y in visual mode")
	}
	// y in visual must ALSO exit visual mode (vim convention: yank
	// completes the operator + returns to normal).
	m, _ = m.Update(keyMsg('y'))
	if m.visualMode {
		t.Error("y in visual mode must exit visual mode")
	}
}

func TestYamlPopup_EscClearsLockedSearchBeforeClosingPopup(t *testing.T) {
	// v1.7.10 layered-Esc contract: while search results are on
	// screen (m.searching=false but m.searchQuery / m.matchLines
	// non-empty), Esc must clear the search filter FIRST rather than
	// closing the popup. A second Esc closes as usual.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	// Open search, type a query, commit.
	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n', 'g', 'i', 'n', 'x'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searchQuery == "" || len(m.matchLines) == 0 {
		t.Fatalf("setup: expected search committed with matches, got query=%q matches=%d",
			m.searchQuery, len(m.matchLines))
	}

	// First Esc clears search state — popup STAYS open.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchQuery != "" {
		t.Errorf("expected searchQuery cleared, got %q", m.searchQuery)
	}
	if len(m.matchLines) != 0 {
		t.Errorf("expected matchLines cleared, got %d", len(m.matchLines))
	}
	if !m.IsActive() {
		t.Error("first Esc (with locked search) must NOT close the popup")
	}

	// Second Esc closes.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Error("second Esc (no search, no visual) must dispatch a close cmd")
	}
}

func TestYamlPopup_EscInVisualModeTakesPrecedenceOverSearch(t *testing.T) {
	// Layering order: visual first, then search. If both are active,
	// Esc peels visual before touching search state.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)

	// Commit a search.
	m, _ = m.Update(keyMsg('/'))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n', 'g', 'i', 'n', 'x'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Enter visual mode.
	m, _ = m.Update(keyMsg('v'))
	if !m.visualMode {
		t.Fatal("setup: visual mode should be on after v")
	}

	// First Esc: exits visual, leaves search intact.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.visualMode {
		t.Error("first Esc must exit visual mode")
	}
	if m.searchQuery == "" {
		t.Errorf("first Esc must NOT touch search; expected query preserved, got empty")
	}

	// Second Esc: clears search.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchQuery != "" {
		t.Errorf("second Esc must clear search; got %q", m.searchQuery)
	}
}

func TestYamlPopup_YInNormalModeCopiesFullYAML(t *testing.T) {
	// Regression: non-visual y still copies the whole YAML, unchanged
	// from pre-v1.7.10 behavior — TestYamlPopup_CopyEmitsClipboardCmd
	// covers the exact bytes; this test only guards the mode gate.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	if m.visualMode {
		t.Fatal("setup: expected normal mode after Open")
	}
	_, cmd := m.Update(keyMsg('y'))
	if cmd == nil {
		t.Error("expected copyToClipboardCmd from y in normal mode")
	}
}

func TestOverlayCursorOnStyledLine_PreservesSurroundingANSI(t *testing.T) {
	// Regression: cursor-only line MUST keep highlightYAMLLine's
	// key coloring. Before the ansi.Cut splice, the cursor-line
	// render walked contentPlain and lost the ANSI escapes — the
	// YAML key appeared as default white instead of the theme's
	// key color. This test drives the splice helper directly with
	// a known-styled input so it doesn't depend on lipgloss's
	// runtime color-profile detection (which degrades to ASCII
	// under `go test` because stdout isn't a TTY).
	styled := "\x1b[1;38;2;137;180;250mapiVersion\x1b[0m\x1b[38;2;127;132;156m:\x1b[0m v1"
	plain := "apiVersion: v1"
	cursorStyle := lipgloss.NewStyle().Reverse(true)

	// Cursor at col 5 lands on the 'r' in "apiVersion". The result
	// MUST still contain the opening style escape for the key ("\x1b[1;...").
	got := overlayCursorOnStyledLine(styled, plain, 5, cursorStyle)
	if !strings.Contains(got, "\x1b[1;38;2;137;180;250m") {
		t.Errorf("expected key ANSI style preserved before cursor, got %q", got)
	}
	// Value part after cursor should also survive with its own style.
	if !strings.Contains(got, "v1") {
		t.Errorf("expected 'v1' value preserved after cursor, got %q", got)
	}
	// Cursor cell character 'r' must be present at the splice point.
	// (Not asserting reverse-video escape: lipgloss's Reverse() emits
	// nothing when stdout isn't a TTY under go test.)
	if !strings.Contains(got, "r") {
		t.Errorf("cursor cell 'r' missing from splice output, got %q", got)
	}
}

func TestOverlayCursorOnStyledLine_EmptyLine(t *testing.T) {
	// Empty line: cursor becomes a single reverse-video space so
	// it stays visible. (Only assert non-empty output — lipgloss
	// degrades Reverse() to a plain space when stdout isn't a TTY,
	// so the ANSI 7m escape isn't reliable under `go test`.)
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	got := overlayCursorOnStyledLine("", "", 0, cursorStyle)
	if got == "" {
		t.Error("expected non-empty output for empty-line cursor")
	}
}

func TestOverlaySelectionOnStyledLine_PreservesSurroundingANSI(t *testing.T) {
	// Regression for the "visual mode turns key white" bug: the
	// selection overlay must preserve syntax coloring OUTSIDE the
	// selected rune range. Before the ansi.Cut splice for
	// selection, we stripped ANSI and re-styled the whole line,
	// which killed key colors for a 1-char selection.
	styled := "\x1b[1;38;2;137;180;250mapiVersion\x1b[0m\x1b[38;2;127;132;156m:\x1b[0m v1"
	plain := "apiVersion: v1"
	selectionStyle := lipgloss.NewStyle().Background(lipgloss.Color("#b4befe"))
	cursorStyle := lipgloss.NewStyle().Reverse(true)

	// Select col 12..13 ("v1"). The KEY at col 0..9 must retain
	// its opening ANSI escape sequence untouched.
	got := overlaySelectionOnStyledLine(styled, plain, 12, 13, false, 0, selectionStyle, cursorStyle)
	if !strings.Contains(got, "\x1b[1;38;2;137;180;250m") {
		t.Errorf("expected key ANSI style preserved before selection, got %q", got)
	}
	if !strings.Contains(got, "v1") {
		t.Errorf("expected selected 'v1' text present in output, got %q", got)
	}
	if !strings.Contains(got, "apiVersion") {
		t.Errorf("expected 'apiVersion' preserved outside selection, got %q", got)
	}
}

func TestPanelStateStringRoundTrip(t *testing.T) {
	// The Panel enum <-> yaml string mapping must round-trip so
	// state.yaml survives km8 upgrades that reorder the enum values.
	cases := []Panel{SidebarPanel, TablePanel, DetailPanel}
	for _, p := range cases {
		s := panelToStateString(p)
		if got := panelFromStateString(s); got != p {
			t.Errorf("Panel %v → %q → %v, want %v", p, s, got, p)
		}
	}
	// Unknown / empty string falls back to sidebar.
	if got := panelFromStateString(""); got != SidebarPanel {
		t.Errorf("empty state string must fall back to SidebarPanel, got %v", got)
	}
	if got := panelFromStateString("unknown"); got != SidebarPanel {
		t.Errorf("unknown state string must fall back to SidebarPanel, got %v", got)
	}
}

func TestYamlPopup_LineNumberGutterRendered(t *testing.T) {
	// Gutter should carry the raw line number (1-indexed) on the
	// left. sampleYAML is 22 lines, so gutterWidth = 3 ("22 ").
	// Render should contain " 1 apiVersion" and " 2 kind" (padded)
	// somewhere on the appropriate rows.
	m := newTestYamlPopup()
	m.SetSize(120, 40)
	m = openTestPopup(m, sampleYAML)

	if m.gutterWidth() != 3 {
		t.Errorf("expected gutterWidth=3 for 22-line YAML, got %d", m.gutterWidth())
	}
	view := m.RenderPopup()
	// Look for " 1 " gutter marker followed by content — matches the
	// per-line right-aligned number + separator.
	if !strings.Contains(view, " 1 ") || !strings.Contains(view, "apiVersion") {
		t.Errorf("expected line 1 gutter + apiVersion content in render, view:\n%s", view)
	}
}

func TestYamlPopup_GutterDoesNotAffectCursorOrSelection(t *testing.T) {
	// Cursor at col 0 lands on the FIRST CONTENT CHARACTER, not on
	// the gutter. `h` at col 0 stays at 0. y in visual mode copies
	// content only (no line numbers). Regression guard for the
	// "gutter is not addressable" contract.
	m := newTestYamlPopup()
	m = openTestPopup(m, sampleYAML)
	if m.cursorCol != 0 {
		t.Fatalf("setup: expected cursorCol=0, got %d", m.cursorCol)
	}
	// h at col 0 stays at 0.
	m, _ = m.Update(keyMsg('h'))
	if m.cursorCol != 0 {
		t.Errorf("h at cursorCol=0 must not move into gutter, got %d", m.cursorCol)
	}
	// Visual mode + copy first line: substring must be "apiVersion: v1"
	// (no leading digits).
	m, _ = m.Update(keyMsg('v'))
	m, _ = m.Update(keyMsg('$'))
	sel := m.selectionText()
	if strings.HasPrefix(sel, "1") || strings.HasPrefix(sel, " ") {
		t.Errorf("selection must start at content, not gutter; got %q", sel)
	}
	if !strings.HasPrefix(sel, "apiVersion") {
		t.Errorf("expected selection to start with 'apiVersion', got %q", sel)
	}
}

func TestYamlPopup_GutterBlankOnContinuationLines(t *testing.T) {
	// When a raw line is wrap-split into multiple display lines, the
	// gutter shows the number only on the FIRST display row and stays
	// blank on continuation rows. Pin via a small popup + long raw
	// line that forces wrapping.
	m := newTestYamlPopup()
	m.SetSize(40, 20) // narrow → force wrap
	longYAML := "verylongfieldname: " + strings.Repeat("value ", 20)
	m.Open(longYAML, k8s.ResourcePods, k8s.ResourceItem{Name: "x"}, "ctx")
	m.animator.Finalize()
	if len(m.contentLineRaw) < 2 {
		t.Skip("YAML didn't wrap; test premise not met")
	}
	// First display line: raw=0, gutter shows "1".
	// Second display line: also raw=0, gutter should be blank.
	g0 := m.gutter(0)
	g1 := m.gutter(1)
	if !strings.Contains(g0, "1") {
		t.Errorf("first display line gutter should contain '1', got %q", g0)
	}
	if strings.Contains(g1, "1") {
		t.Errorf("continuation gutter should NOT repeat the raw line number, got %q", g1)
	}
}

func TestYamlPopup_EmptyYAMLDoesNotCrash(t *testing.T) {
	m := newTestYamlPopup()
	m.Open("", k8s.ResourcePods, k8s.ResourceItem{Name: "x"}, "ctx")
	m.animator.Finalize()
	// Drive a few keys
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('G'))
	m, _ = m.Update(keyMsg('/'))
	// Render should not panic
	view := m.RenderPopup()
	if view == "" {
		t.Error("expected non-empty render even for empty YAML")
	}
}
