# km8

A terminal UI for Kubernetes, inspired by Lens IDE. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![km8 screenshot](docs/screenshot.png)

## Features

- **18 resource types** -- Pods, Deployments, DaemonSets, StatefulSets, Jobs, CronJobs, Services, Ingresses, ConfigMaps, Secrets, Namespaces, Nodes, Events, ServiceAccounts, ClusterRoles, ClusterRoleBindings, Roles, RoleBindings
- **Real-time Watch updates** -- resources refresh automatically via Kubernetes Watch API
- **Vim-style navigation** -- `j`/`k`/`h`/`l`, `gg`/`G`, `/` search
- **3-panel lazygit-style layout** -- numbered sidebar, list, and detail panels
- **Drill-down navigation** -- Deployment -> Pods -> Containers
- **Pod log streaming** -- multi-container support with `<container>|<log>` format
- **Container shell exec** -- `kubectl exec` into any container
- **kubectl edit integration** -- opens `$EDITOR` for in-place resource editing
- **Resource deletion** -- with confirmation dialog
- **Search/filter** -- `/` to search in all three panels
- **Namespace and context switching** -- switch namespace (`n`) or kubeconfig context (`c`)
- **Detail tabs** -- Detail / Events / Logs (Logs tab only for Pods)
- **Expand detail to full screen** -- `+`/`-` to toggle
- **Theme system** -- drop a `theme.yaml` into config directory to override colors
- **Help overlay** -- `?` to show keybinding reference
- **App log** -- `!` to view internal application log

## Installation

### From source

```bash
go install github.com/vulcanshen/km8/cmd@latest
```

### Build locally

```bash
git clone https://github.com/vulcanshen/km8.git
cd km8
go build .
./km8
```

## Quick Start

```bash
# Uses current kubeconfig context
km8

# Or build and run
go build . && ./km8
```

km8 connects to your current kubeconfig context on startup. Use `n` to switch namespaces and `c` to switch contexts.

## Key Bindings

### Global

| Key | Action |
|---|---|
| `q` / `Ctrl+c` | Quit |
| `?` | Toggle help overlay |
| `!` | Toggle app log |
| `1` / `2` / `3` | Jump to panel (Sidebar / List / Detail) |
| `Tab` | Cycle to next panel |
| `Shift+Tab` | Cycle to previous panel |
| `n` | Switch namespace |
| `c` | Switch context |

### Panel 1 -- Sidebar

| Key | Action |
|---|---|
| `j` / `k` | Move cursor down / up |
| `gg` / `G` | Jump to top / bottom |
| `/` | Search resources |
| `Esc` | Clear search filter |

### Panel 2 -- List

| Key | Action |
|---|---|
| `j` / `k` | Move cursor down / up |
| `gg` / `G` | Jump to top / bottom |
| `h` / `l` | Switch detail tab (prev / next) |
| `/` | Search / filter rows |
| `Enter` | Drill down (Deployment -> Pods -> Containers) |
| `Esc` | Clear filter or exit drill-down |
| `e` | Edit resource (`kubectl edit`) |
| `d` | Delete resource (with confirmation) |
| `s` | Shell exec into container |

### Panel 3 -- Detail

| Key | Action |
|---|---|
| `j` / `k` | Scroll down / up |
| `h` / `l` | Switch tab (prev / next) |
| `/` | Search within detail |
| `+` | Expand detail to full screen |
| `-` | Collapse back to split view |

## Layout

```
+-----------------------------------------------------+
| [Status Bar] cluster | namespace | context           |
+----------+------------------------------------------+
| [1]      | [2] Pods                                 |
| Sidebar  |  NAME        STATUS    RESTARTS  AGE      |
|          |  nginx-7b..  Running   0         3d       |
| Cluster  |  redis-5c..  Running   2         1d       |
|  Nodes   |                                           |
| Workloads|------------------------------------------+|
|  Pods    | [3] Detail                                |
|  Deploy  |  Labels: app=nginx                        |
| Network  |  IP: 10.0.0.5                             |
|  Svc     |  Containers: 1/1 ready                    |
| Config   |                                           |
|  CM      |                                           |
+----------+------------------------------------------+
| [Status Line] keybinding hints                       |
+-----------------------------------------------------+
```

## Configuration

km8 looks for configuration files in the OS-appropriate config directory:

| OS | Path |
|---|---|
| Linux | `$XDG_CONFIG_HOME/km8/` or `~/.config/km8/` |
| macOS | `~/Library/Application Support/km8/` |
| Windows | `%APPDATA%/km8/` |

### config.yaml

```yaml
# Kubeconfig context to use on startup (default: current-context)
default_context: ""

# Namespace filter on startup (default: all namespaces)
default_namespace: ""

# Editor for kubectl edit (default: $EDITOR)
editor: ""
```

### theme.yaml

Place a `theme.yaml` in the same config directory to customize colors. Works like lazygit -- community theme files (catppuccin, dracula, etc.) can be dropped in directly.

## Requirements

- **Go** 1.22+ (for building from source)
- **kubectl** on `$PATH` (for edit, delete, and shell exec)
- A valid **kubeconfig** (`~/.kube/config` or `$KUBECONFIG`)
- A running Kubernetes cluster

## License

MIT
