package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-logr/logr"
	terminu "github.com/vulcanshen/terminu-go"
	"k8s.io/klog/v2"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
	"github.com/vulcanshen/kbu/internal/tui"
	"github.com/vulcanshen/kbu/internal/version"
)

// usage is what `kbu help` prints (tdp E1).
const usage = `kbu — a Kubernetes TUI inspired by Lens IDE

Usage:
  kbu              start it, on your kubeconfig's context
  kbu version      print the version (also --version, -v)
  kbu help         print this (also --help, -h)
  kbu iconwidth    print how many cells a Nerd Font icon takes here

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

func main() {
	switch {
	case asksVersion(os.Args[1:]):
		fmt.Println("kbu " + version.Display())
		return
	case asksHelp(os.Args[1:]):
		fmt.Print(usage)
		return
	case len(os.Args) > 1 && os.Args[1] == "iconwidth":
		fmt.Println(terminu.Probe("kbu").IconCells)
		return
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

	// v2.0 km8 → kbu rename: one-shot copy of the km8 settings directory
	// when kbu's does not exist yet. A copy failure is fatal — going on with
	// an empty directory would silently lose the user's pins and sorts.
	notices := []string{}
	migrated, err := config.MigrateLegacyConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error migrating legacy config dir: %v\n", err)
		os.Exit(1)
	}
	notices = append(notices, migrated)
	// tdp E3: the pins and sorts are data. An older kbu's, in config.yaml,
	// move to kinds.yaml in the data directory (after the copy above, which
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

	// State wins over config for context / namespace when present — "where
	// the user left off" is a stronger signal than the static default.
	initialContext := cfg.DefaultContext
	if state.Context != "" {
		initialContext = state.Context
	}
	client, err := k8s.NewClient(initialContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to cluster: %v\n", err)
		os.Exit(1)
	}

	// The saved namespaces go in first; the ones since deleted are dropped
	// once the cluster says which exist.
	switch {
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
		Kinds: kinds, KindsErr: kindsErr, Notices: notices, Moved: moved, Status: theme.DefaultTheme().Status, Version: version.Display()})
	if err := ui.Run(); err != nil {
		// The user left at a file kbu could not read: why, and no crash.
		var notRead tui.NotRead
		if errors.As(err, &notRead) {
			fmt.Fprintf(os.Stderr, "kbu: %v\n", err)
			os.Exit(1)
		}
		// bubbletea catches panics, restores the terminal, then returns
		// tea.ErrProgramPanic.
		path := config.WriteCrashLog(err)
		if errors.Is(err, tea.ErrProgramPanic) {
			fmt.Fprintf(os.Stderr, "\n\x1b[31;1mkbu crashed!\x1b[0m\npanic: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		if path != "" {
			fmt.Fprintf(os.Stderr, "crash log: %s\n", path)
		}
		os.Exit(1)
	}
}
