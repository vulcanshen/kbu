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

	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
	"github.com/vulcanshen/km8/internal/ui"
	"github.com/vulcanshen/km8/internal/version"
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	t, err := theme.LoadTheme(config.ThemePath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading theme: %v\n", err)
		os.Exit(1)
	}

	client, err := k8s.NewClient(cfg.DefaultContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to cluster: %v\n", err)
		os.Exit(1)
	}

	if cfg.DefaultNamespace != "" {
		client.SetNamespace(cfg.DefaultNamespace)
	}

	// Optional Helm Releases category — only registered when `helm` is on PATH.
	k8s.RegisterHelmIfAvailable()

	app := ui.NewAppModel(t, client, cfg.Editor)

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
