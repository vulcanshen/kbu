package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain lets a test run kbu's own main(): the test binary, started again
// with KBU_TEST_RUN_MAIN=1, is kbu with the arguments it was given.
func TestMain(m *testing.M) {
	if os.Getenv("KBU_TEST_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// `kbu version` prints the version, as in the rest of the terminu family;
// the older `--version` and `-v` still do. Nothing else does: anything
// else on the command line starts kbu as before.
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

// Each way of asking prints "kbu <version>" and exits before kbu reads a
// config or reaches for a cluster. The config and state point at an empty
// directory and the kubeconfig at nothing, so a run that went on would fail
// there instead of touching either.
func TestMain_PrintsVersion(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		cmd := exec.Command(os.Args[0], arg)
		cmd.Env = append(os.Environ(),
			"KBU_TEST_RUN_MAIN=1",
			"KBU__CONFIG="+t.TempDir(),
			"KBU__STATE="+t.TempDir(),
			"KUBECONFIG="+filepath.Join(t.TempDir(), "none"),
		)
		out, err := cmd.CombinedOutput()
		if err != nil || string(out) != "kbu dev\n" {
			t.Errorf("kbu %s: %q (err %v), want %q", arg, out, err, "kbu dev\n")
		}
	}
}
