package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"

	"github.com/vulcanshen/kbu/internal/config"
	"github.com/vulcanshen/kbu/internal/k8s"
	"github.com/vulcanshen/kbu/internal/theme"
	"github.com/vulcanshen/kbu/internal/ui"
	"github.com/vulcanshen/kbu/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("km8 " + version.Display())
		return
	}
	// Suppress k8s client-go / klog output that would corrupt the TUI.
	// logr.Discard() is a true no-op; klog.NewKlogr() would route back through
	// klog itself and risk infinite recursion on certain error paths.
	klog.SetOutput(io.Discard)
	klog.SetLogger(logr.Discard())
	log.SetOutput(io.Discard)

	// v2.0 km8 → kbu rename: one-shot migrate ~/.config/km8 to
	// ~/.config/kbu if the new dir doesn't exist yet. Returns silently
	// when nothing to do (fresh install, or already migrated). A copy
	// failure is fatal — proceeding with an empty new config dir would
	// silently lose the user's pins / sort / theme, worse than exiting
	// with a clear error message they can act on.
	migrateNotice, err := config.MigrateLegacyConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error migrating legacy config dir: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Session state is best-effort: a corrupt or unreadable state file
	// must not lock the user out of km8. LoadState errors are stashed
	// on the state value's LoadError field and surfaced in the App Log
	// once NewAppModel builds it — the user notices the nudge in `!`
	// without paying an exit-code for a stale-position file.
	state, stateErr := config.LoadState()
	if stateErr != nil {
		state = config.DefaultState()
	}

	t, err := theme.LoadTheme(config.ThemePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading theme: %v\n", err)
		os.Exit(1)
	}

	// State wins over config for context / namespace when present —
	// "where the user left off" is a stronger signal than the static
	// default. Missing state falls back to config's DefaultContext /
	// DefaultNamespace as before.
	initialContext := cfg.DefaultContext
	if state.Context != "" {
		initialContext = state.Context
	}
	client, err := k8s.NewClient(initialContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to cluster: %v\n", err)
		os.Exit(1)
	}

	if state.Namespace != "" {
		client.SetNamespace(state.Namespace)
	} else if cfg.DefaultNamespace != "" {
		client.SetNamespace(cfg.DefaultNamespace)
	}

	// Optional Helm Releases category — only registered when `helm` is on PATH.
	k8s.RegisterHelmIfAvailable()

	app := ui.NewAppModel(t, client, cfg, state, stateErr, migrateNotice)

	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		// bubbletea catches panics, restores the terminal, then returns
		// tea.ErrProgramPanic. Handle it separately for a clear crash message.
		if errors.Is(err, tea.ErrProgramPanic) {
			path := config.WriteCrashLog(err)
			fmt.Fprintf(os.Stderr, "\n\x1b[31;1mkm8 crashed!\x1b[0m\n")
			fmt.Fprintf(os.Stderr, "panic: %v\n", err)
			if path != "" {
				fmt.Fprintf(os.Stderr, "crash log: %s\n", path)
			}
			os.Exit(1)
		}
		path := config.WriteCrashLog(err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if path != "" {
			fmt.Fprintf(os.Stderr, "crash log: %s\n", path)
		}
		os.Exit(1)
	}
}
