# km8 — KubeMate

<p align="center">
  <img src="docs/icon.svg" width="128" alt="km8 icon" />
</p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/km8)](https://github.com/vulcanshen/km8/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/km8)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/vulcanshen/km8)](https://goreportcard.com/report/github.com/vulcanshen/km8)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)
[![Kubetools](https://img.shields.io/static/v1?label=Curated&message=Kubetools&color=2a7f62)](https://collabnix.github.io/kubetools/#cluster-with-core-cli-tools)
[![Charm in the Wild](https://img.shields.io/static/v1?label=Listed%20in&message=Charm%20in%20the%20Wild&color=6B5CE7)](https://github.com/charm-and-friends/charm-in-the-wild#cloud-and-devops)

**Language**: English · [繁體中文](README-zh_TW.md)

**A single-pane Kubernetes workspace** — `Tab` / `Space` / `Enter` / `Esc` drive everything. No hotkey memorization, no setup, no learning curve. Relatives navigation, YAML compare, and an embedded persistent shell are built in; any other terminal tool you trust rides along through the shell.

> _When in doubt, hit_ **`Space`**.

## Demo

### Getting around km8

![basics](docs/demo-basics.gif)

### Navigate Kubernetes by relatives

![relatives](docs/demo-relatives.gif)

### Edit live resources via the Space menu

![yaml-edit](docs/demo-yaml-edit.gif)

### Diff two resources side-by-side

![compare](docs/demo-compare.gif)

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

When in doubt, press `Space`. Power-user shortcuts (`P` pin / `S` sort or shell / `D` drag-pin or delete / `Alt+Shift+S` panel-2 sort / `C` compare or context / `Y` YAML / `E` edit / `N` ns / `>` settings) exist for speed — every one is also reachable through the `Space` menu, so nothing's required to memorize unless you want it.

**Mouse works too**, since v1.6: left-click focuses a panel and moves the cursor, double-click drills, right-click opens the same context menu as `Space`, and the wheel scrolls half-page. Press `>` to open the Settings popup if you want to flip mouse off and stay keyboard-only.

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

- **Zero learning curve** -- every action surfaces through the `Space` menu. Power-user hotkeys (`P` pin / `S` sort / `C` compare / `Y` YAML / `E` edit / `N` ns / `>` settings / ...) exist for speed but you can ignore the whole cheat sheet — `Space` walks you through the same menus, in context, every time. Onboarding doc: *"When in doubt, hit Space."*
- **Compose, don't replace** -- KM8erm (the embedded persistent shell, `Alt+t`) means any other terminal tool you'd normally drop out of km8 for rides along inside it. Use km8 for navigation and inspection; use whatever you trust for write-side operations — no scrollback split, no context switch, and KM8erm keeps env / cwd / shell history intact across `Alt+t` toggles
- **Pinned resource kinds (`P` + `D` drag-and-drop)** -- panel 1's sidebar grows a Pinned section at the top. `P` on any resource row toggles pin / unpin, and the order persists into the config file. Pins **move** rather than duplicate — a pinned kind disappears from its original category and reappears under Pinned, so each kind has exactly one home. With two or more pinned kinds, press `D` on a pinned row to enter modal drag-and-drop: `j`/`k` swap the locked kind with its neighbour, `Enter` or `D` commits the new order, `Esc` and anything else reverts to the snapshot taken at entry. The header reads `Pinned 󰩐 [D]rop` while dragging, the dragged row paints lavender, and a sticky toast carries the keyboard contract; `Space` mid-drag opens a trimmed drop-only menu if the contract slips out of memory. Pin / sort / future-per-kind settings share the same per-kind config block, so a CRD that briefly goes away (operator reinstall, etc.) keeps its pin and sort silently and restores both the moment it comes back
- **YAML Compare popup (`C`)** -- panel 2 row-level diff. `C` on a row marks it as the **compare anchor** (status-bar glyph shows which row is locked); `C` on a different row of the same kind opens a side-by-side or unified YAML diff. `C` on the anchor itself cancels — the same key toggles all three states (mark / diff / cancel). The diff popup has its own action menu (`Space`) to switch layout live, and the default layout (Unified) is persisted in config. Compare YAML is pre-cleaned (status / managedFields / resourceVersion / uid stripped) so the diff focuses on what the user actually authored
- **List-view sort (`S` on sidebar, `Alt+Shift+S` on panel 2)** -- per-kind multi-column sort persisted across restarts. Pick a column → direction → the picker loops back to the column step so additional tiers can be stacked without re-invoking the flow. Each tier renders its priority and direction in the panel-2 header (`Name (1) ↑ · Restarts (2) ↓ …`); single-tier chains collapse to just the arrow to keep the simple case visually quiet. Reset row at the bottom drops the entire chain in one shot; per-column `Unset` removes a single tier from the direction step. `Esc` is the only way out — the picker never closes itself between operations. Comparators are type-aware: `Age` / `Updated` use the underlying timestamp (not the rendered "5d3h" string); `Ready` parses "N/M" as a pair of ints; `Restarts`, `Desired`, `Current`, `Up-to-date`, `Available`, `Active`, `Rev` use the int form so "10" sorts above "2". Unknown columns silently skip so a stale config doesn't break the sort. No saved sort = `(namespace, name)` ascending, matching kubectl's cross-namespace default
- **Mouse support** -- click a panel to focus it + move the cursor, double-click to drill (synthesizes `Enter`), right-click to open the row's context menu (synthesizes `Space`), wheel scrolls half-page (synthesizes `u` / `d`). 13 popups respond too: list popups commit on left-click, viewer popups (YAML / Compare / App Log / Help) keep wheel scroll, the confirm dialog deliberately makes left-click a no-op so a stray click can't trigger a destructive delete / quit / rollback. Mouse can be turned off in the Settings popup (`>`) and a `scroll_direction: natural | reverse` setting flips the wheel for users who prefer the inverse mapping
- **Settings popup (`>`)** -- app-level surface with a cog glyph in the title. Currently carries Mouse on/off + Scroll Direction; future global settings drop in here. The popup is its own escape hatch: even when Mouse is off, clicking remains possible inside the popup so users can turn mouse back on
- **Layer-based popup borders (v1.7.4)** -- every popup picks its border color from a `lavender → sapphire` gradient based on nesting depth: layer 1 (any first-tier popup) uses lavenphire25, layer 2 (a popup over another popup — e.g. the Diff menu inside Compare, or Confirm inside Breadcrumb) uses lavenphire50, layer 3 lavenphire75, layer 4+ sapphire. The visual stack always reads "this thing is on top of what's underneath" without needing to think about it. Toast warnings keep their Catppuccin Peach border as a dedicated warning signal; KM8erm's user-footprint identity lives on the statusbar marker (still lavender) so the popup border stays consistent with every other overlay
- **27 built-in resource types + CRD support** -- dynamic discovery of Custom Resources at startup, across Cluster / Workloads / Network / Config / Storage / RBAC / Autoscaling / Helm categories. The Helm category only registers when the `helm` CLI is on `PATH`
- **Real-time Watch updates** -- resources refresh automatically via Kubernetes Watch API
- **Vim-style navigation** -- `j`/`k`, `u`/`d` page scroll, `gg`/`G`, `/` search
- **3-panel lazygit-style layout** -- numbered sidebar, list, and detail panels with scroll indicator
- **Drill-down navigation** -- Deployment / DaemonSet / StatefulSet / Job → Pods → Containers; CronJob → Jobs; HPA → target workload; PVC → mounting Pods; PDB → protected Pods; Helm Release → each native K8s object the chart deployed
- **Relatives tab — Lens-style navigation** -- every detail panel (except Namespaces) lists the resource's navigable references (owners, selected pods, scaleTargetRef, mounted-by pods, ...). `Enter` drills into the cursor's ref — the panel re-renders showing *that* resource's Relatives, building a chain (Deployment → Pod → ConfigMap → consumer Pods, ...). `Esc` pops one level. `Space` opens a breadcrumb popup so you can jump panels 1+2 back to any chain ancestor (confirms first). Tab label shows `Relatives N` at depth>1. `Y` opens the YAML of whichever entry the cursor is on. Cycle detection blocks revisiting an ancestor; fetch failures toast and stay put. 26 of 27 resource kinds covered — ConfigMaps / Secrets / ServiceAccounts surface *reverse* refs (which Pods use me, which RoleBindings name this SA as a subject, ...); Helm releases surface their `Deployed Resources` so each chart-deployed K8s object is one drill away
- **Helm releases (when `helm` is on `PATH`)** -- a dedicated `Helm > Releases` sidebar category lists every release in the cluster (`helm list -A` polled every 3s; no Helm watch API). Panel 2 columns: `NAME / NAMESPACE / CHART / APP VER / REV / STATUS / UPDATED`. Press `Space` on a release row to open a doc menu (Manifest / Creator Notes / User Values / Merged Values / Hooks); pick one with `Enter` to fetch via `helm get ...` and view the result in the YAML popup. The menu stays open behind the YAML so consecutive docs flow without re-opening. Panel 3 carries a `History` tab in place of Events — table view of every revision (REV / STATUS / DATE / CHART / DESCRIPTION) with the current deployed rev marked `●`. `Space` on a non-current row asks to roll back; confirm shows the exact `helm rollback` command and runs it asynchronously, with the result surfaced as a toast. Helm-managed K8s objects (label `app.kubernetes.io/managed-by: Helm` or annotation `meta.helm.sh/release-name`) are marked with a `` glyph in panel 2 and block `E` (kubectl edit) with a "Helm-managed (read-only)" toast — use `helm upgrade` / `rollback` instead. Press `.` on any non-Releases list to hide all helm-managed objects (panel 2 bottom-left always shows the `.: toggle helm` hint)
- **YAML popup (`Y`)** -- raw `kubectl get -o yaml` of the selected resource in a full-screen overlay with `j/k/u/d/gg/G` scroll, `/` search (`n`/`N` step through matches with full-row highlight), `y` to copy the full YAML to your clipboard, and `E` to dispatch `kubectl edit` directly from the popup. YAML lives in the popup, not the detail panel, so vertical layout no longer wraps long YAML lines awkwardly
- **Pod log streaming with auto-follow** -- multi-container support with `<container>|<log>` format; the Logs tab sticks to the tail by default. The Logs tab label carries an inline Nerd Font glyph showing the follow state — `▶` (live, U+F0753) when auto-follow is on, `⏸` (paused, U+F0754) after the user scrolls up. The glyph stays on the tab whether or not it's active so panel-3's tab bar stays the same width when you switch tabs. Scroll up (`k`/`↑`/`u`/`gg`) to pause and read history; press `G` to catch up and resume following
- **Aggregate logs for all workload kinds** -- selecting a workload row streams logs from **every Pod** the workload manages into a single Logs tab. Lines are prefixed `<pod-hash>│<container>│<text>` with each segment in its own stable color, so during a rollout you can spot at a glance which pod is throwing errors without drill-down. Covers Deployment (current ReplicaSet, falls back to selector on RBAC miss), StatefulSet, DaemonSet, Job, ReplicaSet, CronJob (across all retained Jobs). Pod churn: stream snapshots at row-select; re-select the row to refresh
- **Aggregate child events for workload kinds** -- the Events tab on a workload row merges events from the workload AND its child Pods, sorted newest first. The Object column ("`Pod/web-abc-xyz`" vs "`Deployment/web`") names each event's source so the chain is visible inline. CronJob is 3-tier: CronJob's own events + every owned Job's events + every Pod's events, so "why did last night's cron fail" reads from one tab instead of `kubectl describe` × N
- **Conditions tab** -- new detail-panel tab showing `.status.conditions` as a `TYPE / STATUS / REASON / MESSAGE / AGE` table (same as `kubectl describe`'s Conditions section). Status `False` rows highlighted red. Appears for kinds that populate conditions (Pod / Node / PVC / Deployment / StatefulSet / DaemonSet / Job / HPA / Ingress); hidden for kinds without (ConfigMap, Secret, Service, etc.). Critical when events have expired past TTL — conditions reflect *current* state, events reflect *recent* state
- **Status column coloring — abnormal-only** -- panel 2's Status column (every kind that has one — Pod / Node / Namespace / PVC / PV / Helm Release) plus the Events Type column paint **only** abnormal values. Yellow for transitional / degraded (Pending / Terminating / SchedulingDisabled / Released / pending-* / Init:*), red for failures (Failed / Error / CrashLoopBackOff / ImagePullBackOff / NotReady / Lost / Warning). Healthy values (Running / Bound / Active / Deployed / Normal) stay at the row's base foreground. Color = signal, not decoration — your eye is drawn only to rows that need attention. Cursor / lock rows pick the darker Catppuccin Latte variant so pastels don't wash out on the reverse-video bg
- **Row switch debounce (300ms)** -- panel 2 j/k mashing used to fire one detail fetch + one log-stream Start per row, even for rows the cursor flew past. The dispatch is now debounced: every row switch bumps a sequence counter and schedules the fetch / stream Start 300ms later, so a rapid scroll through 49 rows fires exactly one fetch (for the row you stopped on) instead of 49. Lie-as-lock invariant preserves: panel 2 still feels instantly responsive because the cheap state mutations (Stop previous stream, clear retry throttle) run inline; only the expensive work defers. Matches the existing sidebar `switchSeq` debounce window for muscle-memory consistency
- **Edit & shell exec via embedded PTY** -- `E` runs `kubectl edit` and `S` runs `kubectl exec -it -- /bin/sh`, both inside an in-app virtual terminal so the editor and shell session never touch the host terminal scrollback. Editor honors `$KUBE_EDITOR` / `$EDITOR` (or `config.yaml editor`)
- **KM8erm internal terminal** -- `Alt+t` toggles an embedded shell (login shell with full env / cwd) inside km8 — like `ssh localhost` in a popup. Run `kubectl apply -f`, `helm`, anything you'd normally drop out of km8 to do. The shell is **persistent**: pressing `Alt+t` while the popup is visible hides it without killing the shell; pressing it again reattaches (cwd, history, env, background jobs all preserved). A `KM8erm` chip on the right of the status bar shows when the shell is alive in the background. Independent of `kubectl edit` / `kubectl exec` — you can keep KM8erm running while editing a resource or exec'ing into a container in a separate popup
- **PTY popup borders follow the popup layer scale** -- KM8erm, `kubectl edit`, and `kubectl exec` all render with the popup-layer border color (deeper popups get deeper colors along the `lavender → sapphire` scale). KM8erm's "your persistent shell" identity lives on the statusbar marker (still lavender — the user-footprint accent), while the popup border itself reads as just another floating overlay. The title (`KM8erm: hostname` vs `Edit: pod/foo` vs `Shell: pod/foo → ctnr`) carries the kind distinction
- **PTY scrollback** -- 10k-line history for all PTY popups (KM8erm, shell exec, edit). `PgUp` / `PgDn` page, `Home` / `End` jump to top / live. Disabled in alt-screen apps (vim, less, htop) so they keep their own paging
- **Per-container colored log labels** -- multi-container pods are visually distinguishable line-by-line; stable color per container name
- **Resource deletion** -- `D` (uppercase, both as a hotkey and via the `Space` menu) with confirmation dialog
- **Search/filter** -- `/` to search in the sidebar and table panels, and in the namespace/context picker popups. Sidebar search also matches category names (e.g. "cluster" expands the Cluster category). Search clears automatically when focus moves to another panel — selection persists, the filter doesn't
- **Clipboard copy (`y`)** -- copies the focused panel's content via OSC 52 (works through tmux/SSH, no `xclip`/`pbcopy` required). Inside the App Log popup (`!`), `y` copies the full log; inside the YAML popup, `y` copies the full YAML
- **Toast notifications with levels** -- info-level (1s, popup-layer border + `󰵅 km8` title, hint reads `auto-dismiss`) for confirmations like "Copied!"; warning-level (2s Catppuccin Peach + `󰀦 km8` title) for blocked actions like Relatives cycle detection or drill failures; sticky variant for modal state contracts (drag mode hint) — hint switches to `Esc: close`, no auto-dismiss timer
- **Namespace and context switching** -- `N` for namespace, `C` for context (uppercase — trigger keys are uppercase to avoid mis-triggering while typing search queries)
- **Session-local context** -- switching context in km8 doesn't touch `~/.kube/config`. Run `kubectl` in another terminal in parallel without interference
- **Panel-aware selection styling** -- the focused panel's cursor row gets a bright lavender chip; the *unfocused* panel's selected row keeps a softer bg + bold so you can always see which resource each panel "remembers" while you work in another. Unfocused panels dim down to a muted overlay grey so the eye lands on the focused panel without losing the "you are here" memory of the other two
- **Detail tabs** -- panel 3's tab list is per-kind. Workload kinds (Pods / Deployments / StatefulSets / DaemonSets / Jobs / CronJobs) lead with `Logs` because switching rows is most often a "what is this thing doing right now" gesture — Relatives is a deliberate drill action that warrants the extra tab switch. Order is `Logs` / `Relatives` / `Events` / `Conditions` (Conditions only for kinds that populate `.status.conditions`). Non-workload kinds lead with `Relatives` so `Space` jumps from a Relatives entry land on the same tab the user came from. Helm releases get `Relatives` / `History`. Panel 3 has no `/` search — cursor tabs (Relatives / History) don't tolerate row filtering, and Logs read better as a plain follow-tail view; use `Y` + your editor to grep large content
- **Long values wrap, never truncate** -- applies to YAML, Events, and Logs; wrap points reflow on panel resize
- **Panel expand** -- `z` toggles full-screen on the focused Table or Detail panel; pressing `z` again restores the 3-panel layout
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
| **`Enter`** | **Into** — drill into the selected resource / commit a popup choice. Does **not** forward focus to another panel (use `Tab` / `1` / `2` / `3` for that) |
| **`Space`** | **Menu** — open a contextual popup wherever focus is: sidebar cheatsheet + Pin / Unpin / Sort / Drag actions (panel 1), per-row action menu split into item operations + panel-level Sort entry / container Shell menu / empty-list hint (panel 2), Logs / Events / Relatives-drill / Relatives-breadcrumb / History rollback (panel 3 by tab). Also closes any open popup (mirror open) |
| **`Esc`** | **Back** — pop one drill level / close any popup |

Where a contextual menu exists, `Space` is enough — you don't need to memorize the per-action keys.

Tab navigation also responds to `h`/`l` (or `[`/`]`) for switching panel 3 tabs.

### Mouse (since v1.6)

| Gesture | Behavior |
|---|---|
| **Left-click** on a panel row | Focus that panel + move the cursor to the clicked row |
| **Double-click** | Synthesizes `Enter` (drill into the cursor row) |
| **Right-click** on a row | Synthesizes `Space` (opens the row's context menu / cheatsheet) |
| **Wheel up / down** | Synthesizes `u` / `d` (half-page move). Direction can be flipped via Settings popup (`scroll_direction: natural | reverse`) |
| **Left-click** inside a list popup | Commits that row (same as cursor + `Enter`) |
| **Right-click** inside any popup | Closes it (same as `Esc`) |

Menu-style popups (panel 2 menu, sort picker, namespace / context picker, breadcrumb, helm doc menu, hint, settings, confirm) ignore the wheel — content is short and half-page semantics don't fit. Viewer popups (YAML / Compare / App Log / Help) **do** scroll on wheel. The confirm dialog deliberately makes left-click a no-op so a stray click can't trigger a destructive delete / quit / rollback — you confirm with keyboard `Enter` / `y` only.

Mouse can be disabled in the Settings popup (`>`); the popup itself stays mouse-reachable in that state so you can flip it back on.

### Accelerators — cursor + power triggers

```
 cursor    j k         u d         gg G        / (search inside current panel)
 trigger   Y YAML      E edit      N namespace
 panel 1   P pin       S sort      D drag-and-drop pinned (modal)    C context
 panel 2   S shell     Alt+Shift+S sort    D delete    C compare anchor
 expand    z           z toggles full-screen on current panel
 helm      .           . toggles helm-managed visibility on panel 2
 settings  >           > (shift+.) opens the global Settings popup
```

`S` / `C` / `D` are panel-aware overloads — same letter, different action depending on which panel has focus, mirroring how `P` only makes sense on panel 1. On panel 2, sort needs the `Alt+Shift+S` chord because bare `S` is already Shell — the modifier carves out a panel-2 sort gesture without breaking that. Trigger keys are deliberately uppercase to avoid misfiring while typing in a `/` search field.

### Global

| Key | Action |
|---|---|
| `>` | Open the global Settings popup (mouse on/off, scroll direction; future settings) |
| `Alt+t` | Toggle KM8erm (spawn / show / hide; shell stays alive across hide) |
| `y` | Copy focused panel content to clipboard (OSC 52) |
| `!` | App log |
| `?` | Help |
| `q` | Quit km8 (asks for confirmation) |
| `Ctrl+C` | Quit km8 immediately (no confirm) |

### Panel 1 sidebar Space menu

The action region is split into two labelled groups when both apply:

| Key | Action |
|---|---|
| `P` | **item operation** — Pin / Unpin the cursor's resource kind. Pinned kinds appear in a top "Pinned" section and **move** out of their original category. Order persists per-context to config |
| `S` | **item operation** — Open the Sort flow for the cursor's resource kind. Column picker loops with the direction picker so multi-column chains can be built without re-invoking. Reset row drops the whole chain |
| `D` | **panel operation** — Enter drag-and-drop reorder mode for the cursor's pinned kind. Only surfaces when the cursor is on a pinned row AND there's at least one other pinned kind to swap with |

### Panel 2 context menu (`Space` on any row)

Per-row menu with resource-aware items — `Y` YAML / `E` Edit / `S` Shell / `D` Delete plus a contextual **`C` Compare** entry (Mark anchor / Compare to anchor / Unmark anchor depending on state). A separator below them carves out a **panel operation** region: `[Alt][S]ort panel 2 list` opens the same column picker the panel-1 Space menu opens, scoped to the kind currently being viewed. Use `j`/`k` + `Enter` or hit the letter directly. Helm-managed rows hide `E`/`D` (Rule A: read-only — edits would be overwritten by `helm upgrade`/`rollback`); resources without containers hide `S`.

Two cursor-only entries are appended for navigation discoverability (no single-letter hotkey, reached via `j`/`k` + `Enter`):
- `Enter ↘` — drill into the row's children (`pods` / `jobs` / etc., per kind). Same action as pressing `Enter` on the row directly.
- `Esc ↖` — back to the parent list. Only appears when you're already inside a kind-level drill chain (e.g. viewing a Deployment's Pods). Same action as pressing `Esc` directly.

**Container drill exception:** when the cursor is on a container row (one level deeper than a Pod's pods list), the Space menu collapses to just `Shell` — the Esc entry is dropped because Esc is the universal pop-one-level gesture and a one-row menu doesn't need a redundant "back" affordance.

### Compare mode

While the anchor is set, panel 2's bottom-left border shows an `esc: exit compare` hint. The locked row paints with a lavender (Mocha) bold-reverse-video bg, the same accent as Pinned items and the Settings ON toggle — "user-set state on this row". A fixed-width `<icon> Compare` chip on the status bar confirms the mode is on without claiming variable cells for the resource name (the popup itself shows `left vs right` when you engage). Compare lock auto-clears when focus leaves panel 2 or when the anchored row falls out of the watcher stream (deleted / namespace-filtered away).

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
km8erm_shell: ""         # shell launched by KM8erm (default: $SHELL → /bin/sh).
                         # Bare names are resolved via $PATH at popup-open time
                         # (Go exec.Command semantics); absolute paths are used
                         # verbatim. Lets you pick e.g. fish inside km8erm
                         # while keeping zsh as host shell.
km8erm_login_shell: false # Flip true to launch KM8erm with `-l` so it sources
                         # ~/.zprofile / ~/.bash_profile / /etc/profile.
                         # Default false matches the v1.7.2 baseline (non-login
                         # interactive — .bashrc/.zshrc still loads, no
                         # /etc/profile PS1 surprise). Set true when km8 is
                         # launched from a non-login parent (Raycast, Alfred,
                         # cron, tmux configured non-login) and your PATH
                         # lives in .zprofile rather than .zshrc.

# Compare popup defaults (v1.6+). `layout` picks the diff render —
# "unified" (default) is a single column with -/+ markers,
# "split" is side-by-side.
compare:
  layout: unified

# Mouse settings (v1.6+). Both fields optional; omitting either
# falls back to the defaults below.
mouse_opt_config:
  enabled: true                # set false to disable click + double-click + right-click + wheel
  scroll_direction: natural    # "natural": wheel-up = cursor up. "reverse" swaps the mapping.

# Per-kind preferences (v1.6+). Keyed by kubectl name
# ("pod" / "deployment" / "configmap" / ...). Each entry is
# optional and unknown kinds are preserved across the rewrite,
# so a CRD that briefly uninstalls won't lose its pin / sort.
#
# Sort is a multi-tier chain (v1.7+); tier 0 is the primary,
# tier 1 the first tiebreaker, etc. The v1.6 single-mapping
# shape is still accepted on load — it lifts to a one-tier
# chain and is rewritten in the new sequence form on save.
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

Both override the corresponding config slot for one-shot runs without editing the YAML — useful for CI / scripted demos / quick "try this shell" sessions.

| Variable | Effect | Precedence |
|---|---|---|
| `KM8__CONFIGPATH` | Use this file as the config file instead of the default layout (`$XDG_CONFIG_HOME/km8/config.yaml` etc.). Theme file path is NOT affected — it still lives under the OS config directory. Absolute path recommended; relative path resolves against CWD at load/save time. | `KM8__CONFIGPATH` > default layout |
| `KM8__SHELL` | Use this binary as the KM8erm shell. Bare names are looked up on `$PATH` at popup-open time (Go `exec.Command` semantics); absolute paths run verbatim. Leading / trailing whitespace is trimmed. | `KM8__SHELL` > `km8erm_shell` config > `$SHELL` > `/bin/sh` |
| `KM8__LOGIN_SHELL` | Force the KM8erm shell into login mode (`-l`) or out of it. Truthy values: `true` / `1` / `yes` (and uppercase). Any other value disables login mode. Use when launched from a non-login parent and your PATH is set in `.zprofile`. | `KM8__LOGIN_SHELL` > `km8erm_login_shell` config > `false` |

Example:

```sh
# Try fish in KM8erm without editing config.yaml
KM8__SHELL=/opt/homebrew/bin/fish km8

# Point km8 at a per-project config (e.g. checked into the repo)
KM8__CONFIGPATH="$PWD/.km8.yaml" km8
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
- **A Nerd Font Mono variant** for the terminal (e.g. JetBrains Mono Nerd Font Mono, FiraCode Nerd Font Mono). km8's popup titles + row markers use Nerd Font glyphs from the Material Design icon range; the Mono variants are designed to render every glyph at exactly 1 cell, so column / border alignment is stable. The proportional (non-Mono) variants and terminals running East-Asian-Ambiguous=double (some tmux + iTerm2 CJK setups) can paint these glyphs at 2 cells — km8 still works, but helm-managed rows + popup top borders may sit 1 cell off-grid. Switch to the Mono variant or set ambiguous-width=single if you see drift.

## License

[GPL-3.0](LICENSE)
