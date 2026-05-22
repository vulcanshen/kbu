# km8 — KubeMate

<p align="center">
  <img src="docs/icon.svg" width="128" alt="km8 icon" />
</p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/km8)](https://github.com/vulcanshen/km8/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/km8)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanshen/km8)](https://goreportcard.com/report/github.com/vulcanshen/km8)
[![License](https://img.shields.io/github/license/vulcanshen/km8)](LICENSE)
[![Kubetools](https://img.shields.io/static/v1?label=Curated&message=Kubetools&color=2a7f62)](https://collabnix.github.io/kubetools/#cluster-with-core-cli-tools)

A terminal UI for Kubernetes, inspired by [Lens IDE](https://k8slens.dev/), [lazygit](https://github.com/jesseduffield/lazygit), [lazydocker](https://github.com/jesseduffield/lazydocker), and [k9s](https://github.com/derailed/k9s). Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![km8 screenshot](docs/screenshot.png)

## Features

- **26 built-in resource types + CRD support** -- dynamic discovery of Custom Resources at startup, across Cluster / Workloads / Network / Config / Storage / RBAC / Autoscaling categories
- **Real-time Watch updates** -- resources refresh automatically via Kubernetes Watch API
- **Vim-style navigation** -- `j`/`k`, `u`/`d` page scroll, `gg`/`G`, `/` search
- **3-panel lazygit-style layout** -- numbered sidebar, list, and detail panels with scroll indicator
- **Drill-down navigation** -- Deployment / DaemonSet / StatefulSet / Job → Pods → Containers; CronJob → Jobs; HPA → target workload; PVC → mounting Pods; PDB → protected Pods
- **YAML detail view with syntax highlighting** -- `[YAML]` tab shows the resource serialized exactly like `kubectl get -o yaml`; container drill-down shows the extracted `spec`/`status` for that container
- **Pod log streaming with auto-follow** -- multi-container support with `<container>|<log>` format; the Logs tab sticks to the tail by default (a `▼` marker in `[3] Logs ▼` shows follow is active). Scroll up (`k`/`↑`/`u`/`gg`) to pause and read history; press `G` to catch up and resume following
- **Edit & shell exec via embedded PTY** -- `e` runs `kubectl edit` and `s` runs `kubectl exec -it -- /bin/sh`, both inside an in-app virtual terminal so the editor and shell session never touch the host terminal scrollback. Editor honors `$KUBE_EDITOR` / `$EDITOR` (or `config.yaml editor`)
- **KM8erm internal terminal** -- `T` opens an embedded shell (login shell with full env / cwd) inside km8 — like `ssh localhost` in a popup. Run `kubectl apply -f`, `helm`, anything you'd normally drop out of km8 to do
- **PTY scrollback** -- 10k-line history for all PTY popups (KM8erm, shell exec, edit). `PgUp` / `PgDn` page, `Home` / `End` jump to top / live. Disabled in alt-screen apps (vim, less, htop) so they keep their own paging
- **Colored Pod status** -- `Running` green, `Pending` yellow, `CrashLoopBackOff` / `ImagePullBackOff` / `OOMKilled` red, `Terminating` gray. STATUS column shows the kubectl-equivalent reason, not raw `Pod.Status.Phase`
- **Per-container colored log labels** -- multi-container pods are visually distinguishable line-by-line; stable color per container name
- **Resource deletion** -- `D` with confirmation dialog
- **Search/filter** -- `/` to search in all three panels and in the namespace/context picker popups. Sidebar search also matches category names (e.g. "cluster" expands the Cluster category)
- **Clipboard copy (`y`)** -- copies the focused panel's content via OSC 52 (works through tmux/SSH, no `xclip`/`pbcopy` required)
- **Namespace and context switching** -- `n` / `c`
- **Detail tabs** -- YAML / Events / Logs (Logs tab only for Pods)
- **Long values wrap, never truncate** -- applies to YAML, Events, and Logs; wrap points reflow on panel resize
- **Panel expand** -- `=`/`-` to toggle full-screen Table or Detail panel
- **Theme system** -- drop a `theme.yaml` into config directory to override colors
- **Help & App Log overlays** -- `?` / `!` popup on top of main UI
- **Error notifications** -- status bar badge + status line message
- **Crash logging** -- panics written to the km8 log directory
- **Audit logging** -- every `kubectl edit` and `kubectl delete` recorded to `audit-*.log`

## Installation

### Quick Install (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/km8/main/install.sh | sh
```

### Quick Install (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/vulcanshen/km8/main/install.ps1 | iex
```

### Homebrew (macOS/Linux)

```bash
brew install vulcanshen/tap/km8
```

### Scoop (Windows)

```powershell
scoop bucket add vulcanshen https://github.com/vulcanshen/scoop-bucket
scoop install km8
```

### From source

```bash
go install github.com/vulcanshen/km8/cmd@latest
```

### Uninstall

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/vulcanshen/km8/main/uninstall.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/vulcanshen/km8/main/uninstall.ps1 | iex
```

### Build locally

```bash
git clone https://github.com/vulcanshen/km8.git
cd km8
go build -o km8 ./cmd/
./km8
```

## Quick Start

```bash
km8
```

Connects to your current kubeconfig context. Use `n` to switch namespaces, `c` to switch contexts.

## Key Bindings

### Navigation

| Key | Action |
|---|---|
| `j` / `k` | Move cursor up / down |
| `u` / `d` | Page up / down |
| `gg` / `G` | Jump to top / bottom |
| `1` / `2` / `3` | Switch panel |
| `Tab` | Cycle panels |

### Table (Panel 2)

| Key | Action |
|---|---|
| `/` | Search / filter |
| `Enter` | Drill down |
| `e` | Edit resource via `kubectl edit` (asks for confirmation) |
| `D` | Delete resource (asks for confirmation) |
| `s` | Shell into container via `kubectl exec -it` (asks for confirmation) |

### Detail (Panel 3)

| Key | Action |
|---|---|
| `h` / `l` | Switch tab (YAML / Events / Logs) |
| `=` / `-` | Expand / restore panel |
| `G` | Jump to bottom (on Logs: also resumes follow-tail) |
| `k` / `↑` / `u` / `gg` | Scroll up (on Logs: pauses follow-tail) |

### Global

| Key | Action |
|---|---|
| `n` | Switch namespace (`/` to filter inside the popup) |
| `c` | Switch context (`/` to filter inside the popup) |
| `T` | Open KM8erm (embedded shell — login shell, full env, host cwd) |
| `y` | Copy focused panel content to clipboard (OSC 52) |
| `!` | App log |
| `?` | Toggle help |
| `q` | Quit km8 (asks for confirmation) |
| `Ctrl+C` | Quit km8 immediately (no confirm) |
| `Esc` | Close current modal / overlay |

### PTY popups (KM8erm, edit, shell exec)

| Key | Action |
|---|---|
| `PgUp` / `PgDn` | Scroll history by one page |
| `Home` / `End` | Jump to top of history / back to live |
| Any other key | Snap back to live, key forwards to subprocess |

Scrollback is disabled when a full-screen app (vim, less, htop) takes over the PTY via alt-screen; those keys forward to the app instead so it keeps its own paging.

## Editing Resources

Pressing `e` on a resource runs **`kubectl edit <kind>/<name> -n <ns> --context <ctx>`** inside an embedded PTY popup. Behavior is identical to running the same command in a terminal: strategic merge patch, `resourceVersion` conflict detection, no `last-applied-configuration` annotation side-effect.

The editor is resolved by kubectl itself in this priority order:

1. `$KUBE_EDITOR` (km8 sets this if `editor` is configured in `config.yaml`)
2. `$EDITOR`
3. `vi` (Linux/macOS) or `notepad` (Windows)

When the editor exits, the popup closes and the table refreshes via the resource watch — no manual reload needed.

### Why an embedded PTY?

Earlier versions of km8 ran the editor through `tea.ExecProcess` and applied the result with `kubectl apply -f`. That approach leaked kubectl's confirmation messages into the host terminal's scrollback after quitting km8, and the apply-vs-edit semantic mismatch surprised users coming from `kubectl edit`. The PTY popup keeps everything inside km8 and uses `kubectl edit` directly so behavior is exactly what `kubectl edit` users expect.

### Note for nvim users

If your nvim setup has noticeable shutdown lag inside the popup (LSP attach/detach, plugin teardown), set `editor: "nvim --noplugin"` in `config.yaml` to skip plugin loading for the kubectl-edit session only. Your everyday `nvim` is unaffected.

## Context Isolation

km8 maintains its own **session-local** context. Switching context with `c` inside km8 **does not** modify `~/.kube/config` or the `KUBECONFIG` environment variable in any other terminal.

All `kubectl` subprocesses spawned by km8 (edit, delete, shell exec) receive an explicit `--context <name>` flag, so they always target the cluster km8 is showing — regardless of what `kubectl`'s default context is set to.

This means you can safely run km8 in one terminal while using `kubectl` in another without either session interfering with the other's context.

## Configuration

Config files are in the OS-appropriate config directory. Set `XDG_CONFIG_HOME` to override on any platform:

| OS | Default Path |
|---|---|
| Linux | `$XDG_CONFIG_HOME/km8/` or `~/.config/km8/` |
| macOS | `~/Library/Application Support/km8/` |
| Windows | `%APPDATA%/km8/` |

Logs (crash and audit) are written to the `logs/` subdirectory of the config directory.

### config.yaml

```yaml
default_context: ""      # kubeconfig context (default: current-context)
default_namespace: ""    # namespace filter (default: all namespaces)
editor: ""               # exposed to kubectl as $KUBE_EDITOR
                         # (default: kubectl falls back to $EDITOR → vi / notepad)
```

### theme.yaml

Drop a `theme.yaml` to customize colors. Only override what you need -- unspecified fields keep defaults.

```yaml
sidebar:
  background: ""           # empty = terminal transparent
  foreground: "#cdd6f4"
  selected_bg: "#45475a"
  selected_fg: "#cdd6f4"
  category_fg: "#89b4fa"

table:
  header_bg: "#313244"
  header_fg: "#89b4fa"
  row_fg: "#cdd6f4"
  selected_row_bg: "#45475a"
  selected_row_fg: "#cdd6f4"
  alternating_bg: ""

detail:
  border_color: "#585b70"
  label_fg: "#89b4fa"
  value_fg: "#cdd6f4"
  tab_active_bg: "#45475a"
  tab_active_fg: "#cdd6f4"
  tab_inactive_fg: "#6c7086"

status_bar:
  background: "#181825"
  foreground: "#cdd6f4"
  cluster_fg: "#a6e3a1"
  namespace_fg: "#f9e2af"
  context_fg: "#89b4fa"

status_line:
  background: "#313244"
  foreground: "#a6adc8"

status:
  running: "#a6e3a1"
  pending: "#f9e2af"
  error: "#f38ba8"
  unknown: "#6c7086"
```

## Requirements

- **kubectl** on `$PATH` (for edit, delete, and shell exec)
- A valid **kubeconfig** (`~/.kube/config` or `$KUBECONFIG`)
- A running Kubernetes cluster

## License

[GPL-3.0](LICENSE)
