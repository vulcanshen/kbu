package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/theme"
)

func newTestAppLog() AppLogModel {
	t := theme.DefaultTheme()
	m := NewAppLogModel(t)
	m.SetSize(120, 40)
	return m
}

// openLog toggles the log open and finalizes the animation.
func openLog(m *AppLogModel) {
	m.Toggle()
	m.animator.Finalize()
}

// ── Initial state ──────────────────────────────────────────────────────────

func TestAppLogModel_InitialState(t *testing.T) {
	m := newTestAppLog()

	if m.IsActive() {
		t.Error("log should be inactive initially")
	}
	if m.UnreadErrorCount() != 0 {
		t.Errorf("expected 0 unread errors initially, got %d", m.UnreadErrorCount())
	}
	if m.LastErrorMessage() != "" {
		t.Errorf("expected empty last error initially, got %q", m.LastErrorMessage())
	}
	if m.LastSuccessMessage() != "" {
		t.Errorf("expected empty last success initially, got %q", m.LastSuccessMessage())
	}
}

// ── Add / level helpers ────────────────────────────────────────────────────

func TestAppLogModel_Add_Info(t *testing.T) {
	m := newTestAppLog()
	m.Info("hello")

	if len(m.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.entries))
	}
	if m.entries[0].Level != LogInfo {
		t.Errorf("expected LogInfo level")
	}
	if m.entries[0].Message != "hello" {
		t.Errorf("expected message %q, got %q", "hello", m.entries[0].Message)
	}
}

func TestAppLogModel_Add_Warn_IncrementsErrorCount(t *testing.T) {
	m := newTestAppLog()
	m.Warn("something bad")

	if m.UnreadErrorCount() != 1 {
		t.Errorf("expected 1 unread error after Warn, got %d", m.UnreadErrorCount())
	}
	if m.LastErrorMessage() != "something bad" {
		t.Errorf("expected last error to be set after Warn")
	}
}

func TestAppLogModel_Add_Error_IncrementsErrorCount(t *testing.T) {
	m := newTestAppLog()
	m.Error("boom")

	if m.UnreadErrorCount() != 1 {
		t.Errorf("expected 1 unread error after Error, got %d", m.UnreadErrorCount())
	}
	if m.LastErrorMessage() != "boom" {
		t.Errorf("expected last error = %q, got %q", "boom", m.LastErrorMessage())
	}
}

func TestAppLogModel_Add_Success_SetsLastSuccess(t *testing.T) {
	m := newTestAppLog()
	m.Success("applied")

	if m.LastSuccessMessage() != "applied" {
		t.Errorf("expected lastSuccess = %q, got %q", "applied", m.LastSuccessMessage())
	}
	if m.UnreadErrorCount() != 0 {
		t.Errorf("Success must not increment error count")
	}
}

func TestAppLogModel_Add_TruncatesLongMessage(t *testing.T) {
	m := newTestAppLog()
	long := strings.Repeat("x", maxEntryChars+50)
	m.Info(long)

	msg := m.entries[0].Message
	runes := []rune(msg)
	if len(runes) > maxEntryChars+1 { // +1 for the ellipsis rune
		t.Errorf("message should be truncated to ~%d runes, got %d", maxEntryChars, len(runes))
	}
	if !strings.HasSuffix(msg, "…") {
		t.Error("truncated message must end with ellipsis")
	}
}

func TestAppLogModel_Add_CapsAtMaxEntries(t *testing.T) {
	m := newTestAppLog()
	for i := 0; i < m.maxEntries+10; i++ {
		m.Info("entry")
	}
	if len(m.entries) > m.maxEntries {
		t.Errorf("entries should be capped at %d, got %d", m.maxEntries, len(m.entries))
	}
}

// ── Toggle / unread count ──────────────────────────────────────────────────

func TestAppLogModel_Toggle_Open(t *testing.T) {
	m := newTestAppLog()
	openLog(&m)

	if !m.IsActive() {
		t.Error("log should be active after Toggle open")
	}
}

func TestAppLogModel_Toggle_Close(t *testing.T) {
	m := newTestAppLog()
	openLog(&m)

	m.Toggle()
	m.animator.Finalize()

	if m.IsActive() {
		t.Error("log should be inactive after second Toggle")
	}
}

func TestAppLogModel_Toggle_Open_ClearsUnreadCount(t *testing.T) {
	m := newTestAppLog()
	m.Error("e1")
	m.Error("e2")

	if m.UnreadErrorCount() != 2 {
		t.Fatalf("expected 2 unread errors before open")
	}

	openLog(&m)

	if m.UnreadErrorCount() != 0 {
		t.Errorf("opening log must clear unread error count, got %d", m.UnreadErrorCount())
	}
}

func TestAppLogModel_Toggle_Open_ClearsLastError(t *testing.T) {
	m := newTestAppLog()
	m.Error("some error")
	openLog(&m)

	if m.LastErrorMessage() != "" {
		t.Errorf("opening log must clear lastError, got %q", m.LastErrorMessage())
	}
}

func TestAppLogModel_Toggle_Open_ResetsScroll(t *testing.T) {
	m := newTestAppLog()
	openLog(&m)
	m.scrollOffset = 5

	// Close and reopen — scroll resets.
	m.Toggle()
	m.animator.Finalize()
	openLog(&m)

	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after reopen, got %d", m.scrollOffset)
	}
}

// ── Scroll keybindings ─────────────────────────────────────────────────────

// addManyEntries adds n entries so maxScrollOffset > 0.
func addManyEntries(m *AppLogModel, n int) {
	for i := 0; i < n; i++ {
		m.Info(strings.Repeat("x", 80)) // long enough to wrap
	}
}

func TestAppLogModel_Scroll_J_Down(t *testing.T) {
	m := newTestAppLog()
	addManyEntries(&m, 50)
	openLog(&m)

	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after j, got %d", m.scrollOffset)
	}
}

func TestAppLogModel_Scroll_K_Up(t *testing.T) {
	m := newTestAppLog()
	addManyEntries(&m, 50)
	openLog(&m)

	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('j'))
	m, _ = m.Update(keyMsg('k'))
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after j,j,k, got %d", m.scrollOffset)
	}
}

func TestAppLogModel_Scroll_K_DoesNotGoBelowZero(t *testing.T) {
	m := newTestAppLog()
	openLog(&m)

	m, _ = m.Update(keyMsg('k'))
	if m.scrollOffset != 0 {
		t.Errorf("k at top must stay at 0, got %d", m.scrollOffset)
	}
}

func TestAppLogModel_Scroll_G_JumpsToBottom(t *testing.T) {
	m := newTestAppLog()
	addManyEntries(&m, 50)
	openLog(&m)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	max := m.maxScrollOffset()
	if m.scrollOffset != max {
		t.Errorf("G must jump to maxScrollOffset=%d, got %d", max, m.scrollOffset)
	}
}

func TestAppLogModel_Scroll_g_JumpsToTop(t *testing.T) {
	m := newTestAppLog()
	addManyEntries(&m, 50)
	openLog(&m)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m, _ = m.Update(keyMsg('g'))
	if m.scrollOffset != 0 {
		t.Errorf("g must jump to top (0), got %d", m.scrollOffset)
	}
}

func TestAppLogModel_Scroll_D_ClearsLog(t *testing.T) {
	m := newTestAppLog()
	m.Error("e1")
	m.Error("e2")
	openLog(&m)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if len(m.entries) != 0 {
		t.Errorf("D must clear all entries, got %d", len(m.entries))
	}
	if m.UnreadErrorCount() != 0 {
		t.Errorf("D must reset error count")
	}
	if m.scrollOffset != 0 {
		t.Errorf("D must reset scrollOffset")
	}
}

func TestAppLogModel_Close_OnEsc(t *testing.T) {
	m := newTestAppLog()
	openLog(&m)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.animator.Finalize()
	if m.IsActive() {
		t.Error("Esc must close the log")
	}
}

func TestAppLogModel_InactiveIgnoresKeys(t *testing.T) {
	m := newTestAppLog()
	m.Info("msg")

	before := m.scrollOffset
	m, _ = m.Update(keyMsg('j'))
	if m.scrollOffset != before {
		t.Error("inactive log must ignore scroll keys")
	}
}

// ── renderAllLines ─────────────────────────────────────────────────────────

func TestAppLogModel_RenderAllLines_NewestFirst(t *testing.T) {
	m := newTestAppLog()
	m.Info("first")
	m.Info("second")

	lines := m.renderAllLines()
	if len(lines) == 0 {
		t.Fatal("expected rendered lines")
	}
	// "second" was added last and must appear in the first rendered line.
	if !strings.Contains(lines[0], "second") {
		t.Errorf("newest entry must appear first; got %q", lines[0])
	}
}

func TestAppLogModel_PlainText_NewestFirst(t *testing.T) {
	m := newTestAppLog()
	m.Info("first")
	m.Warn("second warn")
	m.Error("third err")

	got := m.PlainText()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "third err") {
		t.Errorf("line[0] should be newest 'third err', got %q", lines[0])
	}
	if !strings.Contains(lines[2], "first") {
		t.Errorf("line[2] should be oldest 'first', got %q", lines[2])
	}
	// Each line should contain timestamp + LEVEL + message.
	for _, l := range lines {
		if !strings.Contains(l, ":") {
			t.Errorf("line missing timestamp colon: %q", l)
		}
	}
}

func TestAppLogModel_PlainText_EmptyWhenNoEntries(t *testing.T) {
	m := newTestAppLog()
	if got := m.PlainText(); got != "" {
		t.Errorf("empty log should yield empty string, got %q", got)
	}
}

func TestAppLogModel_Y_ReturnsCopyCmd(t *testing.T) {
	m := newTestAppLog()
	m.Info("first")
	openLog(&m)

	_, cmd := m.Update(keyMsg('y'))
	if cmd == nil {
		t.Fatal("y should return a copy Cmd")
	}
	// Execute the Cmd to verify it produces a clipboard message.
	msg := cmd()
	if msg == nil {
		t.Fatal("copy Cmd returned nil msg")
	}
	if _, ok := msg.(ClipboardCopiedMsg); !ok {
		// ClipboardCopyFailedMsg is also acceptable in CI without a terminal
		// — what matters is the Cmd is correctly wired, not whether OSC 52
		// actually reaches a real clipboard.
		if _, ok := msg.(ClipboardCopyFailedMsg); !ok {
			t.Errorf("expected clipboard msg, got %T", msg)
		}
	}
}

func TestAppLogModel_RenderAllLines_EmptyWhenNoEntries(t *testing.T) {
	m := newTestAppLog()
	lines := m.renderAllLines()
	if lines != nil {
		t.Errorf("expected nil lines with no entries, got %v", lines)
	}
}
