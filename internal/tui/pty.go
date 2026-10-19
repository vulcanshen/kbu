package tui

import (
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	terminu "github.com/vulcanshen/terminu-go"
)

// Alterm: a shell that lives inside kbu (components/dialog/terminal). It
// keeps running while it is hidden — its directory, history and jobs stay —
// and Alt-t brings it back. Alt-Esc ends it, asked first (tdp K10).

// showAlterm opens Alterm, starting a shell the first time.
func (m *Model) showAlterm() tea.Cmd {
	if m.alterm == nil || m.alterm.Exited() {
		m.alterm = m.newAlterm()
	}
	return m.app.OpenTerminal(m.alterm)
}

func (m *Model) newAlterm() *terminu.Terminal {
	// PgUp and PgDn scroll back. TODO(lib KB7): Home and End, the oldest
	// line and back to live, as before.
	t := &terminu.Terminal{Title: "Alterm", Subject: hostname(), Command: altermCommand(m.cfg.AltermShell, m.cfg.AltermLoginShell),
		Ends: true, History: true}
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

// altermCommand is the shell Alterm runs: $KBU__ALTERM_SHELL, else the
// config's alterm_shell, else $SHELL, else /bin/sh; with -l when
// $KBU__ALTERM_LOGIN_SHELL or alterm_login_shell says so (the variable
// first).
func altermCommand(cfgShell string, cfgLogin bool) []string {
	sh := strings.TrimSpace(os.Getenv("KBU__ALTERM_SHELL"))
	if sh == "" {
		sh = strings.TrimSpace(cfgShell)
	}
	if sh == "" {
		sh = strings.TrimSpace(os.Getenv("SHELL"))
	}
	if sh == "" {
		sh = "/bin/sh"
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

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}
