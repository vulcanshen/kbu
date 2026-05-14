# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [0.1.0] - 2026-05-14

### Added
- Go module with Bubble Tea (bubbletea), Bubbles, Lipgloss, and client-go
- K8s client layer: kubeconfig loading, context listing, namespace filtering
- Watch-based real-time resource updates via List+Watch pattern
- Resource fetchers and table columns for all 13 resource types (Namespaces, Nodes, Pods, Deployments, DaemonSets, StatefulSets, Jobs, CronJobs, Services, Ingress, ConfigMaps, Secrets, Events)
- Sidebar with resource tree categories and vim navigation (j/k/gg/G/h/l)
- Table panel with column headers, vim-style scrolling, and auto-updated detail on cursor change
- Detail panel with structured tabs (Detail/Events/Logs) and dynamic tab visibility per resource type
- Lazygit-style panel borders with numbered titles [1]/[2]/[3]
- Panel focus system: Tab to cycle, 1/2/3 for direct panel switching
- Namespace picker overlay (n key)
- Context switching with full screen redraw (c key)
- Pod log streaming: multi-container format (`container │ log`), Follow mode with TailLines 100
- Table search/filter: / to search, real-time case-insensitive filter, Esc to clear
- Search (/) in all 3 panels: sidebar filters resources, table filters rows, detail filters content
- Drill-down navigation: Deployment/DaemonSet/StatefulSet/Job → Pods, CronJob → Jobs, Pod → Containers
- Stack-based multi-level drill-down with Esc to go back and breadcrumb titles
- Container detail view: image, state, ready, restarts, ports
- YAML edit via `kubectl edit` using $EDITOR (e key)
- Container shell exec with confirm popup and colored header (s key)
- Resource deletion with confirm popup and background kubectl delete (d key)
- Help overlay with keybinding reference (? key)
- App log overlay for operation history and errors (! key)
- +/- to expand/restore detail panel to full screen
- Status bar: cluster, namespace, context display
- Status line: context-sensitive keybinding hints
- Responsive layout with WindowSizeMsg handling and fixed proportional panels
- Mouse scroll support
- Bordered search box shared across all panels
- Theme system: default theme with ~/.config/km8/theme.yaml override support
- Config loader: ~/.config/km8/config.yaml with cross-platform paths
- Cross-platform build support (macOS/Linux/Windows)
- 38+ programmatic model tests for TUI logic and keybindings

### Fixed
- Search box width calculation for UTF-8 characters (lipgloss.Width vs len)
- ANSI-aware line truncation in panel rendering
- Column width calculation to prevent row wrapping
- Suppressed k8s client-go klog/stderr output corrupting TUI
- Esc clears active search filter before exiting drill-down
- Clear screen after shell session instead of on km8 exit
- Events resource shows only Detail tab (no redundant Events tab)
- Sidebar scroll logic removed (full-height sidebar fits all items)
