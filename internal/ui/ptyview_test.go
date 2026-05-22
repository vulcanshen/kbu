package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hinshun/vt10x"
)

func TestPtyView_Initial_Inactive(t *testing.T) {
	v := NewPtyView()
	if v.IsActive() {
		t.Fatal("new PtyView must be inactive")
	}
	if got := v.RenderPopup(); got != "" {
		t.Errorf("inactive RenderPopup must be empty, got %q", got)
	}
	if got := v.View(); got != "" {
		t.Errorf("View must always return empty, got %q", got)
	}
}

func TestPtyView_PtyDims_ClampsToMinimum(t *testing.T) {
	tests := []struct {
		name           string
		hostW, hostH   int
		wantMinCols    int
		wantMinRows    int
		wantExactCols  int
		wantExactRows  int
		useExactAssert bool
	}{
		{"tiny host clamps to minimum", 10, 5, 20, 5, 0, 0, false},
		{"normal host = host - 2*margin - border", 100, 50, 0, 0, 94, 45, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := PtyView{hostW: tt.hostW, hostH: tt.hostH}
			cols, rows := v.ptyDims()
			if tt.useExactAssert {
				if cols != tt.wantExactCols || rows != tt.wantExactRows {
					t.Errorf("ptyDims(%dx%d) = (%d, %d), want (%d, %d)",
						tt.hostW, tt.hostH, cols, rows, tt.wantExactCols, tt.wantExactRows)
				}
				return
			}
			if cols < tt.wantMinCols {
				t.Errorf("cols=%d below minimum %d", cols, tt.wantMinCols)
			}
			if rows < tt.wantMinRows {
				t.Errorf("rows=%d below minimum %d", rows, tt.wantMinRows)
			}
		})
	}
}

func TestPtyKeyBytes_Runes_Passthrough(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")}
	if got := string(ptyKeyBytes(msg, false)); got != "hello" {
		t.Errorf("KeyRunes passthrough = %q, want %q", got, "hello")
	}
}

func TestPtyKeyBytes_AppCursorOverrides(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyUp}

	normal := ptyKeyBytes(msg, false)
	if len(normal) != 3 || normal[0] != 0x1b || normal[1] != '[' || normal[2] != 'A' {
		t.Errorf("KeyUp normal = %v, want ESC [ A", normal)
	}

	appCur := ptyKeyBytes(msg, true)
	if len(appCur) != 3 || appCur[0] != 0x1b || appCur[1] != 'O' || appCur[2] != 'A' {
		t.Errorf("KeyUp appCursor = %v, want ESC O A", appCur)
	}
}

func TestPtyKeyBytes_SpecialKeys(t *testing.T) {
	cases := []struct {
		key  tea.KeyType
		want []byte
	}{
		{tea.KeyEnter, []byte{'\r'}},
		{tea.KeyTab, []byte{'\t'}},
		{tea.KeyBackspace, []byte{'\x7f'}},
		{tea.KeyEscape, []byte{'\x1b'}},
		{tea.KeyCtrlC, []byte{'\x03'}},
		{tea.KeyCtrlD, []byte{'\x04'}},
	}
	for _, c := range cases {
		got := ptyKeyBytes(tea.KeyMsg{Type: c.key}, false)
		if string(got) != string(c.want) {
			t.Errorf("key %v = %v, want %v", c.key, got, c.want)
		}
	}
}

// TestPtyView_StartEcho_Exits drives an echo subprocess through the PTY and
// verifies the exit message arrives via tick-based detection. Light
// integration test — requires the host OS to support PTY allocation.
func TestPtyView_StartEcho_Exits(t *testing.T) {
	v := NewPtyView()
	cmd := exec.Command("echo", "hello world")
	startCmd := v.Start(cmd, "echo test", 80, 24)
	if startCmd == nil {
		t.Fatal("Start must return a tick command")
	}
	if !v.IsActive() {
		t.Fatal("after Start, IsActive must be true")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if v.done != nil && v.done.Load() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if v.done == nil || !v.done.Load() {
		t.Fatal("echo subprocess did not flag done within deadline")
	}

	updated, cmd2 := v.Update(ptyTickMsg{})
	if cmd2 == nil {
		t.Fatal("expected exit command from tick after done flag set")
	}
	msg := cmd2()
	exit, ok := msg.(PtyExitMsg)
	if !ok {
		t.Fatalf("expected PtyExitMsg, got %T", msg)
	}
	if exit.ExitCode != 0 {
		t.Errorf("echo exited with code %d, want 0", exit.ExitCode)
	}
	if updated.IsActive() {
		t.Error("PtyView must be inactive after PtyExitMsg dispatch")
	}
}

func TestPtyView_CaptureToScrollback_SplitsOnNewline(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	p.mu.Lock()
	p.captureToScrollback([]byte("hello\nworld\nfoo"))
	p.mu.Unlock()

	if len(p.scrollback) != 2 {
		t.Fatalf("expected 2 finalized lines, got %d", len(p.scrollback))
	}
	if p.scrollback[0] != "hello" || p.scrollback[1] != "world" {
		t.Errorf("expected [hello, world], got %v", p.scrollback)
	}
	if p.pendingLine.String() != "foo" {
		t.Errorf("expected pending=foo, got %q", p.pendingLine.String())
	}
}

func TestPtyView_CaptureToScrollback_CarriageReturnResetsLine(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	// Progress-bar style: write content, then \r resets, then write more, then \n commits.
	p.mu.Lock()
	p.captureToScrollback([]byte("50%\rDONE\n"))
	p.mu.Unlock()

	if len(p.scrollback) != 1 {
		t.Fatalf("expected 1 line, got %d", len(p.scrollback))
	}
	if p.scrollback[0] != "DONE" {
		t.Errorf("expected DONE (after \\r reset), got %q", p.scrollback[0])
	}
}

func TestPtyView_CaptureToScrollback_RingBufferCap(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	// Write maxScrollbackLines + 100 lines.
	var sb strings.Builder
	want := maxScrollbackLines + 100
	for i := 0; i < want; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	p.mu.Lock()
	p.captureToScrollback([]byte(sb.String()))
	p.mu.Unlock()

	if len(p.scrollback) != maxScrollbackLines {
		t.Errorf("ring buffer size = %d, want capped at %d", len(p.scrollback), maxScrollbackLines)
	}
	// Oldest 100 should have been dropped — first remaining is line 100.
	if p.scrollback[0] != "line 100" {
		t.Errorf("oldest retained line = %q, want 'line 100'", p.scrollback[0])
	}
	// Newest = line want-1.
	last := p.scrollback[len(p.scrollback)-1]
	wantLast := fmt.Sprintf("line %d", want-1)
	if last != wantLast {
		t.Errorf("newest line = %q, want %q", last, wantLast)
	}
}

func TestPtyView_CaptureToScrollback_PreservesAnsiButFiltersAnsiOnly(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	// "\x1b[31merror\x1b[0m" — has visible "error" after strip → keep raw.
	// "\x1b[?25l\x1b[?25h" — pure ANSI noise → filtered.
	// "plain" — keep.
	p.mu.Lock()
	p.captureToScrollback([]byte("\x1b[31merror\x1b[0m\n\x1b[?25l\x1b[?25h\nplain\n"))
	p.mu.Unlock()

	if len(p.scrollback) != 2 {
		t.Fatalf("expected 2 lines (ANSI-only filtered), got %d: %v", len(p.scrollback), p.scrollback)
	}
	// Raw ANSI preserved for rendering.
	if p.scrollback[0] != "\x1b[31merror\x1b[0m" {
		t.Errorf("expected raw ANSI preserved, got %q", p.scrollback[0])
	}
	if p.scrollback[1] != "plain" {
		t.Errorf("expected 'plain', got %q", p.scrollback[1])
	}
}

func TestPtyView_CaptureToScrollback_ClearScreenResetsBuffer(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	p.mu.Lock()
	p.captureToScrollback([]byte("line1\nline2\nline3\n"))
	if len(p.scrollback) != 3 {
		t.Fatalf("setup: expected 3 lines, got %d", len(p.scrollback))
	}
	// macOS `clear` emits "\x1b[H\x1b[J"; xterm-256color's clear capability
	// also supports "\x1b[2J" and "\x1b[3J". Test the macOS variant.
	p.captureToScrollback([]byte("\x1b[H\x1b[J"))
	p.mu.Unlock()

	if len(p.scrollback) != 0 {
		t.Errorf("expected scrollback cleared after \\x1b[2J, got %d: %v", len(p.scrollback), p.scrollback)
	}
	if p.scrollOffset != 0 {
		t.Errorf("expected scrollOffset reset to 0 after clear, got %d", p.scrollOffset)
	}
}

func TestPtyView_ScrollPage_PgUpGrowsOffset(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}
	p.term = vt10x.New(vt10x.WithSize(80, 10))
	// Inject 50 scrollback lines so PgUp has room.
	for i := 0; i < 50; i++ {
		p.scrollback = append(p.scrollback, fmt.Sprintf("line %d", i))
	}

	p.scrollPage(-1) // PgUp
	if p.scrollOffset != 10 {
		t.Errorf("PgUp on 50 lines × 10 rows: expected offset=10, got %d", p.scrollOffset)
	}

	p.scrollPage(-1) // PgUp again
	if p.scrollOffset != 20 {
		t.Errorf("second PgUp: expected offset=20, got %d", p.scrollOffset)
	}

	p.scrollPage(1) // PgDown
	if p.scrollOffset != 10 {
		t.Errorf("PgDown: expected offset=10, got %d", p.scrollOffset)
	}

	// PgDown past zero — clamps.
	p.scrollPage(1)
	p.scrollPage(1)
	if p.scrollOffset != 0 {
		t.Errorf("PgDown past bottom should clamp at 0, got %d", p.scrollOffset)
	}
}

func TestPtyView_ScrollPage_BufferFitsViewport_NoMove(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}
	p.term = vt10x.New(vt10x.WithSize(80, 24))
	// Only 5 lines — fits in 24-row viewport.
	for i := 0; i < 5; i++ {
		p.scrollback = append(p.scrollback, fmt.Sprintf("line %d", i))
	}

	p.scrollPage(-1)
	if p.scrollOffset != 0 {
		t.Errorf("PgUp on short buffer must not change offset, got %d", p.scrollOffset)
	}
}

func TestPtyView_ScrollPage_PgUpClampsAtMax(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}
	p.term = vt10x.New(vt10x.WithSize(80, 10))
	for i := 0; i < 15; i++ {
		p.scrollback = append(p.scrollback, fmt.Sprintf("line %d", i))
	}
	// maxOffset = 15 - 10 = 5.
	p.scrollPage(-1) // would add 10, clamp to 5
	if p.scrollOffset != 5 {
		t.Errorf("PgUp must clamp at maxOffset=5, got %d", p.scrollOffset)
	}
	// Another PgUp doesn't go beyond.
	p.scrollPage(-1)
	if p.scrollOffset != 5 {
		t.Errorf("subsequent PgUp must stay at maxOffset=5, got %d", p.scrollOffset)
	}
}

func TestPtyView_CaptureToScrollback_HandlesCRLF(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	// macOS shells (and Windows) emit CRLF (\r\n) as line terminator.
	// Before the pendingCR state-tracking, \r would reset the line before
	// \n could commit it — scrollback ended up empty.
	p.mu.Lock()
	p.captureToScrollback([]byte("1\r\n2\r\n3\r\n"))
	p.mu.Unlock()

	if len(p.scrollback) != 3 {
		t.Fatalf("CRLF should yield 3 lines, got %d: %v", len(p.scrollback), p.scrollback)
	}
	for i, want := range []string{"1", "2", "3"} {
		if p.scrollback[i] != want {
			t.Errorf("line %d = %q, want %q", i, p.scrollback[i], want)
		}
	}
}

func TestPtyView_CaptureToScrollback_CRLF_AcrossChunks(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}

	// \r at end of one chunk, \n at start of next — pendingCR must persist
	// between captureToScrollback calls.
	p.mu.Lock()
	p.captureToScrollback([]byte("hello\r"))
	p.captureToScrollback([]byte("\nworld\r\n"))
	p.mu.Unlock()

	if len(p.scrollback) != 2 {
		t.Fatalf("expected 2 lines across chunked CRLF, got %d: %v", len(p.scrollback), p.scrollback)
	}
	if p.scrollback[0] != "hello" || p.scrollback[1] != "world" {
		t.Errorf("expected [hello, world], got %v", p.scrollback)
	}
}

func TestPtyView_ScrollToEnd_HomeAndEnd(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}
	p.term = vt10x.New(vt10x.WithSize(80, 10))
	for i := 0; i < 50; i++ {
		p.scrollback = append(p.scrollback, fmt.Sprintf("line %d", i))
	}

	// Home → top (maxOffset = 50 - 10 = 40)
	p.scrollToEnd(-1)
	if p.scrollOffset != 40 {
		t.Errorf("Home: expected offset=40 (maxOffset), got %d", p.scrollOffset)
	}

	// End → back to live
	p.scrollToEnd(1)
	if p.scrollOffset != 0 {
		t.Errorf("End: expected offset=0 (live), got %d", p.scrollOffset)
	}
}

func TestPtyView_ScrollToEnd_BufferFitsViewport_NoMove(t *testing.T) {
	p := &PtyView{}
	p.mu = &sync.Mutex{}
	p.pendingLine = &strings.Builder{}
	p.term = vt10x.New(vt10x.WithSize(80, 24))
	for i := 0; i < 5; i++ {
		p.scrollback = append(p.scrollback, fmt.Sprintf("line %d", i))
	}

	p.scrollToEnd(-1)
	if p.scrollOffset != 0 {
		t.Errorf("Home on short buffer: expected offset=0, got %d", p.scrollOffset)
	}
}
