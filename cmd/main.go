package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vulcanshen/km8/internal/config"
	"github.com/vulcanshen/km8/internal/k8s"
	"github.com/vulcanshen/km8/internal/theme"
	"github.com/vulcanshen/km8/internal/ui"
)

func main() {
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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
