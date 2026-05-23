package ui

import (
	"os"
	"strings"
	"testing"
)

func TestBuildShellTerminalCmd_UsesShellEnv(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Setenv("SHELL", "/usr/bin/fish")
	cmd := buildShellTerminalCmd()
	if cmd == nil {
		t.Fatal("buildShellTerminalCmd returned nil")
	}
	if cmd.Args[0] != "/usr/bin/fish" {
		t.Errorf("expected /usr/bin/fish, got %q", cmd.Args[0])
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "-l" {
		t.Errorf("expected login shell flag, got args %v", cmd.Args)
	}
}

func TestBuildShellTerminalCmd_FallbackWhenShellUnset(t *testing.T) {
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	os.Unsetenv("SHELL")
	cmd := buildShellTerminalCmd()
	if cmd.Args[0] != "/bin/sh" {
		t.Errorf("expected /bin/sh fallback, got %q", cmd.Args[0])
	}
}

func TestTerminalTitle_PrefixAndSuffix(t *testing.T) {
	title := terminalTitle()
	if !strings.Contains(title, "KM8erm") {
		t.Errorf("title must contain 'KM8erm' marker, got %q", title)
	}
	// We deliberately pass mDNS suffixes through (`.local`, `.home`, ...)
	// because the user wants the raw hostname. No suffix assertion.
}
