package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// PtyKind discriminates the three kinds of PTY popup. Only PtyKindShell
// supports hide-without-kill (Alt+T toggle); Edit and Exec are transient and
// always end when the subprocess exits.
type PtyKind int

const (
	PtyKindShell PtyKind = iota // KM8erm (T key)
	PtyKindEdit                 // `kubectl edit`
	PtyKindExec                 // `kubectl exec -it -- /bin/sh`
)

// PtyView renders an embedded PTY in a popup overlay. Used for `kubectl edit`,
// `kubectl exec`, and the KM8erm internal shell so subprocess output stays
// inside km8 instead of leaking into the host terminal scrollback after quit.
//
// The Shell-kind variant is *persistent*: pressing Alt+T inside the popup
// hides it without killing the subprocess. The shell keeps running in the
// background (readLoop continues, scrollback continues accumulating) and the
// next `T` press from outside re-attaches.
type PtyView struct {
	active       bool
	hidden       bool // alive but not currently rendered; only meaningful for PtyKindShell
	kind         PtyKind
	title        string
	hostW, hostH int

	ptmx *os.File
	term vt10x.Terminal
	cmd  *exec.Cmd

	done *atomic.Bool
	mu   *sync.Mutex

	// Scrollback: a ring buffer of historical lines captured from the PTY
	// byte stream. scrollOffset is how many lines back from the latest we're
	// currently displaying; 0 = live (use vt10x). Mouse wheel adjusts it.
	// pendingLine is a pointer because strings.Builder may not be value-copied
	// (its internal addr-check panics) — PtyView itself is value-copied through
	// Update's receiver, so any inline Builder would race + panic.
	scrollback   []string
	pendingLine  *strings.Builder
	pendingCR    bool // last byte was \r — wait for next byte to decide CRLF vs progress-bar reset
	scrollOffset int
}

const maxScrollbackLines = 10000

func NewPtyView() *PtyView {
	return &PtyView{}
}

// IsActive reports whether the popup should be drawn AND receive input
// (alive and not hidden). Use this for view-overlay + key-routing checks.
func (p *PtyView) IsActive() bool { return p.active && !p.hidden }

// IsAlive reports whether the subprocess is running, regardless of whether
// the popup is currently visible. Use this for status-bar marker rendering
// and to refuse new PTY launches while one is in flight.
func (p *PtyView) IsAlive() bool { return p.active }

// IsHidden reports whether a subprocess is alive but the popup is hidden.
// Equivalent to IsAlive() && !IsActive().
func (p *PtyView) IsHidden() bool { return p.active && p.hidden }

// Kind returns the kind of PTY currently running (shell / edit / exec).
// Meaningless when IsAlive() is false.
func (p *PtyView) Kind() PtyKind { return p.kind }

// Title returns the title set at Start time. Used by the status-bar marker.
func (p *PtyView) Title() string { return p.title }

// Hide marks the popup as hidden without killing the subprocess. Only takes
// effect for PtyKindShell; transient PTYs (edit / exec) ignore the call.
func (p *PtyView) Hide() {
	if !p.active || p.kind != PtyKindShell {
		return
	}
	p.hidden = true
}

// Show un-hides the popup and refreshes the PTY size to the current host
// dimensions (it may have changed while hidden). No-op if not alive.
func (p *PtyView) Show(hostW, hostH int) {
	if !p.active {
		return
	}
	p.hidden = false
	p.SetSize(hostW, hostH)
}

// Start launches cmd in a PTY sized to fit a popup inside hostW × hostH.
// Returns a tick Cmd that drives the render loop and exit detection. The
// `kind` parameter controls whether the popup supports persistent-hide.
func (p *PtyView) Start(cmd *exec.Cmd, title string, hostW, hostH int, kind PtyKind) tea.Cmd {
	p.active = true
	p.hidden = false
	p.kind = kind
	p.title = title
	p.cmd = cmd
	p.hostW = hostW
	p.hostH = hostH
	p.done = &atomic.Bool{}
	p.mu = &sync.Mutex{}
	p.scrollback = nil
	p.pendingLine = &strings.Builder{}
	p.scrollOffset = 0

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
			p.captureToScrollback(buf[:n])
			p.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	_ = p.cmd.Wait()
	p.done.Store(true)
}

// captureToScrollback splits the PTY byte stream into lines and appends them
// to the ring buffer. Carriage returns reset the pending line (terminals use
// \r alone for in-place updates like progress bars — we keep the latest
// content). ANSI escape sequences are stripped before storage so cursor-
// positioning, color, and clear codes don't become invisible "blank" entries
// (zsh / fancy prompts emit dozens of these per command). mu must be held by
// caller.
func (p *PtyView) captureToScrollback(buf []byte) {
	// Detect clear-screen sequences and reset scrollback to match the
	// terminal's visible state:
	//   `\x1b[2J`     — erase entire screen
	//   `\x1b[3J`     — erase scrollback (xterm extension)
	//   `\x1b[H\x1b[J` — cursor home + erase-below (what macOS `clear` sends)
	//   `\x1bc`        — RIS (full terminal reset)
	// `\x1b[J` alone is NOT enough (zsh prompt redraw uses it too) — must
	// be preceded by cursor-home to confirm a screen clear.
	if bytes.Contains(buf, []byte("\x1b[2J")) ||
		bytes.Contains(buf, []byte("\x1b[3J")) ||
		bytes.Contains(buf, []byte("\x1b[H\x1b[J")) ||
		bytes.Contains(buf, []byte("\x1bc")) {
		p.scrollback = nil
		p.scrollOffset = 0
		p.pendingLine.Reset()
		p.pendingCR = false
		return
	}
	for _, b := range buf {
		// Resolve a pending \r based on what follows. \r\n is a line
		// terminator (CRLF); a lone \r is an in-place progress-bar reset.
		// macOS shells (zsh / oh-my-zsh) emit CRLF — without this the line
		// gets reset before the \n commits it, leaving scrollback empty.
		if p.pendingCR {
			p.pendingCR = false
			if b == '\n' {
				p.commitScrollbackLine()
				continue
			}
			p.pendingLine.Reset()
		}
		switch b {
		case '\r':
			p.pendingCR = true
		case '\n':
			p.commitScrollbackLine()
		default:
			p.pendingLine.WriteByte(b)
		}
	}
}

func (p *PtyView) commitScrollbackLine() {
	raw := p.pendingLine.String()
	// Filter using the visible content (post-strip) so pure-ANSI lines like
	// zsh prompt repaints don't become invisible "blank" entries — but
	// STORE the raw line so its colors / styles re-render correctly when
	// the user scrolls back.
	if strings.TrimSpace(ansi.Strip(raw)) != "" {
		p.scrollback = append(p.scrollback, raw)
		if len(p.scrollback) > maxScrollbackLines {
			p.scrollback = p.scrollback[len(p.scrollback)-maxScrollbackLines:]
		}
	}
	p.pendingLine.Reset()
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
	p.hidden = false
	p.cmd = nil
	p.ptmx = nil
	p.term = nil
}

func (p *PtyView) Update(msg tea.Msg) (*PtyView, tea.Cmd) {
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
		// Alt+T hides KM8erm popup without killing the shell — persistent PTY.
		// Always intercepted for PtyKindShell regardless of alt-screen mode;
		// users running vim *inside* KM8erm still need an escape hatch to
		// peek at km8 panels without losing their shell session.
		// Edit/Exec popups pass it through (transient — no hide concept).
		if p.kind == PtyKindShell {
			switch msg.String() {
			case "alt+t", "alt+T":
				p.hidden = true
				return p, nil
			}
		}

		// Scrollback navigation keys (PgUp/PgDn/Home/End) intercept ONLY
		// when the PTY isn't in alt-screen mode — i.e. plain shell output.
		// Full-screen apps (vim / nvim / less / htop / kubectl edit's
		// editor) switch to alt screen, so we forward all keys to them
		// and they keep PgUp/PgDn/Home/End semantics. Match on
		// msg.String() so terminal-specific encodings of these keys are
		// all caught.
		altScreen := p.term != nil && p.term.Mode()&vt10x.ModeAltScreen != 0
		if p.term != nil && !altScreen {
			switch msg.String() {
			case "pgup", "shift+pgup":
				p.scrollPage(-1)
				return p, nil
			case "pgdown", "shift+pgdown":
				p.scrollPage(1)
				return p, nil
			case "home", "shift+home":
				p.scrollToEnd(-1) // top of scrollback
				return p, nil
			case "end", "shift+end":
				p.scrollToEnd(1) // back to live
				return p, nil
			}
		}
		if p.ptmx == nil {
			return p, nil
		}
		appCursor := p.term != nil && p.term.Mode()&vt10x.ModeAppCursor != 0
		raw := ptyKeyBytes(msg, appCursor)
		if len(raw) > 0 {
			// Typing snaps the view back to live — standard terminal
			// behavior. User scrolled to read history, now they're
			// interacting again, so jump back to the prompt.
			p.scrollOffset = 0
			_, _ = p.ptmx.Write(raw)
		}
		return p, nil
	}
	return p, nil
}

// scrollToEnd jumps to the top (direction=-1, oldest line at top of viewport)
// or bottom (direction=+1, live) of the scrollback. Wired to Home / End.
// Shell line-edit users have Ctrl+A / Ctrl+E as readline-native equivalents.
func (p *PtyView) scrollToEnd(direction int) {
	if p.term == nil {
		return
	}
	_, rows := p.term.Size()
	p.mu.Lock()
	total := len(p.scrollback)
	p.mu.Unlock()
	maxOffset := total - rows
	if maxOffset <= 0 {
		p.scrollOffset = 0
		return
	}
	if direction < 0 {
		p.scrollOffset = maxOffset
	} else {
		p.scrollOffset = 0
	}
}

// scrollPage moves the scrollback view by one viewport.
// direction = -1 → PgUp (look further back, scrollOffset grows).
// direction = +1 → PgDown (look forward, scrollOffset shrinks; 0 = live).
// Clamped to [0, total-rows]; if the buffer fits in the viewport,
// scrollOffset is forced to 0.
func (p *PtyView) scrollPage(direction int) {
	if p.term == nil {
		return
	}
	_, rows := p.term.Size()
	p.mu.Lock()
	total := len(p.scrollback)
	p.mu.Unlock()
	maxOffset := total - rows
	if maxOffset <= 0 {
		p.scrollOffset = 0
		return
	}
	p.scrollOffset -= direction * rows
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}
	if p.scrollOffset > maxOffset {
		p.scrollOffset = maxOffset
	}
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
func (p *PtyView) ptyDims() (cols, rows int) {
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

func (p *PtyView) View() string { return "" }

// RenderPopup builds the title bar + bordered PTY grid as a single string
// ready for overlay.Composite over the main view. Returns empty when the
// popup should not be drawn — either no subprocess alive, or the subprocess
// is alive but hidden (Alt+T from a Shell-kind PtyView).
func (p *PtyView) RenderPopup() string {
	if !p.IsActive() || p.term == nil {
		return ""
	}
	cols, rows := p.term.Size()

	var lines []string
	p.mu.Lock()
	// Re-clamp scrollOffset against the latest scrollback length + viewport.
	// The wheel handler clamps too, but a window-resize between events can
	// shrink the maximum without re-clamping the existing offset.
	total := len(p.scrollback)
	maxOffset := total - rows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.scrollOffset > maxOffset {
		p.scrollOffset = maxOffset
	}
	if p.scrollOffset > 0 && maxOffset > 0 {
		// Scrollback mode: render a slice of historical lines instead of
		// vt10x's live state. end never exceeds total, start never below 0.
		end := total - p.scrollOffset
		start := end - rows
		if start < 0 {
			start = 0
		}
		for _, l := range p.scrollback[start:end] {
			// Lines hold raw output (with ANSI color codes preserved).
			// Use visual-width (ansi.StringWidth) for layout math — len()
			// would count escape bytes and break the popup grid.
			w := ansi.StringWidth(l)
			if w > cols {
				l = ansi.Truncate(l, cols, "")
			} else if w < cols {
				l = l + strings.Repeat(" ", cols-w)
			}
			lines = append(lines, l)
		}
		// Bottom-pad with blanks so the viewport is always exactly `rows`
		// lines tall. Without this, scrolling far enough up to expose the
		// head of the buffer would leave the bottom of the popup empty
		// (background bleeds through the overlay).
		for len(lines) < rows {
			lines = append(lines, strings.Repeat(" ", cols))
		}
	} else {
		cursorX, cursorY := -1, -1
		if p.term.CursorVisible() {
			c := p.term.Cursor()
			cursorX, cursorY = c.X, c.Y
		}
		for y := 0; y < rows; y++ {
			var line strings.Builder
			for x := 0; x < cols; x++ {
				glyph := p.term.Cell(x, y)
				isCursor := x == cursorX && y == cursorY
				line.WriteString(renderGlyph(glyph, isCursor))
			}
			lines = append(lines, line.String())
		}
	}
	p.mu.Unlock()

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#74c7ec"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F0AE49")).Bold(true)

	// Title gains a [SCROLLED N] marker while the user is viewing history so
	// the state is unmistakable. Cleared the moment scrollOffset returns to 0
	// (mouse wheel down to the bottom = back to live).
	titleText := p.title

	// Manually compose ╭─ 󰵅 Title ────╮ / │ content │ / ╰─────╯ so the title
	// sits inside the top border line and shares the same popup glyph as the
	// other popup overlays (toast / confirm / namespace / context).
	// Note: `─` is 3 bytes in UTF-8 so we must NOT use len() for visual widths.
	title := " " + titleText + " "
	titleW := lipgloss.Width(title)
	if titleW > cols-2 {
		title = " "
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
	bottom := p.renderBottomBorder(cols, borderStyle, titleStyle)
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

// renderBottomBorder builds the closing border with a key-hint embedded in
// the dashes — Alt+T for Shell-kind (persistent), plus scrollback navigation
// when the PTY isn't in alt-screen mode. Edit/Exec popups still get the
// scrollback hint; Shell gets both. If the available width is too narrow to
// fit any hint, falls back to plain dashes.
func (p *PtyView) renderBottomBorder(cols int, borderStyle, hintStyle lipgloss.Style) string {
	hint := ""
	altScreen := p.term != nil && p.term.Mode()&vt10x.ModeAltScreen != 0
	switch {
	case p.kind == PtyKindShell && !altScreen:
		hint = " Alt+T:hide  PgUp/Home:scroll "
	case p.kind == PtyKindShell && altScreen:
		hint = " Alt+T:hide "
	case !altScreen:
		hint = " PgUp/Home:scroll "
	}
	if hint == "" || lipgloss.Width(hint)+4 > cols {
		return borderStyle.Render("╰" + strings.Repeat("─", cols) + "╯")
	}
	hintW := lipgloss.Width(hint)
	trail := cols - hintW - 1
	if trail < 0 {
		trail = 0
	}
	return borderStyle.Render("╰─") + hintStyle.Render(hint) + borderStyle.Render(strings.Repeat("─", trail)+"╯")
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
