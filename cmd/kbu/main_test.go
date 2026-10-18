package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain lets a test run kbu's own main(): the test binary, started again
// with KBU__TEST_RUN_MAIN=1, is kbu with the arguments it was given.
func TestMain(m *testing.M) {
	if os.Getenv("KBU__TEST_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// `kbu version` prints the version, as in the rest of the terminu family
// (tdp E1); `--version` and `-v` do too. Nothing else does: anything else on
// the command line starts kbu as before.
func TestAsksVersion(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"version"}, true},
		{[]string{"--version"}, true},
		{[]string{"-v"}, true},
		{nil, false},
		{[]string{"iconwidth"}, false},
		{[]string{"versions"}, false},
		{[]string{"-version"}, false},
		{[]string{"help"}, false},
	} {
		if got := asksVersion(c.args); got != c.want {
			t.Errorf("asksVersion(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

// `kbu help` prints how to run kbu (tdp E1); `--help` and `-h` do too.
func TestAsksHelp(t *testing.T) {
	for _, c := range []struct {
		args []string
		want bool
	}{
		{[]string{"help"}, true},
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{nil, false},
		{[]string{"version"}, false},
		{[]string{"helps"}, false},
	} {
		if got := asksHelp(c.args); got != c.want {
			t.Errorf("asksHelp(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

// kbuRun runs kbu's main() with args, before it could read a config or reach
// for a cluster: the config and state point at an empty directory and the
// kubeconfig at nothing, so a run that went on would fail there instead of
// touching either.
func kbuRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(),
		"KBU__TEST_RUN_MAIN=1",
		"KBU__CONFIG="+t.TempDir(),
		"KBU__STATE="+t.TempDir(),
		"KBU__DATA="+t.TempDir(),
		"KUBECONFIG="+filepath.Join(t.TempDir(), "none"),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Each way of asking prints "kbu <version>" and exits.
func TestMain_PrintsVersion(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		if out, err := kbuRun(t, arg); err != nil || out != "kbu dev\n" {
			t.Errorf("kbu %s: %q (err %v), want %q", arg, out, err, "kbu dev\n")
		}
	}
}

// Each way of asking prints the usage, which names the other commands.
func TestMain_PrintsHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		out, err := kbuRun(t, arg)
		if err != nil || !strings.Contains(out, "kbu version") || !strings.Contains(out, "kbu help") {
			t.Errorf("kbu %s: %q (err %v)", arg, out, err)
		}
	}
}
