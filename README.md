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

**A single-pane Kubernetes workspace in your terminal.** The kinds on the left, the resources of the one you are on, and
everything about the row under the cursor — its logs, what it is connected to, its events — live as the cluster changes.
`Space` shows what you can do wherever you are, `?` shows the keys, `Esc` steps back. Relatives navigation, YAML compare and
an embedded shell are built in; any other tool you trust rides along in that shell.

> _When in doubt, press_ **`Space`**.

![basics](docs/demo-basics.gif)

## Features

### Find your way around

- **Relatives** — every resource lists what it is connected to: its owner, the Pods a Service selects, the workload an HPA
  scales, the Pods mounting a PVC, the Pods using a ConfigMap or Secret. `Enter` follows a link, `Esc` steps back, and
  `[B]readcrumb` takes panels 1 and 2 to any point of the chain you walked.
- **Drill-down** — Deployment / StatefulSet / DaemonSet / Job → Pods → Containers, CronJob → Jobs, HPA → its target, Helm
  release → every object the chart deployed.
- **28 built-in resource types plus your CRDs** — Custom Resources are found at start, and every list updates live through
  the Kubernetes watch.
- **Several namespaces at once** — check any set of namespaces in the `[N]amespace` picker, or `Select all` for the whole
  cluster. kbu remembers the choice.
- **Pin and sort** — pin the kinds you use most to the top of panel 1 and drag them into order; sort any list by one or more
  columns. Both are kept per kind.
- **Filter** — `/` narrows panel 1 and panel 2 as you type, and the namespace and context pickers too.
- **Picks up where you left off** — quit and start again: the same context, namespaces, kind, row, panel and tab.

### Look closer

- **Logs** — follow the newest line, pause when you scroll up, `G` follows again. On a workload the logs of **every Pod** it
  runs come in one view, each Pod and container in its own colour, so during a rollout you see which one fails.
- **Events** — a workload's own events with its Pods' (a CronJob's with its Jobs' too), newest first.
- **Conditions** — `.status.conditions` as a table, as `kubectl describe` shows them; still there after the events expire.
- **YAML viewer** (`[Y]AML`) — line numbers and colours; `/` searches, `n` / `N` go to the next and the previous match, `v`
  selects, `y` copies.
- **Compare** — diff two resources of the same kind, only where they differ, unified or side by side (`h|l` switches, and
  kbu keeps your choice).
  Status and server-managed fields are left out, so you see only what was written.
- **Problems stand out** — a status cell is coloured only when it needs a look: pending or degraded, or failed.
- **Helm releases** — when `helm` is on your `PATH`: each release's manifest, values, notes and hooks; its revision history
  with a rollback. Objects a chart manages are marked, protected from edits, and hidden with `.` when you want.
- **KubeConfig contexts** — your kubeconfig, read-only; `Enter` on a context switches kbu to it. Credentials are never shown.

### Get things done

- **Edit** (`[E]dit`) — the real `kubectl edit` in a terminal inside kbu, with your editor.
- **Shell** (`[S]hell`) — `kubectl exec` into a container, inside kbu too.
- **Delete** (`[D]elete`) — always asks first; a namespace gets an extra warning.
- **Alterm** (`Alt-t`) — a shell that lives inside kbu. Hide it and bring it back with the same key: its directory, history
  and jobs stay. `Alt-Esc` ends it. What kbu does not do, your usual tools do here.
- **Copy** (`y`) — the row, or the whole view, to your clipboard (OSC 52, so it works over ssh and in tmux).
- **Audit log** — every edit and delete made from kbu is written down.
- **Your context stays yours** — switching context in kbu never touches `~/.kube/config`; `kubectl` in another terminal is
  unaffected.
- **Full screen** (`z`) — one panel across the whole screen; `z` again for the three.

## Install

### Requirements

- **kubectl** on `PATH`, for edit, delete, shell and what you run in Alterm.
- A **kubeconfig** (`~/.kube/config` or `$KUBECONFIG`) and a cluster to reach.
- **A Nerd Font** in your terminal profile. Some fonts draw an icon two cells wide, some draw the powerline symbols (the ends
  of the capsules on the frames) two cells wide too; kbu asks the terminal at start and lays the screen out to match. To say
  it yourself: `KBU__ICON_WIDTH` and `KBU__POWERLINE_WIDTH` (`1` or `2`) for kbu alone, or `TERMINU__ICON_WIDTH` and
  `TERMINU__POWERLINE_WIDTH`, shared by the whole terminu family — set them once and every family app reads them; when kbu
  runs inside another family app's terminal, that app sets them for it.
- **A truecolor terminal** (24-bit colour): the soft colours and the shading between stacked popups cannot be told apart in
  256 colours.
- **A terminal of at least 40×15.** Below 60 columns kbu shows one panel at a time; below 40×15 it shows only its name,
  the size it needs and how to quit (`Ctrl-C` twice), and comes back as you make the window bigger.

### Quick install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/install.sh | sh
```

### Quick install (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/install.ps1 | iex
```

### Homebrew (macOS / Linux)

```bash
brew install vulcanshen/tap/kbu
```

### Scoop (Windows)

```powershell
scoop bucket add vulcanshen https://github.com/vulcanshen/scoop-bucket
scoop install kbu
```

Building from source is in [`docs/dev-remarks.md`](docs/dev-remarks.md).

### The first start

kbu connects to your kubeconfig's current context and shows the Pods. Nothing is written until you pin, sort or quit:
pinning or sorting makes `~/.kbu`, quitting `~/.config/kbu` (see [Settings and data](#settings-and-data)). If an older kbu left its settings in
`~/Library/Application Support/kbu` (macOS) or `%APPDATA%\kbu` (Windows), the first start moves them to `~/.config/kbu`
and says so.

### Uninstall

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.ps1 | iex
```

Both ask before they remove your settings, your state and your logs.

## Quick start

```bash
kbu
```

Move with `j|k`. Panel 2 follows the kind under panel 1's cursor, panel 3 the row under panel 2's. `Tab` goes to the
next panel (or `1`–`3`), `Enter` does the obvious thing to what is under the cursor, `Space` lists everything you can do
there, `Esc` steps back, `q` leaves.

`kbu version` prints the version, `kbu help` how to run it.

## Using kbu

### The keys every panel shares

| Key | What it does |
|---|---|
| `Space` | What you can do here: the item, then the panel, then `Global operation` (namespaces, context, Alterm, quit). In a mode (Compare, Drag), what the mode can do |
| `?` | The keys of whatever is in front: the panel, or the popup on top. `?` again or `Esc` closes it |
| `Tab`, `Shift-Tab`, `1–3` | The next panel, the previous one, a panel by its number |
| `Enter` | The obvious action: drill in (a workload into its Pods), open a YAML, switch to a context, a shell into a container, roll back to a Helm revision. On panel 1 it takes you to panel 2; on Logs, Events, Conditions and Info it is full screen |
| `Esc` | One step back: clear the filter, leave a mode (Compare, Drag), come back up a drill. It never leaves kbu |
| `!` | The App Log: what kbu did and what went wrong, newest last. The footer says `!:2 errors` until you have looked |
| `q`, `Ctrl-C` | Leave kbu (it remembers where you were). With a popup open it asks first |
| `j\|k`, `u\|d`, `gg\|G` | Down and up, half a page, the top and the end. On panel 3 `h\|l` switch tabs |

Every other key is a shortcut of a row of the `Space` menu, and the `?` of each panel lists them. The ones you will use most:

| Where | Key | What it does |
|---|---|---|
| Panel 1 | `P` | Pin or unpin the kind |
| Panel 1 | `D` | Drag the pinned kind into place: `j\|k`, then `Enter`; `Esc` puts it back |
| Panel 2 | `Y`, `E`, `S`, `D` | YAML, edit, shell, delete |
| Panel 2 | `C` | Compare: the row becomes the anchor and panel 2 goes into the Compare mode. Move to another row and press `C` or `Enter` to see where the two differ, as often as you like; `Esc` leaves the mode |
| Panel 2 | `Alt-S` | Sort the list by a column, then a direction |
| Panel 2 | `.` | Show or hide what a Helm chart manages |
| Any panel | `z` | Full screen; `z` again for the three |
| Panel 3 | `B` | On Relatives, deep in a chain: the breadcrumb |
| Anywhere on a panel | `N`, `C`, `Alt-t` | Namespaces, context (on panel 2 `C` is compare), Alterm |

When a popup opens over another, only the one you are in is bright; everything under it dims.

### Terminals (Alterm, edit, shell)

| Key | What it does |
|---|---|
| `Alt-Esc` | End the program, asked first |
| `Alt-t` | Alterm only: hide it; the shell keeps running |
| `PgUp` | Alterm and shell: History, back through what went off the top (`j\|k`, `u\|d`, `gg\|G`; `Esc` for the live screen) |
| `Alt-v` | Alterm and shell: select text and copy it (`v`, then `y`) |
| Any other key | To the program; in edit every key is the editor's, `PgUp` too |

### Helm

Panel 2's `Space` menu on a release lists its documents beside `[Y]AML`: `Manifest`, `Creator Notes`, `User Values`,
`Merged Values`, `Hooks`. On panel 3's History tab, `Enter` rolls the release back to the revision under the cursor (not the
one deployed); the confirm shows the exact `helm rollback`.

### Editing

`[E]dit` asks, then runs `kubectl edit <kind>/<name> -n <namespace> --context <context>` in a terminal popup: the same as in
any terminal — a strategic merge patch, the conflict check, no `last-applied-configuration` side effect. kubectl picks the
editor: `$KUBE_EDITOR` (kbu sets it when `editor` is in `config.yaml`), then `$EDITOR`, then `vi` (`notepad` on Windows).
When the editor exits, the list catches up through the watch.

If your nvim is slow to quit in the popup (plugins tearing down), `editor: "nvim --noplugin"` skips the plugins for kbu's
edits only.

## Settings and data

kbu's settings and where you left off are in **`~/.config/kbu`** (`$XDG_CONFIG_HOME/kbu` when that is set; on Windows
`%USERPROFILE%\.config\kbu`). `KBU__CONFIG` points it at another directory. Deleting them only takes kbu back to its
defaults.

| File | What it is |
|---|---|
| `config.yaml` | Your settings (below) |
| `state.yaml` | Where you left off, and whether the compare was unified or side by side; written when you quit. `KBU__STATE` keeps it in another directory |

kbu's data is in **`~/.kbu`** (`KBU__DATA` points it elsewhere). Back it up if you need it: deleted, it is gone.

| File | What it is |
|---|---|
| `kinds.yaml` | Your pins and sorts, kind by kind (below). kbu writes it when you pin or sort |
| `logs/audit-<date>.log` | Every edit and delete made from kbu |
| `logs/crash-<date>.log` | What kbu left when it crashed |

An older kbu kept the settings in `~/Library/Application Support/kbu` (macOS) or `%APPDATA%\kbu` (Windows), and its logs
next to them. The first time you start this one, it moves them, one by one, to `~/.config/kbu` and `~/.kbu/logs`, and says
so. What the new place has already stays where it was, and kbu shows both paths for you to merge them; if the settings
cannot be moved, kbu keeps using the old place and shows both paths, so you can move them yourself. An older kbu also
kept the pins and sorts in `config.yaml` (`resource_kind_config`): the first start moves them to `~/.kbu/kinds.yaml` and
leaves the rest of `config.yaml` as it was. When `kinds.yaml` is there already, they stay in `config.yaml` for you to
merge; when they cannot be moved, kbu keeps using them there; either way it shows both paths.

When kbu cannot read `config.yaml`, `state.yaml` or `kinds.yaml` (a broken file, no permission), it asks at the start:
`Enter` renames it to `config.yaml.bak` (or `.bak.1`…, never over another) and kbu starts as on its first start, from the
defaults, or with no pins and sorts; `Esc` leaves kbu and prints the path and the reason in the terminal.

### config.yaml

```yaml
default_context: ""       # the context to start on (default: the kubeconfig's current-context)
default_namespace: ""     # the namespace to start on (default: all)
editor: ""                # given to kubectl as $KUBE_EDITOR
alterm_shell: ""          # Alterm's shell (default: $SHELL, then /bin/sh); a bare name is looked up on PATH
alterm_login_shell: false # true starts Alterm's shell with -l, reading ~/.zprofile / ~/.bash_profile
input_tab_width: 4        # the cells a tab takes in the viewers (YAML, logs); shared by the terminu family
```

### kinds.yaml

```yaml
# Per kind, by its kubectl name. A kind that is gone for a while (a CRD
# reinstalled) keeps its pin and sort.
pod:
  pinned:
    order: 10           # 10 apart, so a hand edit can put one in between
  sort:                 # first the restarts, most first; then the name
    - column: Restarts
      direction: desc
    - column: Name
      direction: asc
```

### Environment variables

| Variable | What it does |
|---|---|
| `KBU__CONFIG` | The settings directory, instead of `~/.config/kbu` |
| `KBU__STATE` | Where `state.yaml` goes, instead of the settings directory |
| `KBU__DATA` | Where the data goes (the pins and sorts, the logs), instead of `~/.kbu` |
| `KBU__ALTERM_SHELL` | Alterm's shell, before `alterm_shell` and `$SHELL` |
| `KBU__ALTERM_LOGIN_SHELL` | `true` or `false`: Alterm's shell as a login shell, before `alterm_login_shell` |
| `KBU__ICON_WIDTH`, `KBU__POWERLINE_WIDTH` | `1` or `2`: the cells an icon and a powerline symbol take (see [Requirements](#requirements)) |
| `TERMINU__ICON_WIDTH`, `TERMINU__POWERLINE_WIDTH` | The same for every terminu app; kbu sets them for what runs in its terminals |

## Limits

- **The mouse does nothing.** kbu, like every terminu app, is used from the keyboard.
- **Alterm, edit and shell need macOS or Linux** (WSL too): on Windows kbu cannot open a terminal inside itself.
- **If the frames are a cell off**, the check kbu runs at start got a wrong answer from your terminal: `kbu iconwidth` prints
  what it found; set `KBU__ICON_WIDTH` and `KBU__POWERLINE_WIDTH`. On Windows there is no check: set them there when your font
  draws icons two cells wide.
- **Helm needs the `helm` command**: the Helm category appears only when it is on `PATH`, and the release list is read again
  every 3 seconds (Helm has no watch).
- **A workload's logs follow the Pods there when you chose the row.** After a rollout, choose the row again for the new ones.
- **Some edits and deletes are not offered.** Events and nodes cannot be deleted from kbu; what a Helm chart manages is changed
  with `helm upgrade`, `rollback` and `uninstall`.
- **Panel 3 has no `/` filter.** Open the YAML with `Y` and search there.
- **On a narrow panel 2 long values are cut first**, names in the middle (`demo-app…66-gm72n`), and only then do whole
  columns go, the least needed first; the name never goes. `z` shows every column in full.

## Links

- [CHANGELOG.md](CHANGELOG.md) — what changed in each release
- [`docs/dev-remarks.md`](docs/dev-remarks.md) — how it works, why, building and testing

## terminu family

kbu follows the [terminu design](https://github.com/vulcanshen/terminu): the same keys and the same menus as the rest of the
family — [filu](https://github.com/vulcanshen/filu) (files), [sshu](https://github.com/vulcanshen/sshu) (ssh),
[webu](https://github.com/vulcanshen/webu) (the web), [locku](https://github.com/vulcanshen/locku) (screen lock),
[kamonu](https://github.com/vulcanshen/kamonu) (the family's marks) and [ccdrawu](https://github.com/vulcanshen/ccdrawu)
(a sidebar and a status line for Claude Code).

## License

[GPL-3.0](LICENSE)
