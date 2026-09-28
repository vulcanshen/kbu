# kbu — KubeUI

<p align="center">
  <img src="docs/icon.svg" width="128" alt="kbu icon" />
</p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/kbu)](https://github.com/vulcanshen/kbu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/kbu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)
[![Kubetools](https://img.shields.io/static/v1?label=Curated&message=Kubetools&color=2a7f62)](https://collabnix.github.io/kubetools/#cluster-with-core-cli-tools)
[![Charm in the Wild](https://img.shields.io/static/v1?label=Listed%20in&message=Charm%20in%20the%20Wild&color=6B5CE7)](https://github.com/charm-and-friends/charm-in-the-wild#cloud-and-devops)

**Language**: English · [繁體中文](README-zh_TW.md)

> [!WARNING]
> **v2.0 rename note.** kbu is the same tool previously released as **km8** (v1.7.x and earlier). Everything you know still works — the command binary is now `kbu`, the config directory moved from `~/.config/km8/` to `~/.config/kbu/` with a one-shot auto-migration on first launch, and `$KM8__*` env vars are still read as a fallback, kept permanently for backward compatibility (see the Environment variables table). Upgrade is drop-in; no manual steps required.

**A single-pane Kubernetes workspace** — `Tab` / `Space` / `Enter` / `Esc` drive everything. No hotkey memorization, no setup, no learning curve. Relatives navigation, YAML compare, and an embedded persistent shell are built in; any other terminal tool you trust rides along through the shell.

> _When in doubt, hit_ **`Space`**.

## Demo

![basics](docs/demo-basics.gif)

## Features

### Find your way around

- **Relatives** -- every resource lists what it is connected to: its owner, the Pods a Service selects, the workload an HPA scales, the Pods mounting a PVC, the Pods using a ConfigMap or Secret. `Enter` follows a link, `Esc` steps back, and `B` (**Breadcrumb**, also in the `Space` menu) takes you to any point in the chain you walked.
- **Drill-down** -- Deployment / StatefulSet / DaemonSet / Job → Pods → Containers, CronJob → Jobs, HPA → its target, Helm release → every object the chart deployed.
- **28 built-in resource types plus your CRDs** -- Custom Resources are discovered at startup, and every list updates live through the Kubernetes Watch API.
- **Multi-namespace view** -- check any set of namespaces in the `N` picker, or pick "All Namespaces". kbu remembers the selection.
- **Pin and sort** -- pin the resource kinds you use most to the top of the sidebar and drag them into order; sort any list by one or more columns. Both are saved per kind.
- **Search** -- `/` filters the sidebar, the resource list, and the namespace / context pickers.
- **Picks up where you left off** -- quit and relaunch, and you're back on the same context, namespace, resource, row, panel, and tab.

### Look closer

- **Logs** -- follows the tail, pauses when you scroll up, `G` to go live again. Select a workload and the logs of **every Pod** it runs stream into one view, each Pod and container in its own color, so during a rollout you can see which one is failing.
- **Events** -- on a workload, its own events are merged with its Pods' events (a CronJob also includes its Jobs), newest first.
- **Conditions** -- `.status.conditions` as a table, the same thing `kubectl describe` shows. Useful after events have expired.
- **YAML viewer (`Y`)** -- a vim-style buffer: move with `hjkl` / `w` / `b`, search with `/`, select with `v`, copy with `y`.
- **Compare (`C`)** -- diff two resources of the same kind, unified or side by side (`L` switches, and kbu keeps your choice for next time). Status and server-managed fields are stripped so you only see what was authored.
- **Problems stand out** -- status columns color only what needs attention: yellow for pending or degraded, red for failures. Healthy rows stay plain.
- **Helm releases** -- when `helm` is on your `PATH`, releases get their own view: manifest, values, notes, and hooks; a revision history with one-key rollback. Objects a chart manages are marked, protected from accidental edits, and can be hidden with `.`.
- **KubeConfig contexts** -- a read-only view of your kubeconfig; `Enter` on a context switches kbu to it. Credentials are never shown.

### Get things done

- **Edit (`E`)** -- runs the real `kubectl edit` in a terminal popup inside kbu, with your `$KUBE_EDITOR` / `$EDITOR`.
- **Shell (`S`)** -- `kubectl exec` into a container, also inside kbu.
- **Delete (`D`)** -- always asks first; deleting a namespace carries an extra warning.
- **Alterm (`Alt-t`)** -- a persistent shell inside kbu. Hide it and bring it back with the same key; directory, history, and running jobs are kept. `Alt-Esc` ends it. Anything kbu doesn't do, your usual tools can do here.
- **Copy (`y`)** -- copies the current row or the whole view to your clipboard via OSC 52, so it works over SSH and tmux too.
- **Audit log** -- every edit and delete made from kbu is recorded.

### Fits your setup

- **Session-local context** -- switching context in kbu never touches `~/.kube/config`; `kubectl` in another terminal is unaffected.
- **Mouse** -- click to focus and select, double-click to drill, right-click for the menu, wheel to scroll. Turn it off in Settings (`>`) if you prefer keyboard only.
- **Full screen (`z`)** -- expand the list or detail panel, `z` again to restore.
- **Themes** -- override any color with a `theme.yaml`.

## Install

### Requirements

- **kubectl** on `$PATH` (for edit, delete, and shell exec)
- A valid **kubeconfig** (`~/.kube/config` or `$KUBECONFIG`)
- A running Kubernetes cluster
- **A Nerd Font**, preferably a Mono variant (e.g. JetBrains Mono Nerd Font Mono) so icons line up with the grid.
- **A truecolor terminal** (24-bit color). kbu's soft colors and the shading between stacked popups can't be told apart in 256 colors.

### Quick Install (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/install.sh | sh
```

### Quick Install (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/install.ps1 | iex
```

### Homebrew (macOS/Linux)

```bash
brew install vulcanshen/tap/kbu
```

### Scoop (Windows)

```powershell
scoop bucket add vulcanshen https://github.com/vulcanshen/scoop-bucket
scoop install kbu
```

Building from source is in [`docs/dev-remarks.md`](docs/dev-remarks.md).

### Uninstall

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.ps1 | iex
```

## Quick Start

```bash
kbu
```

Connects to your current kubeconfig context. Press `Enter` to drill, `Space` for the contextual menu, `Esc` to back out, `Tab` to move between panels.

## Four keys to drive kbu

| Key | Behavior |
|---|---|
| **`Tab`** | Switch panel focus (or `1` / `2` / `3` directly) |
| **`Enter`** | The obvious action for what's selected: drill in (a workload → its pods), open the YAML of a kind that doesn't drill, switch to a kubeconfig context, shell into a container, roll back to a Helm revision. On panel 1 it shows the kind in panel 2; on a Logs / Events / Conditions tab it full-screens the panel |
| **`Space`** | *What can I do here?* — the menu of everything this panel or tab can do, ending with **Global operation** (namespace, context, Alterm, settings, app log, quit) |
| **`Esc`** | Back out — pop one drill level / close any popup |

When in doubt, press `Space`. Power-user shortcuts (`P` pin / `S` sort or shell / `D` drag-pin or delete / `Alt-S` panel-2 sort / `C` compare or context / `Y` YAML / `E` edit / `N` ns / `>` settings) exist for speed — every one is also a row of the `Space` menu (the app-wide ones under its **Global operation** row), so nothing's required to memorize unless you want it. A row that can't run right now is shown dimmed rather than hidden. When a popup opens over another, only the one you're in is bright; everything beneath it dims.

**Mouse works too**: left-click focuses a panel and moves the cursor, double-click drills, right-click opens the same context menu as `Space`, and the wheel scrolls half-page. Press `>` to open the Settings popup if you want to flip mouse off and stay keyboard-only.

## Key Bindings

`h` / `l` (or `[` / `]`) switch between panel 3's tabs.

### Shortcuts

Everything below is also in the `Space` menu -- these are just faster.

```
 cursor    j k         u d         gg G        / (search inside current panel)
 trigger   Y YAML      E edit      N namespace
 panel 1   P pin       S sort      D drag-and-drop pinned (modal)    C context
 panel 2   S shell     Alt-S sort          D delete    C compare anchor
 expand    z           z toggles full-screen on current panel
 helm      .           . toggles helm-managed visibility on panel 2
 settings  >           > (Shift-.) opens the global Settings popup
```

`S`, `C`, and `D` do different things depending on the focused panel. Shortcut keys are uppercase so they don't fire while you type in a search field.

### Global

| Key | Action |
|---|---|
| `>` | Open the global Settings popup (mouse on/off, scroll direction; future settings) |
| `Alt-t` | Toggle Alterm (spawn / show / hide; shell stays alive across hide) |
| `y` | Copy focused element to clipboard (OSC 52) -- cursor row when the focus has one, whole content otherwise |
| `!` | App log |
| `?` | The keys of whatever is in front — the focused panel, or the menu, popup or mode on top. `?` again or `Esc` closes it |
| `q` | Quit kbu (saves session state on the way out) -- from any panel, menu or popup; while you type in a search it is just a letter |
| `Ctrl-C` | Same as `q`, and it also works while you type |

`N`, `C`, `Alt-t`, `>`, `!` and `q` are also rows of the global operation popup: the last row of every `Space` menu.

### Mouse

| Gesture | Behavior |
|---|---|
| **Left-click** on a panel row | Focus that panel + move the cursor to the clicked row |
| **Double-click** | Synthesizes `Enter` on panels 2 and 3; on panel 1 it only selects |
| **Right-click** on a row | Synthesizes `Space` (opens the `Space` menu for that row) |
| **Wheel up / down** | Synthesizes `u` / `d` (half-page move). Direction can be flipped via Settings popup (`scroll_direction: natural | reverse`) |
| **Left-click** inside a list popup | Commits that row (same as cursor + `Enter`) |
| **Right-click** inside any popup | Closes it (same as `Esc`) |

Mouse can be disabled in the Settings popup (`>`); the popup itself stays mouse-reachable in that state so you can flip it back on.

### Helm-specific

| Key | Where | Action |
|---|---|---|
| `Space` | Panel 2, Release row | The `Space` menu lists the release's documents next to `YAML` — `Manifest` / `Creator Notes` / `User Values` / `Merged Values` / `Hooks`; each opens over the menu, so you can read several in a row |
| `Enter` | Panel 3, History tab | Roll back to that revision (also in the `Space` menu; nothing on the deployed revision); the confirm shows the exact `helm rollback` command |
| `.` | Any non-Releases panel 2 list | Toggle visibility of helm-managed objects |

### PTY popups (Alterm, edit, shell exec)

| Key | Action |
|---|---|
| `Alt-t` | Alterm: hide it (the shell keeps running) |
| `Alt-Esc` | Close the terminal — asks, then ends the session: Alterm's shell and anything running in it stop; an unsaved `kubectl edit` is dropped |
| `PgUp` / `PgDn` | Scroll history by one page |
| `Home` / `End` | Jump to top of history / back to live |
| Any other key | Snap back to live, key forwards to subprocess |

Scrollback is disabled when a full-screen app (vim, less, htop) takes over the PTY via alt-screen; those keys forward to the app instead so it keeps its own paging.

The terminal fills the screen, less one column and one row on each side; other popups are at most 120 columns wide.

## Editing Resources

Pressing `E` on a resource or in its YAML viewer (or picking `Edit` from the `Space` menu) asks first, then runs **`kubectl edit <kind>/<name> -n <ns> --context <ctx>`** inside an embedded PTY popup. Behavior is identical to running the same command in a terminal: strategic merge patch, `resourceVersion` conflict detection, no `last-applied-configuration` annotation side-effect.

The editor is resolved by kubectl itself in this priority order:

1. `$KUBE_EDITOR` (kbu sets this if `editor` is configured in `config.yaml`)
2. `$EDITOR`
3. `vi` (Linux/macOS) or `notepad` (Windows)

When the editor exits, the popup closes and the table refreshes via the resource watch — no manual reload needed.

### Note for nvim users

If your nvim setup has noticeable shutdown lag inside the popup (LSP attach/detach, plugin teardown), set `editor: "nvim --noplugin"` in `config.yaml` to skip plugin loading for the kubectl-edit session only. Your everyday `nvim` is unaffected.

## Context Isolation

kbu maintains its own **session-local** context. Switching context with `C` inside kbu **does not** modify `~/.kube/config` or the `KUBECONFIG` environment variable in any other terminal.

All `kubectl` subprocesses spawned by kbu (edit, delete, shell exec) receive an explicit `--context <name>` flag, so they always target the cluster kbu is showing — regardless of what `kubectl`'s default context is set to.

This means you can safely run kbu in one terminal while using `kubectl` in another without either session interfering with the other's context.

## Configuration

Config files are in the OS-appropriate config directory. Set `XDG_CONFIG_HOME` to override on any platform:

| OS | Default Path |
|---|---|
| Linux | `$XDG_CONFIG_HOME/kbu/` or `~/.config/kbu/` |
| macOS | `~/Library/Application Support/kbu/` |
| Windows | `%APPDATA%/kbu/` |

Logs (crash and audit) are written to the `logs/` subdirectory of the config directory.

kbu also keeps a `state.yaml` next to `config.yaml` to remember where you left off. It is managed automatically; edit `config.yaml` instead.

### config.yaml

```yaml
default_context: ""      # kubeconfig context (default: current-context)
default_namespace: ""    # namespace filter (default: all namespaces)
editor: ""               # exposed to kubectl as $KUBE_EDITOR
                         # (default: kubectl falls back to $EDITOR → vi / notepad)
alterm_shell: ""         # shell launched by Alterm (default: $SHELL → /bin/sh).
                         # Bare names are resolved via $PATH at popup-open time
                         # (Go exec.Command semantics); absolute paths are used
                         # verbatim. Lets you pick e.g. fish inside alterm
                         # while keeping zsh as host shell.
alterm_login_shell: false # true launches Alterm with `-l` so it sources
                         # ~/.zprofile / ~/.bash_profile / /etc/profile.
                         # Turn on if kbu starts from a launcher (Raycast,
                         # Alfred, ...) and your PATH lives in .zprofile.

# Compare popup defaults. `layout` picks the diff render —
# "unified" (default) is a single column with -/+ markers,
# "split" is side-by-side. Pressing `L` in the popup writes
# your choice back here.
compare:
  layout: unified

# Mouse settings. Both fields optional; omitting either
# falls back to the defaults below.
mouse_opt_config:
  enabled: true                # set false to disable click + double-click + right-click + wheel
  scroll_direction: natural    # "natural": wheel-up = cursor up. "reverse" swaps the mapping.

# Per-kind preferences. Keyed by kubectl name
# ("pod" / "deployment" / "configmap" / ...). Each entry is
# optional and unknown kinds are preserved across the rewrite,
# so a CRD that briefly uninstalls won't lose its pin / sort.
#
# Sort is a multi-tier chain: tier 0 is the primary,
# tier 1 the first tiebreaker, etc.
resource_kind_config:
  pod:
    pinned:
      order: 10              # sparse — increments of 10, so manual YAML
                             # tweaks can wedge a kind between two existing pins
    sort:
      - column: Restarts     # column title from the kind's panel-2 columns
        direction: desc      # "asc" or "desc"
      - column: Name         # tier 1 — tiebreaker when Restarts is equal
        direction: asc
  configmap:
    pinned:
      order: 20
    sort:                    # single-tier chain also valid
      - column: Age
        direction: desc
```

### Environment variables

Override the corresponding config slot for one-shot runs without editing the YAML — useful for CI / scripted demos / quick "try this shell" sessions.

> **v2.0 rename note.** The `KBU__*` names below replaced the pre-v2.0 `KM8__*` names. The old `KM8__*` names are still read as a fallback (kept permanently for backward compatibility) — a `KM8__CONFIGPATH` in your `~/.zshrc` from a v1.7.x install keeps working. If both a `KBU__` and its legacy `KM8__` counterpart are set, `KBU__` wins.

| Variable | Effect | Precedence |
|---|---|---|
| `KBU__CONFIGPATH` | Use this file as the config file instead of the default layout (`$XDG_CONFIG_HOME/kbu/config.yaml` etc.). Theme file path is NOT affected — it still lives under the OS config directory. Absolute path recommended; relative path resolves against CWD at load/save time. | `KBU__CONFIGPATH` > default layout |
| `KBU__STATEPATH` | Use this file as the session state file instead of `<config-dir>/state.yaml`. Same TrimSpace-then-empty-check pattern as `KBU__CONFIGPATH`. Handy for sandbox / test runs where you want per-run state without touching the real state file. | `KBU__STATEPATH` > default layout |
| `KBU__ALTERM_SHELL` | Use this binary as the Alterm shell. Bare names are looked up on `$PATH` at popup-open time (Go `exec.Command` semantics); absolute paths run verbatim. Leading / trailing whitespace is trimmed. | `KBU__ALTERM_SHELL` > `alterm_shell` config > `$SHELL` > `/bin/sh` |
| `KBU__ALTERM_LOGIN_SHELL` | Force the Alterm shell into login mode (`-l`) or out of it. Truthy values: `true` / `1` / `yes` (and uppercase). Any other value disables login mode. Use when launched from a non-login parent and your PATH is set in `.zprofile`. | `KBU__ALTERM_LOGIN_SHELL` > `alterm_login_shell` config > `false` |

Example:

```sh
# Try fish in Alterm without editing config.yaml
KBU__ALTERM_SHELL=/opt/homebrew/bin/fish kbu

# Point kbu at a per-project config (e.g. checked into the repo)
KBU__CONFIGPATH="$PWD/.kbu.yaml" kbu
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
  header_bg: ""                        # empty = sits on the panel canvas, fg alone signals header
  header_fg: "#89b4fa"
  row_fg: "#cdd6f4"
  selected_row_bg: "#bac2de"           # focused-panel cursor bg (reverse-video)
  selected_row_fg: "#1e1e2e"
  unfocused_selected_row_bg: "#b4befe" # other-panel "remembered" selection bg — Catppuccin lavender chip
  unfocused_selected_row_fg: "#1e1e2e"
  alternating_bg: ""

detail:
  border_color: "#585b70"
  label_fg: "#89b4fa"
  value_fg: "#cdd6f4"
  tab_active_bg: "#45475a"
  tab_active_fg: "#cdd6f4"
  tab_inactive_fg: "#7f849c"

status_bar:
  background: ""                       # empty = terminal transparent
  foreground: "#cdd6f4"
  context_fg: "#89b4fa"

status_line:
  background: ""                       # empty = terminal transparent
  foreground: "#89b4fa"

status:
  running: "#a6e3a1"
  pending: "#f9e2af"
  error: "#f38ba8"
  unknown: "#7f849c"
```

## Limits

- **Use the Mono variant of your Nerd Font.** With a proportional variant, or a terminal set to East-Asian-Ambiguous=double (some tmux + iTerm2 CJK setups), helm-managed rows and popup top borders may sit 1 cell off the grid. Switch to the Mono variant or set ambiguous-width to single.
- **Helm needs the `helm` CLI.** The Helm category only appears when `helm` is on your `PATH`, and the release list refreshes every 3 seconds (Helm has no watch API).
- **Workload logs follow the Pods that exist when you select the row.** After a rollout, select the row again to pick up the new Pods.
- **Some deletes and edits are blocked.** Events and Nodes can't be deleted from kbu; helm-managed objects can't be edited or deleted — use `helm upgrade` / `rollback` / `uninstall`.
- **Panel 3 has no `/` search.** Open the YAML with `Y` and search there.

## Links

- [CHANGELOG.md](CHANGELOG.md) — what changed in each release
- [`docs/dev-remarks.md`](docs/dev-remarks.md) — the developer's notes: how it works, why, building and testing

## terminu family

kbu follows the [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.13/principle): the same keys and the same menus as the rest of the family — [filu](https://github.com/vulcanshen/filu) (files), [sshu](https://github.com/vulcanshen/sshu) (ssh), [webu](https://github.com/vulcanshen/webu) (the web) and [locku](https://github.com/vulcanshen/locku) (screen lock).

## License

[GPL-3.0](LICENSE)
