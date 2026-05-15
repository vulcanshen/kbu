package main

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/klog/v2"

	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
	"github.com/vulcanshen/km8/internal/ui"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("km8 " + Version)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			path := config.WriteCrashLog(r)
			fmt.Fprintf(os.Stderr, "km8 crashed: %v\n", r)
			if path != "" {
				fmt.Fprintf(os.Stderr, "crash log: %s\n", path)
			}
			os.Exit(1)
		}
	}()

	// Suppress k8s client-go / klog output that would corrupt the TUI.
	klog.SetOutput(io.Discard)
	klog.SetLogger(klog.NewKlogr().V(100)) // effectively disable all logging
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

	app := ui.NewAppModel(t, client)

	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		path := config.WriteCrashLog(err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if path != "" {
			fmt.Fprintf(os.Stderr, "crash log: %s\n", path)
		}
		os.Exit(1)
	}
}
