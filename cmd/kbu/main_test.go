package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	terminu "github.com/vulcanshen/terminu-go"

	"github.com/vulcanshen/kbu/v4/internal/config"
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
// (tdp E1); `--version` and `-v` do too. Nothing else does.
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

// What a command prints takes the family's form (tdp E1): results on
// stdout, status 0; iconwidth's two lines, each width with where it came
// from; a command kbu does not know on stderr, after "kbu: ", a blank line
// and the usage, status 2, nothing on stdout.
func TestCommand(t *testing.T) {
	t.Setenv("KBU__ICON_WIDTH", "2")
	t.Setenv("KBU__POWERLINE_WIDTH", "1")
	for _, c := range []struct {
		args           []string
		status         int
		stdout, stderr string
	}{
		{[]string{"version"}, 0, "kbu dev\n", ""},
		{[]string{"help"}, 0, usage, ""},
		{[]string{"iconwidth"}, 0, "icon 2 (KBU__ICON_WIDTH)\npowerline 1 (KBU__POWERLINE_WIDTH)\n", ""},
		{[]string{"x"}, 2, "", "kbu: Unknown command \"x\"\n\n" + usage},
		{[]string{"--versions"}, 2, "", "kbu: Unknown command \"--versions\"\n\n" + usage},
	} {
		var stdout, stderr strings.Builder
		if got := command(c.args, &stdout, &stderr); got != c.status || stdout.String() != c.stdout || stderr.String() != c.stderr {
			t.Errorf("kbu %q: status %d, stdout %q, stderr %q; want %d, %q, %q",
				c.args, got, stdout.String(), stderr.String(), c.status, c.stdout, c.stderr)
		}
	}
}

// A command kbu does not know exits with status 2 instead of starting kbu.
func TestMain_UnknownCommand(t *testing.T) {
	out, err := kbuRun(t, "x")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.HasPrefix(out, "kbu: Unknown command \"x\"\n\n") {
		t.Errorf("kbu x: %q (err %v), want status 2", out, err)
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

// tdp E3: a value kbu cannot use is put back to its default, the file
// copied to .bak first and written again; the start's toast says so, a WARN
// for each value. When the copy cannot be made, the file is left as it was.
func TestFitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("editor: vi\ninput_tab_width: -2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	notices := fitted(path, cfg.Misfits(), func() error { return cfg.SaveTo(path) })
	want := []string{"1 value in config.yaml did not fit: defaults used, yours kept in config.yaml.bak",
		`config.yaml: input_tab_width "-2" did not fit; using "4"`}
	if len(notices) != 1 || notices[0].Kind != terminu.Warning || !slices.Equal(notices[0].Lines(), want) {
		t.Errorf("notices %+v, want one Warning saying %q", notices, want)
	}
	if b, _ := os.ReadFile(path + ".bak"); string(b) != "editor: vi\ninput_tab_width: -2\n" {
		t.Errorf("config.yaml.bak is %q", b)
	}
	if b, _ := os.ReadFile(path); strings.Contains(string(b), "input_tab_width") || !strings.Contains(string(b), "editor: vi") {
		t.Errorf("config.yaml was not written again with the default:\n%s", b)
	}
	if notices := fitted(path, nil, nil); notices != nil {
		t.Errorf("nothing that did not fit said %+v", notices)
	}

	if runtime.GOOS == "windows" {
		return // a folder Windows does not keep from being written to
	}
	ro := t.TempDir()
	path = filepath.Join(ro, "config.yaml")
	if err := os.WriteFile(path, []byte("input_tab_width: -2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	cfg = &config.Config{InputTabWidth: -2}
	saved := false
	notices = fitted(path, cfg.Misfits(), func() error { saved = true; return nil })
	if saved || len(notices) != 2 || notices[0].Kind != terminu.Error || !strings.Contains(notices[0].Text, "config.yaml left as it was") {
		t.Errorf("a copy that cannot be made: saved %v, notices %+v", saved, notices)
	}
}

// tdp E3: a context the kubeconfig no longer has is left as written, and kbu
// starts on the next one, the settings' or the kubeconfig's current one,
// with a Warning naming it.
func TestConnect(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
current-context: a
clusters:
- name: c
  cluster: {server: "https://127.0.0.1:6443"}
users:
- name: u
  user: {token: t}
contexts:
- name: a
  context: {cluster: c, user: u}
- name: b
  context: {cluster: c, user: u}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)
	for _, c := range []struct {
		left, setting, want string
		warns               []string
	}{
		{"b", "", "b", nil},
		{"", "", "a", nil},
		{"gone", "", "a", []string{`Context "gone" in state.yaml is not in the kubeconfig: using "a"`}},
		{"gone", "b", "b", []string{`Context "gone" in state.yaml is not in the kubeconfig: using "b"`}},
		{"", "old", "a", []string{`Context "old" in config.yaml is not in the kubeconfig: using "a"`}},
	} {
		client, notices, err := connect(c.left, c.setting)
		if err != nil {
			t.Fatalf("connect(%q, %q): %v", c.left, c.setting, err)
		}
		var warns []string
		for _, n := range notices {
			if n.Kind == terminu.Warning {
				warns = append(warns, n.Text)
			}
		}
		if client.ContextName() != c.want || !slices.Equal(warns, c.warns) {
			t.Errorf("connect(%q, %q): on %q, warnings %q; want %q, %q", c.left, c.setting, client.ContextName(), warns, c.want, c.warns)
		}
	}
}
