package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

type ptyTickMsg struct{}

// PtyExitMsg is emitted when the PTY subprocess exits. ExitCode 0 means
// success; non-zero means the subprocess failed (kubectl's error text is
// already visible in the PTY buffer up until the moment it closes).
type PtyExitMsg struct {
	ExitCode int
}

// PtyView renders an embedded PTY in a popup overlay. Used for `kubectl edit`
// and `kubectl exec` so subprocess output stays inside km8 instead of leaking
// into the host terminal scrollback after quit.
type PtyView struct {
	active       bool
	title        string
	hostW, hostH int

	ptmx *os.File
	term vt10x.Terminal
	cmd  *exec.Cmd

	done *atomic.Bool
	mu   *sync.Mutex
}

func NewPtyView() PtyView {
	return PtyView{}
}

func (p PtyView) IsActive() bool { return p.active }

// Start launches cmd in a PTY sized to fit a popup inside hostW × hostH.
// Returns a tick Cmd that drives the render loop and exit detection.
func (p *PtyView) Start(cmd *exec.Cmd, title string, hostW, hostH int) tea.Cmd {
	p.active = true
	p.title = title
	p.cmd = cmd
	p.hostW = hostW
	p.hostH = hostH
	p.done = &atomic.Bool{}
	p.mu = &sync.Mutex{}

	cols, rows := p.ptyDims()
	p.term = vt10x.New(vt10x.WithSize(cols, rows))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		p.active = false
		return func() tea.Msg {
			return PtyExitMsg{ExitCode: -1}
		}
	}
	p.ptmx = ptmx

	go p.readLoop()
	return p.tick()
}

func (p *PtyView) tick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return ptyTickMsg{}
	})
}

func (p *PtyView) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := p.ptmx.Read(buf)
		if n > 0 {
			p.mu.Lock()
			_, _ = p.term.Write(buf[:n])
			p.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	_ = p.cmd.Wait()
	p.done.Store(true)
}

// Stop force-terminates the PTY subprocess (if still running) and clears state.
func (p *PtyView) Stop() {
	if p.cmd != nil && p.cmd.Process != nil && p.done != nil && !p.done.Load() {
		_ = p.cmd.Process.Kill()
	}
	if p.ptmx != nil {
		_ = p.ptmx.Close()
	}
	p.active = false
	p.cmd = nil
	p.ptmx = nil
	p.term = nil
}

func (p PtyView) Update(msg tea.Msg) (PtyView, tea.Cmd) {
	if !p.active {
		return p, nil
	}
	switch msg := msg.(type) {
	case ptyTickMsg:
		if p.done != nil && p.done.Load() {
			exitCode := 0
			if p.cmd != nil && p.cmd.ProcessState != nil {
				exitCode = p.cmd.ProcessState.ExitCode()
			}
			p.Stop()
			return p, func() tea.Msg {
				return PtyExitMsg{ExitCode: exitCode}
			}
		}
		return p, p.tick()

	case tea.KeyMsg:
		if p.ptmx == nil {
			return p, nil
		}
		appCursor := p.term != nil && p.term.Mode()&vt10x.ModeAppCursor != 0
		raw := ptyKeyBytes(msg, appCursor)
		if len(raw) > 0 {
			_, _ = p.ptmx.Write(raw)
		}
		return p, nil
	}
	return p, nil
}

// SetSize updates the host dimensions and resizes the underlying PTY so the
// running subprocess (vim, sh, etc.) redraws to the new size via SIGWINCH.
func (p *PtyView) SetSize(hostW, hostH int) {
	p.hostW = hostW
	p.hostH = hostH
	if !p.active || p.ptmx == nil {
		return
	}
	cols, rows := p.ptyDims()
	_ = pty.Setsize(p.ptmx, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if p.term != nil {
		p.term.Resize(cols, rows)
	}
}

// ptyDims returns the PTY content dimensions inside the popup. Margins are
// fixed (preserving host parity so overlay.Composite centers symmetrically)
// and asymmetric: horizontal margin is wider than vertical because terminals
// are typically much wider than tall.
func (p PtyView) ptyDims() (cols, rows int) {
	const (
		popupMarginX = 2
		popupMarginY = 1
	)
	popupW := p.hostW - 2*popupMarginX
	popupH := p.hostH - 2*popupMarginY
	cols = popupW - 2 // left + right border
	rows = popupH - 3 // top border + bottom border + 1 row for title
	if cols < 20 {
		cols = 20
	}
	if rows < 5 {
		rows = 5
	}
	return cols, rows
}

func (p PtyView) View() string { return "" }

// RenderPopup builds the title bar + bordered PTY grid as a single string
// ready for overlay.Composite over the main view.
func (p PtyView) RenderPopup() string {
	if !p.active || p.term == nil {
		return ""
	}
	cols, rows := p.term.Size()

	p.mu.Lock()
	cursorX, cursorY := -1, -1
	if p.term.CursorVisible() {
		c := p.term.Cursor()
		cursorX, cursorY = c.X, c.Y
	}
	var lines []string
	for y := 0; y < rows; y++ {
		var line strings.Builder
		for x := 0; x < cols; x++ {
			glyph := p.term.Cell(x, y)
			isCursor := x == cursorX && y == cursorY
			line.WriteString(renderGlyph(glyph, isCursor))
		}
		lines = append(lines, line.String())
	}
	p.mu.Unlock()

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#74c7ec"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F0AE49")).Bold(true)

	// Manually compose ╭─ 󰵅 Title ────╮ / │ content │ / ╰─────╯ so the title
	// sits inside the top border line and shares the same popup glyph as the
	// other popup overlays (toast / confirm / namespace / context).
	// Note: `─` is 3 bytes in UTF-8 so we must NOT use len() for visual widths.
	title := " " + popupGlyph + " " + p.title + " "
	titleW := lipgloss.Width(title)
	if titleW > cols-2 {
		title = " " + popupGlyph + " "
		titleW = lipgloss.Width(title)
	}
	const leadDashCount = 2
	leadDashes := strings.Repeat("─", leadDashCount)
	trailLen := cols - titleW - leadDashCount
	if trailLen < 0 {
		trailLen = 0
	}
	trailDashes := strings.Repeat("─", trailLen)
	top := borderStyle.Render("╭"+leadDashes) + titleStyle.Render(title) + borderStyle.Render(trailDashes+"╮")
	bottom := borderStyle.Render("╰" + strings.Repeat("─", cols) + "╯")
	vbar := borderStyle.Render("│")

	var out strings.Builder
	out.WriteString(top)
	out.WriteString("\n")
	for _, line := range lines {
		out.WriteString(vbar)
		out.WriteString(line)
		out.WriteString(vbar)
		out.WriteString("\n")
	}
	out.WriteString(bottom)
	return out.String()
}

// vt10x attr bit positions (package-private constants in state.go; values
// fixed by iota order: reverse=1, underline=2, bold=4, italic=16).
const (
	vtAttrReverse   int16 = 1
	vtAttrUnderline int16 = 2
	vtAttrBold      int16 = 4
	vtAttrItalic    int16 = 16
	vtAttrAny             = vtAttrReverse | vtAttrUnderline | vtAttrBold | vtAttrItalic
)

// renderGlyph maps a vt10x cell to a lipgloss-styled rune. The hot-path
// shortcut: a cell with default colors, no attributes, and not under the
// cursor emits the raw rune directly — for an 80×24 grid at 20 ticks/s that
// avoids ~30k lipgloss style allocations per second.
func renderGlyph(g vt10x.Glyph, isCursor bool) string {
	ch := string(g.Char)
	if g.Char == 0 {
		ch = " "
	}
	defaultFG := g.FG == vt10x.DefaultFG
	defaultBG := g.BG == vt10x.DefaultBG
	hasAttrs := g.Mode&vtAttrAny != 0
	if !isCursor && defaultFG && defaultBG && !hasAttrs {
		return ch
	}

	style := lipgloss.NewStyle()
	if !defaultFG {
		if fg, ok := vtColorToLipgloss(g.FG); ok {
			style = style.Foreground(fg)
		}
	}
	if !defaultBG {
		if bg, ok := vtColorToLipgloss(g.BG); ok {
			style = style.Background(bg)
		}
	}
	if g.Mode&vtAttrBold != 0 {
		style = style.Bold(true)
	}
	if g.Mode&vtAttrUnderline != 0 {
		style = style.Underline(true)
	}
	if g.Mode&vtAttrItalic != 0 {
		style = style.Italic(true)
	}
	reverse := g.Mode&vtAttrReverse != 0
	if isCursor {
		reverse = !reverse
	}
	if reverse {
		style = style.Reverse(true)
	}
	return style.Render(ch)
}

// vtColorToLipgloss maps a vt10x Color to a lipgloss color. The default
// foreground/background return ok=false so the host terminal default applies.
//
// vt10x encodes colors as: 0–15 ANSI, 16–255 xterm-256, 256+ 24-bit RGB
// (r<<16 | g<<8 | b). Lipgloss accepts "0".."255" for the palette indices
// and "#RRGGBB" for true-color; without this split, RGB values become
// uninterpretable strings and modern editors render as plain white.
func vtColorToLipgloss(c vt10x.Color) (lipgloss.Color, bool) {
	if c == vt10x.DefaultFG || c == vt10x.DefaultBG || c == vt10x.DefaultCursor {
		return "", false
	}
	u := uint32(c)
	if u < 256 {
		return lipgloss.Color(fmt.Sprintf("%d", u)), true
	}
	r := (u >> 16) & 0xFF
	g := (u >> 8) & 0xFF
	b := u & 0xFF
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b)), true
}

// ptyKeyBytes converts a Bubble Tea KeyMsg into the raw byte sequence a real
// terminal would write to a process's stdin. appCursor selects the DEC
// application-cursor sequences (\x1bO_) when the running app has set DECCKM
// (vim's normal mode); otherwise the standard sequences (\x1b[_) apply.
func ptyKeyBytes(msg tea.KeyMsg, appCursor bool) []byte {
	if msg.Type == tea.KeyRunes {
		return []byte(string(msg.Runes))
	}
	if appCursor {
		if b, ok := ptyKeyBytesAppCursorMap[msg.Type]; ok {
			return b
		}
	}
	if b, ok := ptyKeyBytesMap[msg.Type]; ok {
		return b
	}
	s := msg.String()
	if len(s) == 1 {
		return []byte(s)
	}
	return nil
}

var ptyKeyBytesMap = map[tea.KeyType][]byte{
	tea.KeyEnter:     {'\r'},
	tea.KeyTab:       {'\t'},
	tea.KeyBackspace: {'\x7f'},
	tea.KeyDelete:    {'\x1b', '[', '3', '~'},
	tea.KeySpace:     {' '},
	tea.KeyEscape:    {'\x1b'},
	tea.KeyUp:        {'\x1b', '[', 'A'},
	tea.KeyDown:      {'\x1b', '[', 'B'},
	tea.KeyRight:     {'\x1b', '[', 'C'},
	tea.KeyLeft:      {'\x1b', '[', 'D'},
	tea.KeyHome:      {'\x1b', '[', 'H'},
	tea.KeyEnd:       {'\x1b', '[', 'F'},
	tea.KeyPgUp:      {'\x1b', '[', '5', '~'},
	tea.KeyPgDown:    {'\x1b', '[', '6', '~'},
	tea.KeyCtrlA:     {'\x01'},
	tea.KeyCtrlB:     {'\x02'},
	tea.KeyCtrlC:     {'\x03'},
	tea.KeyCtrlD:     {'\x04'},
	tea.KeyCtrlE:     {'\x05'},
	tea.KeyCtrlF:     {'\x06'},
	tea.KeyCtrlG:     {'\x07'},
	tea.KeyCtrlH:     {'\x08'},
	tea.KeyCtrlK:     {'\x0b'},
	tea.KeyCtrlL:     {'\x0c'},
	tea.KeyCtrlN:     {'\x0e'},
	tea.KeyCtrlO:     {'\x0f'},
	tea.KeyCtrlP:     {'\x10'},
	tea.KeyCtrlQ:     {'\x11'},
	tea.KeyCtrlR:     {'\x12'},
	tea.KeyCtrlS:     {'\x13'},
	tea.KeyCtrlT:     {'\x14'},
	tea.KeyCtrlU:     {'\x15'},
	tea.KeyCtrlV:     {'\x16'},
	tea.KeyCtrlW:     {'\x17'},
	tea.KeyCtrlX:     {'\x18'},
	tea.KeyCtrlY:     {'\x19'},
	tea.KeyCtrlZ:     {'\x1a'},
}

var ptyKeyBytesAppCursorMap = map[tea.KeyType][]byte{
	tea.KeyUp:    {'\x1b', 'O', 'A'},
	tea.KeyDown:  {'\x1b', 'O', 'B'},
	tea.KeyRight: {'\x1b', 'O', 'C'},
	tea.KeyLeft:  {'\x1b', 'O', 'D'},
}
