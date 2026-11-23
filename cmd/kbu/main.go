package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-logr/logr"
	terminu "github.com/vulcanshen/terminu-go"
	"k8s.io/klog/v2"

	"github.com/vulcanshen/kbu/v4/internal/config"
	"github.com/vulcanshen/kbu/v4/internal/k8s"
	"github.com/vulcanshen/kbu/v4/internal/theme"
	"github.com/vulcanshen/kbu/v4/internal/tui"
	"github.com/vulcanshen/kbu/v4/internal/version"
)

// usage is what `kbu help` prints (tdp E1).
const usage = `kbu — a Kubernetes TUI inspired by Lens IDE

Usage:
  kbu              start it, on your kubeconfig's context
  kbu version      print the version (also --version, -v)
  kbu help         print this (also --help, -h)
  kbu update       update to the latest release, the way kbu was installed
  kbu iconwidth    print how many cells an icon and a powerline symbol take
                   here, and where each number came from

Environment:
  KBU__CONFIG               the settings directory (default ~/.config/kbu)
  KBU__STATE                where the session state is kept (default: the settings directory)
  KBU__DATA                 where the logs, pins and sorts are kept (default ~/.kbu)
  KBU__ALTERM_SHELL         the shell Alterm runs
  KBU__ALTERM_LOGIN_SHELL   true runs Alterm's shell as a login shell
  KBU__ICON_WIDTH           1 or 2: how many cells a Nerd Font icon takes, instead of asking the terminal
  KBU__POWERLINE_WIDTH      1 or 2: the same for a powerline symbol
  TERMINU__ICON_WIDTH       the same as KBU__ICON_WIDTH, for every terminu app
  TERMINU__POWERLINE_WIDTH  the same as KBU__POWERLINE_WIDTH, for every terminu app

In kbu, press Space to see what you can do, and ? for the keys.
`

// asksVersion reports whether the command line asks for the version:
// `version`, as in the rest of the terminu family, or `--version` / `-v`.
func asksVersion(args []string) bool {
	return len(args) > 0 && (args[0] == "version" || args[0] == "--version" || args[0] == "-v")
}

// asksHelp reports whether the command line asks for the usage: `help`, or
// `--help` / `-h`.
func asksHelp(args []string) bool {
	return len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h")
}

// command runs what the command line asks for, kbu itself aside, and returns
// its exit status (tdp E1): results on stdout; a wrong use on stderr, after
// "kbu: ", a blank line and the usage, status 2.
func command(args []string, stdout, stderr io.Writer) int {
	switch {
	case asksVersion(args):
		fmt.Fprintln(stdout, "kbu "+version.Display())
	case asksHelp(args):
		fmt.Fprint(stdout, usage)
	case args[0] == "update":
		// The latest release, the way kbu was installed (tdp E1-9, E1-10).
		return terminu.SelfUpdate{App: "kbu", Version: version.Version, Owner: "vulcanshen", Tap: "vulcanshen/tap",
			Source: "git pull && make install"}.Run(stdout, stderr)
	case args[0] == "iconwidth":
		for _, line := range terminu.Probe("kbu").Lines() {
			fmt.Fprintln(stdout, line)
		}
	default:
		fmt.Fprintf(stderr, "kbu: Unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	return 0
}

// fail writes what kept kbu from going on to stderr and exits with status 1
// (tdp E1).
func fail(text string) {
	fmt.Fprintln(os.Stderr, "kbu: "+capital(text))
	os.Exit(1)
}

// fitted copies the file at path aside and writes it again with the
// defaults in place of the misfits, already put in what save writes, and
// says so (tdp E3). When the copy cannot be made, the file is left as it
// was and the defaults hold for this run only.
func fitted(path string, misfits []terminu.Misfit, save func() error) []terminu.Notice {
	if len(misfits) == 0 {
		return nil
	}
	copied, err := terminu.KeepAside(path)
	if err == nil {
		err = save()
	}
	if err != nil {
		return []terminu.Notice{{Kind: terminu.Error, Text: capital(err.Error())}, terminu.MisfitNotice(path, "", misfits...)}
	}
	return []terminu.Notice{terminu.MisfitNotice(path, copied, misfits...)}
}

// connect makes the client on the context kbu starts on: where the user
// left off (left), else the settings' (setting), else the kubeconfig's
// current one. One the kubeconfig no longer has is left as written and the
// next used for this run, with a Warning saying so (tdp E3: a value naming
// an entry that is not there).
func connect(left, setting string) (*k8s.Client, []terminu.Notice, error) {
	var gone []string
	for _, c := range []struct{ name, in string }{{left, "state.yaml"}, {setting, "config.yaml"}, {"", ""}} {
		if c.name == "" && c.in != "" {
			continue
		}
		client, err := k8s.NewClient(c.name)
		if c.name != "" && errors.Is(err, k8s.ErrNoContext) {
			gone = append(gone, fmt.Sprintf("Context %q in %s is not in the kubeconfig", c.name, c.in))
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		var notices []terminu.Notice
		for _, g := range gone {
			notices = append(notices, terminu.Notice{Kind: terminu.Warning, Text: fmt.Sprintf("%s: using %q", g, client.ContextName())})
		}
		return client, notices, nil
	}
	return nil, nil, nil // not reached: the last is the current context, found or an error
}

// capital starts a sentence on stderr with a capital, as a toast does
// (tdp E1).
func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(command(os.Args[1:], os.Stdout, os.Stderr))
	}
	// Suppress k8s client-go / klog output that would corrupt the TUI.
	// logr.Discard() is a true no-op; klog.NewKlogr() would route back through
	// klog itself and risk infinite recursion on certain error paths.
	klog.SetOutput(io.Discard)
	klog.SetLogger(logr.Discard())
	log.SetOutput(io.Discard)

	// tdp E3: what an older kbu kept elsewhere moves, item by item, at the
	// first start; the UI says where to, or what stayed.
	moved := config.MoveOldPlaces()

	// tdp E3: the pins and sorts are data. An older kbu's, in config.yaml,
	// move to kinds.yaml in the data directory (after the moves above, which
	// may bring km8's).
	moved = append(moved, config.MoveKindData()...)

	// tdp E3: settings, a state or pins and sorts kbu cannot read are not
	// used; kbu starts on its defaults and the UI asks to move the file
	// aside, or to leave.
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	state, stateErr := config.LoadState()
	if stateErr != nil {
		state = config.DefaultState()
	}
	kinds, kindsErr := config.LoadKindData()

	// tdp E3: values kbu cannot use go back to their defaults, the file
	// copied aside first; a toast at start says it, one for each file and
	// for what moved, stacked (tdp E3-37).
	var start []terminu.Notice
	if cfgErr == nil {
		start = append(start, fitted(config.ConfigPath(), cfg.Misfits(), cfg.Save)...)
	}
	if stateErr == nil {
		start = append(start, fitted(config.StatePath(), state.Misfits(), state.Save)...)
	}
	if kindsErr == nil {
		start = append(start, fitted(config.KindDataPath(), kinds.Misfits(), kinds.Save)...)
	}

	// A kubeconfig kbu cannot read is outside it: kbu opens all the same,
	// panel 2 saying why (tdp E3). A context the kubeconfig no longer has
	// is a Warning; state.yaml gets the one in use at the next write (tdp
	// E3-41).
	client, gone, clientErr := connect(state.Context, cfg.DefaultContext)
	start = append(start, gone...)

	// The saved namespaces go in first; the ones since deleted are dropped
	// once the cluster says which exist.
	switch {
	case client == nil:
	case len(state.Namespaces) > 0:
		client.SetSelection(k8s.SelectedNamespaces(state.Namespaces...))
	case state.Namespace != "":
		client.SetNamespace(state.Namespace)
	case cfg.DefaultNamespace != "":
		client.SetNamespace(cfg.DefaultNamespace)
	}

	// The Helm Releases category is there only when `helm` is on PATH.
	k8s.RegisterHelmIfAvailable()

	ui := tui.New(tui.Options{Client: client, Config: cfg, ConfigErr: cfgErr, State: state, StateErr: stateErr,
		Kinds: kinds, KindsErr: kindsErr, Moved: moved, Start: start, ClientErr: clientErr,
		Status:  theme.DefaultTheme().Status,
		Version: version.Display()})
	if err := ui.Run(); err != nil {
		// The user left at a file kbu could not read: why, and no crash.
		var notRead tui.NotRead
		var notSaved tui.NotSaved
		if errors.As(err, &notRead) || errors.As(err, &notSaved) {
			fail(err.Error())
		}
		// bubbletea catches panics, restores the terminal, then returns
		// tea.ErrProgramPanic.
		path := config.WriteCrashLog(err)
		if errors.Is(err, tea.ErrProgramPanic) {
			fmt.Fprintln(os.Stderr, "kbu: Crashed: "+err.Error())
		} else {
			fmt.Fprintln(os.Stderr, "kbu: "+capital(err.Error()))
		}
		if path != "" {
			fmt.Fprintln(os.Stderr, "kbu: The crash log is "+path)
		}
		os.Exit(1)
	}
}
