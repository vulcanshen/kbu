package ui

import (
	"os/exec"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
