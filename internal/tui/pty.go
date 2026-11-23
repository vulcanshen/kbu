package tui

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// Alterm: a shell that lives inside kbu (tdp terminal). It
// keeps running while it is hidden — its directory, history and jobs stay —
// and Alt-t brings it back. Alt-Esc ends it, asked first (tdp K10).

// showAlterm opens Alterm, starting a shell the first time.
func (m *Model) showAlterm() tea.Cmd {
	m.log.info("Opened Alterm")
	if m.alterm == nil || m.alterm.Exited() {
		m.alterm = m.newAlterm()
	}
	return m.app.OpenTerminal(m.alterm)
}

func (m *Model) newAlterm() *terminu.Terminal {
	// An interactive terminal, the user types in it (tdp terminal):
	// PgUp goes into the History mode, gg|G to the oldest line and back to
	// live, and Alt-v selects; Home and End stay the shell's (tdp K12).
	t := &terminu.Terminal{Title: "Alterm", Subject: hostname(), Command: altermCommand(runtime.GOOS, m.cfg.AltermShell, m.cfg.AltermLoginShell),
		Ends: true, Interactive: true}
	// Alt-t hides it, as it shows it on a panel. (The Ctrl-T an older kbu
	// also took, for VHS, which cannot send Alt, is gone: on a panel it did
	// not show Alterm, a half alias, tdp K7.)
	t.Keys = []terminu.TerminalKey{{Key: "Alt-t", Desc: "hide", Do: func() tea.Cmd { return t.Hide(m.app) }}}
	t.OnExit = func(err error) tea.Cmd {
		if err != nil {
			return m.toastErr("Alterm ended: " + err.Error()) // tdp F5
		}
		return nil
	}
	return t
}

// lookPath finds a program on PATH; tests give their own.
var lookPath = exec.LookPath

// altermCommand is the shell Alterm runs on goos: $KBU__ALTERM_SHELL, else
// the config's alterm_shell, else $SHELL, else /bin/sh, PowerShell on
// Windows; with -l when $KBU__ALTERM_LOGIN_SHELL or alterm_login_shell says
// so (the variable first). On Windows $SHELL counts only when it can be
// run: one a Unix-like shell set (Git Bash's /usr/bin/bash) is not.
func altermCommand(goos, cfgShell string, cfgLogin bool) []string {
	sh := strings.TrimSpace(os.Getenv("KBU__ALTERM_SHELL"))
	if sh == "" {
		sh = strings.TrimSpace(cfgShell)
	}
	if sh == "" {
		if s := strings.TrimSpace(os.Getenv("SHELL")); s != "" && (goos != "windows" || canRun(s)) {
			sh = s
		}
	}
	if sh == "" {
		sh = defaultShell(goos)
	}
	login := cfgLogin
	if v := strings.TrimSpace(os.Getenv("KBU__ALTERM_LOGIN_SHELL")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			login = b
		}
	}
	if login {
		return []string{sh, "-l"}
	}
	return []string{sh}
}

// defaultShell is Alterm's shell when nothing names one: /bin/sh, and on
// Windows PowerShell, 7 (pwsh) when it is there, else the one Windows has.
func defaultShell(goos string) string {
	if goos != "windows" {
		return "/bin/sh"
	}
	if canRun("pwsh") {
		return "pwsh"
	}
	return "powershell.exe"
}

func canRun(name string) bool {
	_, err := lookPath(name)
	return err == nil
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}
