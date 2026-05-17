# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [v1.0.4] - 2026-05-17

### Added
- Audit log: each session writes `audit-YYYY-MM-DD_HH-MM-SS.log` under the km8 log directory, recording every `kubectl edit` and `kubectl delete` operation with timestamp, resource, namespace, and kubectl output
- `XDG_CONFIG_HOME` support: if set, overrides the platform default config directory (useful for keeping config under `~/.config` on macOS)

### Fixed
- **Critical**: replaced `klog.NewKlogr()` with `logr.Discard()` — the previous logger caused infinite recursion (stack overflow) on certain client-go error paths, crashing the app and corrupting the terminal
- Crash recovery now uses `tea.ErrProgramPanic` so bubbletea correctly restores the terminal before handling the panic
- Fixed TOCTOU race in `Watcher`: both channels are now fetched atomically via `Channels()` to prevent stale channel references after `Start()` recreates them
- Pressing Esc after search in Panel 1 (Sidebar) now returns the cursor to the previously selected resource instead of jumping to index 0
- Pressing Esc after search in Panel 2 (Table) now returns the cursor to the previously selected row instead of jumping to index 0
- Pressing Enter while searching now immediately activates the selected item in both Panel 1 and Panel 2 (previously required two Enter presses)
- Added `go test -race ./...` to CI to catch race conditions automatically

## [v1.0.3] - 2026-05-17

### Added
- Open/close animation for all overlay popups (horizontal line expanding then vertical reveal; reversed on close)
- Hidden km8 logo easter egg — press `K` to show a centered pixel-art logo, `Esc`/`q` to dismiss
- Navigation hotkey hints (`j/k scroll`, `u/d page`, `gg/G top/bot`) added to status line for all panels
- `/ search` hint added to Panel 3 (Detail)

### Changed
- Status line redesigned: removed panel numbers (`[1]/[2]/[3]`), separators (`|`), and colons; keys now styled in bold sapphire with dim descriptions
- Status line auto-wraps to a second row when hints exceed terminal width (drops trailing hints if still too narrow); panel heights adjust accordingly
- Confirm popup width now adapts to content (capped at 70% of screen); long detail strings (e.g. `kubectl exec` commands) word-wrap inside the popup

### Fixed
- Confirm popup right-edge padding when content nearly fills the box

## [v1.0.2] - 2026-05-16

### Changed
- Confirm, namespace picker, and context picker modals now render as overlay popups (background remains visible, consistent with help and app log)
- All popup border color changed to sapphire (#74c7ec, Catppuccin Mocha)
- All popups now have vertical padding for less cramped appearance

## [v1.0.1] - 2026-05-15

### Changed
- Status bar and status line backgrounds removed (terminal transparent)
- Status line text color changed to blue (#89b4fa, matching panel focus color)
- Status line simplified: removed panel titles, kept panel number + hotkeys only
- Panel 2 status line now includes +/- expand hint

### Fixed
- Status line was using StatusBarStyle (white) instead of StatusLineStyle

### Docs
- Added screenshot, badges, and removed ASCII layout from README

## [v1.0.0] - 2026-05-15

### Added
- **CRD support** -- dynamic discovery of Custom Resource Definitions at startup via API server, automatic registration into sidebar under "Custom Resources" category
- **Resource registry pattern** -- centralized ResourceDefinition struct replaces 6 scattered switch statements; adding a new resource type requires a single Register() call
- **Dynamic client** -- k8s.Client now includes dynamic.Interface for CRD fetch/watch/detail
- **Error notifications** -- status bar shows red error badge with unread count; status line shows latest error message; clears on viewing app log
- **Scroll position indicator** -- "X of Y" in bottom-right border of all panels (sidebar, table, detail)
- **Page scroll (u/d)** -- half-page up/down in sidebar, table, detail, and app log
- **Help overlay popup** -- `?` overlays keybinding reference on top of main UI using bubbletea-overlay library
- **App log overlay popup** -- `!` overlays app log on top of main UI with unified format (timestamp + level + message)
- **Sidebar viewport scrolling** -- cursor stays within visible viewport, auto-scrolls at edges; category headers stay visible when scrolling up
- **Debounced API requests** -- 300ms debounce on sidebar resource switching to avoid flooding the API server
- **Crash logging** -- panics captured and written to `~/.config/km8/logs/crash-TIMESTAMP.log` with stack trace
- **Version flag** -- `km8 --version` / `km8 -v`
- **Release pipeline** -- goreleaser v2 config for multi-platform builds (linux/darwin/windows × amd64/arm64), Homebrew tap, Scoop bucket
- **GitHub Actions** -- tag-triggered release workflow with cross-platform tests
- **GPL-3.0 license**

### Changed
- **ResourceType** changed from `int` (iota) to `string` for CRD compatibility
- **Delete key** changed from `d` to `D` (shift+D) to free `d` for page-down
- **Detail tab keys** changed from `[`/`]` to `h`/`l` for consistency with vim motion
- **Search icon** changed from `/` character to nerd font filter icon (U+F0233)
- **App log order** reversed -- newest entries at the top
- **API rate limit** increased from default QPS 5/Burst 10 to QPS 50/Burst 100
- **Watcher lifecycle** -- channels are closed and recreated on each Start() to prevent stale goroutines from stealing data

### Fixed
- Crash on rapid resource switching (stale ResourceDataMsg with wrong type causing type assertion panic)
- Nil pointer crash when ResourceErrorMsg.Err is nil
- Sidebar scroll not working (height was 0 before first View render; now set in layout on WindowSizeMsg)
- Help page content outdated (missing c/!/D/s keys, stale `[`/`]` reference)

## [v0.1.0] - 2026-05-14

### Added
- Go module with Bubble Tea (bubbletea), Bubbles, Lipgloss, and client-go
- K8s client layer: kubeconfig loading, context listing, namespace filtering
- Watch-based real-time resource updates via List+Watch pattern
- Resource fetchers and table columns for 13 resource types (Namespaces, Nodes, Pods, Deployments, DaemonSets, StatefulSets, Jobs, CronJobs, Services, Ingress, ConfigMaps, Secrets, Events)
- RBAC resources: ClusterRoles, ClusterRoleBindings, Roles, RoleBindings
- Sidebar with resource tree categories and vim navigation (j/k/gg/G)
- Table panel with column headers, vim-style scrolling, and auto-updated detail
- Detail panel with structured tabs (Detail/Events/Logs)
- Lazygit-style panel borders with numbered titles [1]/[2]/[3]
- Panel focus system: Tab to cycle, 1/2/3 for direct panel switching
- Namespace picker overlay (n key)
- Context switching with full screen redraw (c key)
- Pod log streaming: multi-container format, Follow mode with TailLines 100
- Search (/) in all 3 panels
- Drill-down navigation with stack-based multi-level support
- Container detail view: image, state, ready, restarts, ports
- YAML edit via `kubectl edit` (e key)
- Container shell exec with confirm popup (s key)
- Resource deletion with confirm popup (d key)
- Help overlay (? key)
- App log overlay (! key)
- +/- to expand/restore detail panel
- Theme system with YAML override
- Config loader with cross-platform paths
- Cross-platform build support (macOS/Linux/Windows)
- 88 programmatic model tests

### Fixed
- Search box width calculation for UTF-8 characters
- ANSI-aware line truncation in panel rendering
- Suppressed k8s client-go klog output corrupting TUI
- Clear screen after shell session instead of on km8 exit
