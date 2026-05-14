# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**km8** is a Kubernetes TUI management tool inspired by Lens IDE's UI design, built as a terminal-native alternative. Uses Go + Bubble Tea (bubbletea) for the terminal interface, with a focus on vim-style navigation and a panel-based layout.

## Build & Test

```bash
go build .                    # build binary → ./km8
go test ./...                 # run all tests (includes TUI model tests)
go test ./internal/ui/ -v     # TUI component tests (programmatic, no TTY needed)
go test ./internal/k8s/ -v    # K8s client tests (needs cluster connection)
go vet ./...                  # static analysis
gofmt -l .                    # check formatting
```

## Architecture

```
cmd/             → main entry point
internal/
├── ui/          → Bubble Tea models (layout, sidebar, table, detail, dock, keybindings)
├── k8s/         → Kubernetes client layer (client-go wrapper, resource fetchers)
├── config/      → Config loading (~/.config/km8/config.yaml)
└── theme/       → lipgloss styles and color definitions
```

**Framework stack:**
- [bubbletea](https://github.com/charmbracelet/bubbletea) — Elm-architecture TUI framework
- [bubbles](https://github.com/charmbracelet/bubbles) — Reusable TUI components (table, list, viewport)
- [lipgloss](https://github.com/charmbracelet/lipgloss) — Terminal styling/layout
- [client-go](https://github.com/kubernetes/client-go) — Official K8s Go client

**Testing approach:**
- TUI logic is tested programmatically via bubbletea's Model-Update-View pattern
- Send `tea.KeyMsg` to model, assert on resulting model state — no TTY needed
- Use `teatest` package for integration-level TUI tests
- K8s client layer tested against live cluster (OrbStack)

## Key Design Decisions

- **Bubble Tea over tview**: tview's event handling is opaque and untestable without a real terminal. Bubble Tea's Elm architecture allows full programmatic testing of keybindings and UI state transitions.
- **Vim motion first**: All navigation is vim-style by default (h/j/k/l, gg/G, /, Esc)
- **Panel focus model**: Ctrl+h/j/k/l or Tab to move focus between panels
- **Read + Edit only**: No delete operations in v1 — safety first

## Development Environment

- **Go:** 1.26+ (darwin/arm64)
- **K8s:** OrbStack K8s — local cluster for testing
- **OS:** macOS (Apple Silicon), targeting macOS/Linux/Windows

## Language

The project owner's primary language is Traditional Chinese (繁體中文). Prefer responding in Traditional Chinese unless the context requires English (code, commits, config files).
