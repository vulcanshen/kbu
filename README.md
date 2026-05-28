# km8 — KubeMate

<p align="center">
  <img src="docs/icon.svg" width="128" alt="km8 icon" />
</p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/km8)](https://github.com/vulcanshen/km8/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/km8)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanshen/km8)](https://goreportcard.com/report/github.com/vulcanshen/km8)
[![License](https://img.shields.io/github/license/vulcanshen/km8)](LICENSE)
[![Kubetools](https://img.shields.io/static/v1?label=Curated&message=Kubetools&color=2a7f62)](https://collabnix.github.io/kubetools/#cluster-with-core-cli-tools)

**Language**: English · [繁體中文](README-zh_TW.md)

A scout-style Kubernetes TUI built around **Relatives navigation**.

## Demo

### Getting around km8

![basics](docs/demo-basics.gif)

### Navigate Kubernetes by relatives

![relatives](docs/demo-relatives.gif)

### Edit live resources via the Space menu

![yaml-edit](docs/demo-yaml-edit.gif)

### Helm as a first-class resource

![helm](docs/demo-helm.gif)

### TUI + persistent shell in one window

![km8erm](docs/demo-km8erm.gif)

## Four keys to drive km8

| Key | Behavior |
|---|---|
| **`Tab`** | Switch panel focus (or `1` / `2` / `3` directly) |
| **`Enter`** | Drill in / commit a choice |
| **`Space`** | *What can I do here?* — opens a contextual menu or cheatsheet on every panel and every tab |
| **`Esc`** | Back out — pop one drill level / close any popup |

When in doubt, press `Space`. Power-user shortcuts (`Y` YAML / `E` edit / `S` shell / `D` delete / `N` ns / `C` context) exist for speed but every one is also reachable through the `Space` menu — nothing to memorize unless you want it.

## Install

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

### Build locally

```bash
git clone https://github.com/vulcanshen/km8.git
cd km8
go build -o km8 ./cmd/
./km8
```

### Uninstall

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/vulcanshen/km8/main/uninstall.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/vulcanshen/km8/main/uninstall.ps1 | iex
```

## Quick Start

```bash
km8
```

Connects to your current kubeconfig context. Press `Enter` to drill, `Space` for the contextual menu, `Esc` to back out, `Tab` to move between panels.

Inspired by [Lens IDE](https://k8slens.dev/), [lazygit](https://github.com/jesseduffield/lazygit), [lazydocker](https://github.com/jesseduffield/lazydocker), and [k9s](https://github.com/derailed/k9s). Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

---

> The rest of this README is the operations manual — read on if you want the full feature surface, every keybinding, and configuration details.

## Features

- **27 built-in resource types + CRD support** -- dynamic discovery of Custom Resources at startup, across Cluster / Workloads / Network / Config / Storage / RBAC / Autoscaling / Helm categories. The Helm category only registers when the `helm` CLI is on `PATH`
- **Real-time Watch updates** -- resources refresh automatically via Kubernetes Watch API
- **Vim-style navigation** -- `j`/`k`, `u`/`d` page scroll, `gg`/`G`, `/` search
- **3-panel lazygit-style layout** -- numbered sidebar, list, and detail panels with scroll indicator
- **Drill-down navigation** -- Deployment / DaemonSet / StatefulSet / Job → Pods → Containers; CronJob → Jobs; HPA → target workload; PVC → mounting Pods; PDB → protected Pods; Helm Release → each native K8s object the chart deployed
- **Relatives tab — Lens-style navigation** -- every detail panel (except Namespaces) lists the resource's navigable references (owners, selected pods, scaleTargetRef, mounted-by pods, ...). `Enter` drills into the cursor's ref — the panel re-renders showing *that* resource's Relatives, building a chain (Deployment → Pod → ConfigMap → consumer Pods, ...). `Esc` pops one level. `Space` opens a breadcrumb popup so you can jump panels 1+2 back to any chain ancestor (confirms first). Tab label shows `Relatives N` at depth>1. `Y` opens the YAML of whichever entry the cursor is on. Cycle detection blocks revisiting an ancestor; fetch failures toast and stay put. 26 of 27 resource kinds covered — ConfigMaps / Secrets / ServiceAccounts surface *reverse* refs (which Pods use me, which RoleBindings name this SA as a subject, ...); Helm releases surface their `Deployed Resources` so each chart-deployed K8s object is one drill away
- **Helm releases (when `helm` is on `PATH`)** -- a dedicated `Helm > Releases` sidebar category lists every release in the cluster (`helm list -A` polled every 3s; no Helm watch API). Panel 2 columns: `NAME / NAMESPACE / CHART / APP VER / REV / STATUS / UPDATED`. Press `Space` on a release row to open a doc menu (Manifest / Creator Notes / User Values / Merged Values / Hooks); pick one with `Enter` to fetch via `helm get ...` and view the result in the YAML popup. The menu stays open behind the YAML so consecutive docs flow without re-opening. Panel 3 carries a `History` tab in place of Events — table view of every revision (REV / STATUS / DATE / CHART / DESCRIPTION) with the current deployed rev marked `●`. `Space` on a non-current row asks to roll back; confirm shows the exact `helm rollback` command and runs it asynchronously, with the result surfaced as a toast. Helm-managed K8s objects (label `app.kubernetes.io/managed-by: Helm` or annotation `meta.helm.sh/release-name`) are marked with a `` glyph in panel 2 and block `E` (kubectl edit) with a "Helm-managed (read-only)" toast — use `helm upgrade` / `rollback` instead. Press `.` on any non-Releases list to hide all helm-managed objects (panel 2 bottom-left always shows the `.: toggle helm` hint)
- **YAML popup (`Y`)** -- raw `kubectl get -o yaml` of the selected resource in a full-screen overlay with `j/k/u/d/gg/G` scroll, `/` search (`n`/`N` step through matches with full-row highlight), `y` to copy the full YAML to your clipboard, and `E` to dispatch `kubectl edit` directly from the popup. YAML lives in the popup, not the detail panel, so vertical layout no longer wraps long YAML lines awkwardly
- **Pod log streaming with auto-follow** -- multi-container support with `<container>|<log>` format; the Logs tab sticks to the tail by default (a `▼` marker in `[3] Logs ▼` shows follow is active). Scroll up (`k`/`↑`/`u`/`gg`) to pause and read history; press `G` to catch up and resume following
- **Aggregate logs for Deployments** -- selecting a Deployment streams logs from **every pod in the current ReplicaSet** into a single Logs tab (also the default tab for Deployment detail). Lines are prefixed `<pod-hash>│<container>│<text>` with each segment in its own stable color, so during a rollout you can spot at a glance which pod is throwing errors without drill-down. Pods churning during rollout: the stream snapshots at row-select; re-select the Deployment row to refresh. Falls back to Deployment selector when current-ReplicaSet lookup fails (e.g. missing RBAC on ReplicaSet)
- **Edit & shell exec via embedded PTY** -- `E` runs `kubectl edit` and `S` runs `kubectl exec -it -- /bin/sh`, both inside an in-app virtual terminal so the editor and shell session never touch the host terminal scrollback. Editor honors `$KUBE_EDITOR` / `$EDITOR` (or `config.yaml editor`)
- **KM8erm internal terminal** -- `Alt+t` toggles an embedded shell (login shell with full env / cwd) inside km8 — like `ssh localhost` in a popup. Run `kubectl apply -f`, `helm`, anything you'd normally drop out of km8 to do. The shell is **persistent**: pressing `Alt+t` while the popup is visible hides it without killing the shell; pressing it again reattaches (cwd, history, env, background jobs all preserved). A `KM8erm` chip on the right of the status bar shows when the shell is alive in the background. Independent of `kubectl edit` / `kubectl exec` — you can keep KM8erm running while editing a resource or exec'ing into a container in a separate popup
- **PTY popup borders signal kind at a glance** -- KM8erm is orange, `kubectl exec` is green, `kubectl edit` is sky blue. Useful when KM8erm is hidden behind a transient exec/edit popup
- **PTY scrollback** -- 10k-line history for all PTY popups (KM8erm, shell exec, edit). `PgUp` / `PgDn` page, `Home` / `End` jump to top / live. Disabled in alt-screen apps (vim, less, htop) so they keep their own paging
- **Colored Pod status** -- `Running` green, `Pending` yellow, `CrashLoopBackOff` / `ImagePullBackOff` / `OOMKilled` red, `Terminating` gray. STATUS column shows the kubectl-equivalent reason, not raw `Pod.Status.Phase`
- **Per-container colored log labels** -- multi-container pods are visually distinguishable line-by-line; stable color per container name
- **Resource deletion** -- `D` (uppercase, both as a hotkey and via the `Space` menu) with confirmation dialog
- **Search/filter** -- `/` to search in the sidebar and table panels, and in the namespace/context picker popups. Sidebar search also matches category names (e.g. "cluster" expands the Cluster category). Search clears automatically when focus moves to another panel — selection persists, the filter doesn't
- **Clipboard copy (`y`)** -- copies the focused panel's content via OSC 52 (works through tmux/SSH, no `xclip`/`pbcopy` required). Inside the App Log popup (`!`), `y` copies the full log; inside the YAML popup, `y` copies the full YAML
- **Toast notifications with levels** -- info-level (1s sky-blue) for confirmations like "Copied!"; warning-level (2s peach with `󰀦`) for blocked actions like Relatives cycle detection or drill failures
- **Namespace and context switching** -- `N` for namespace, `C` for context (uppercase — trigger keys are uppercase to avoid mis-triggering while typing search queries)
- **Session-local context** -- switching context in km8 doesn't touch `~/.kube/config`. Run `kubectl` in another terminal in parallel without interference
- **Panel-aware selection styling** -- the focused panel's cursor row gets a bright reverse-video highlight; the *unfocused* panel's selected row keeps a softer bg + bold so you can always see which resource each panel "remembers" while you work in another. Pod STATUS uses a darker palette variant when it lands on a light-bg highlighted row so the green/yellow/red stays readable
- **Detail tabs** -- `Relatives` / `Logs` (Pods + Deployments) / `Events` for K8s resources; `Relatives` / `History` for Helm releases. Relatives is always first when present, so `Space` jumps land on the same tab you came from. Panel 3 has no `/` search — cursor tabs (Relatives / History) don't tolerate row filtering, and Logs read better as a plain follow-tail view; use `Y` + your editor to grep large content
- **Long values wrap, never truncate** -- applies to YAML, Events, and Logs; wrap points reflow on panel resize
- **Panel expand** -- `=`/`-` to toggle full-screen Table or Detail panel
- **Theme system** -- drop a `theme.yaml` into config directory to override colors
- **Help & App Log overlays** -- `?` / `!` popup on top of main UI
- **Error notifications** -- status bar badge + status line message
- **Crash logging** -- panics written to the km8 log directory
- **Audit logging** -- every `kubectl edit` and `kubectl delete` recorded to `audit-*.log`

## Key Bindings

### Primary interaction: four keys

Most of the time you're driving km8 with just four keys:

| Key | Behavior |
|---|---|
| **`Tab`** | **Panel** — move focus to the next panel (or use `1` / `2` / `3` to jump directly) |
| **`Enter`** | **Into** — drill into the selected resource / focus the next panel / commit a popup choice |
| **`Space`** | **Menu** — open a contextual popup wherever focus is: sidebar cheatsheet (panel 1), per-row action menu / container Shell menu / empty-list hint (panel 2), Logs / Events / Relatives-drill / Relatives-breadcrumb / History rollback (panel 3 by tab). Also closes any open popup (mirror open) |
| **`Esc`** | **Back** — pop one drill level / close any popup |

Where a contextual menu exists, `Space` is enough — you don't need to memorize the per-action keys. The sidebar (panel 1) doesn't have an action menu because every row is itself a navigation target; `j`/`k` to move, `Enter` to focus into the table.

Tab navigation also responds to `h`/`l` (or `[`/`]`) for switching panel 3 tabs.

### Accelerators — cursor + power triggers

```
 cursor      j k        u d        gg G        / (search inside current panel)
 trigger     Y YAML     E edit     S shell     D delete          N ns    C context
 expand      z          z toggles full-screen on current panel
 helm        .          . toggles helm-managed visibility on panel 2
```

Trigger keys are deliberately uppercase to avoid misfiring while typing in a `/` search field.

### Global

| Key | Action |
|---|---|
| `Alt+t` | Toggle KM8erm (spawn / show / hide; shell stays alive across hide) |
| `y` | Copy focused panel content to clipboard (OSC 52) |
| `!` | App log |
| `?` | Help |
| `q` | Quit km8 (asks for confirmation) |
| `Ctrl+C` | Quit km8 immediately (no confirm) |

### Panel 2 context menu (`Space` on any row)

Per-row menu with resource-aware items — `Y` YAML / `E` Edit / `S` Shell / `D` Delete. Use `j`/`k` + `Enter` or hit the letter directly. Helm-managed rows hide `E`/`D` (Rule A: read-only — edits would be overwritten by `helm upgrade`/`rollback`); resources without containers hide `S`.

### Helm-specific

| Key | Where | Action |
|---|---|---|
| `Space` | Panel 2, Release row | Open the doc menu — pick `Manifest` / `Notes` / `User Values` / `Merged Values` / `Hooks` |
| `Space` | Panel 3, History tab, non-current row | Roll back to that revision (confirm popup shows the exact `helm rollback` command) |
| `.` | Any non-Releases panel 2 list | Toggle visibility of helm-managed objects |

### PTY popups (KM8erm, edit, shell exec)

| Key | Action |
|---|---|
| `PgUp` / `PgDn` | Scroll history by one page |
| `Home` / `End` | Jump to top of history / back to live |
| Any other key | Snap back to live, key forwards to subprocess |

Scrollback is disabled when a full-screen app (vim, less, htop) takes over the PTY via alt-screen; those keys forward to the app instead so it keeps its own paging.

## Editing Resources

Pressing `E` on a resource (or picking `Edit` from the `Space` menu) runs **`kubectl edit <kind>/<name> -n <ns> --context <ctx>`** inside an embedded PTY popup. Behavior is identical to running the same command in a terminal: strategic merge patch, `resourceVersion` conflict detection, no `last-applied-configuration` annotation side-effect.

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

km8 maintains its own **session-local** context. Switching context with `C` inside km8 **does not** modify `~/.kube/config` or the `KUBECONFIG` environment variable in any other terminal.

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
  background: ""                       # empty = terminal transparent
  foreground: "#cdd6f4"
  selected_bg: "#bac2de"               # focused-panel cursor bg (reverse-video)
  selected_fg: "#1e1e2e"
  unfocused_selected_bg: "#353648"     # other-panel "remembered" selection bg
  unfocused_selected_fg: "#cdd6f4"
  category_fg: "#89b4fa"

table:
  header_bg: "#313244"
  header_fg: "#89b4fa"
  row_fg: "#cdd6f4"
  selected_row_bg: "#bac2de"           # focused-panel cursor bg (reverse-video)
  selected_row_fg: "#1e1e2e"
  unfocused_selected_row_bg: "#353648" # other-panel "remembered" selection bg
  unfocused_selected_row_fg: "#cdd6f4"
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
