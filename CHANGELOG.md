# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [v1.0.6] - 2026-05-18

### Added
- Easter egg (`K`): logo now reveals with a random pixel-by-pixel animation (~500 ms); each trigger produces a different order. "K M 8" caption and dim close hint appear after animation completes. `K` is intentionally absent from the help overlay.

### Fixed
- **kubectl context safety**: edit (`e`), delete (`D`), and shell exec (`s`) now pass `--context <name>` to every `kubectl` subprocess. Previously they used the ambient kubeconfig `current-context`, which could differ from what km8 was showing after a context switch.
- **Watch reconnection goroutine leak**: each namespace or context switch previously leaked one goroutine that persisted until the next switch; after N switches there were N+1 goroutines waiting on the same channel. Fixed by checking `ok` on channel receive — closed-channel wakeups return `nil` and break the chain.
- **Watch recursive stack growth**: the watch reconnection loop was implemented as a recursive `run()` call; K8s closes watch streams every ~5–10 minutes, so the goroutine stack grew by one frame per reconnect indefinitely. Replaced with an explicit outer loop.
- **Context-canceled log noise**: rapidly switching resources cancelled the previous watch context mid-flight, which non-deterministically logged a spurious `ERR watching <resource>: context canceled`. Now silently discarded when the context was intentionally cancelled.
- **Init log messages discarded**: "km8 started" and "connected to …" were written inside `Init()` which has a value receiver — the mutations were silently thrown away. Moved to an `appInitMsg` dispatched through `Update()`.
- **Shell exec success badge**: exiting a shell (`s`) showed a green "✓ applied" badge and wrote an empty audit entry. Fixed by returning `EditDoneMsg{Output: "no changes"}` to suppress both side-effects.
- **Splash animation never started**: `SplashModel.Show()` was changed to return a `tea.Cmd` for the first tick, but the call site in `app.go` discarded the return value, so the pixel-reveal animation never ran.
- **Duplicate edit / stale badge / double renderAllLines** (carried from main, not previously released): `editing` flag prevents concurrent edits; success badge timer carries a generation ID to prevent stale timers from clearing a newer badge; `maxScrollOffset()` no longer calls `renderAllLines()` to avoid double computation.

### Documentation
- Added **Editing Resources** section to README: three-step flow (get → editor → apply), editor resolution order, comparison table vs `kubectl edit` (declarative apply vs strategic merge patch, no `resourceVersion` check, `last-applied-configuration` annotation caveat, Helm/operator warning).
- Added **Context Isolation** section to README: km8 context is session-local, does not modify `~/.kube/config`, all kubectl subprocesses use `--context`.

## [v1.0.5] - 2026-05-18

### Changed
- **kubectl edit rewritten as get → edit → apply flow**: the editor now opens a local temp file instead of running `kubectl edit`, so vim/nvim works correctly with no "Edit cancelled" message leaking into the terminal. Changes are applied with `kubectl apply -f` after the editor exits; if the file is unchanged the apply is skipped entirely.
- Editor resolution order: `config.yaml editor:` → `$VISUAL` → `$EDITOR` → `vi` (macOS/Linux) / `notepad` (Windows). Editor strings with arguments (e.g. `code --wait`) are handled correctly.
- Audit and crash logs now rotate daily (`audit-YYYY-MM-DD.log`, `crash-YYYY-MM-DD.log`) instead of per-session; multiple events in a day append to the same file.
- App log entries are now word-wrapped at the popup width instead of truncated; scrolling is line-based so wrapped entries scroll smoothly. Maximum entry length capped at 300 characters; log buffer increased to 1000 entries.

### Added
- Status bar shows a green `✓ applied` badge for 2 seconds after a successful `kubectl apply`
- App log `OK` level (green) for successful operations; `WARN` (yellow) for editor crash or apply failure
- Apply failure triggers both an app log warning and a status bar error notification

### Fixed
- Editor crash (non-zero exit code) is now caught: the temp file is cleaned up and no apply is attempted

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
