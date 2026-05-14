# GOAL.md — km8

> Kubernetes TUI 管理工具，以 Lens IDE 的 UI 設計為參考，用 Go + Bubble Tea 實作終端介面版本。

## 技術選型

- **語言：** Go
- **TUI 框架：** [Bubble Tea](https://github.com/charmbracelet/bubbletea)（Elm 架構，可程式化測試）
- **UI 元件：** [Bubbles](https://github.com/charmbracelet/bubbles)（table, list, viewport）
- **樣式：** [Lipgloss](https://github.com/charmbracelet/lipgloss)（terminal styling/layout）
- **K8s 互動：** [client-go](https://github.com/kubernetes/client-go)（官方 Go client）
- **跨平台：** macOS、Linux、Windows

## UI 架構

```
┌─────────────────────────────────────────────────────┐
│ [Status Bar] cluster | namespace | context | help   │
├──────────┬──────────────────────────────────────────┤
│ Sidebar  │  NAME        STATUS    RESTARTS  AGE     │
│          │  nginx-7b..  Running   0         3d      │
│ Cluster  │  redis-5c..  Running   2         1d      │
│ ► Nodes  │  api-6f...   Pending   0         5m      │
│ Workloads│                                          │
│  Pods    │──────────────────────────────────────────│
│  Deploy  │  Detail / Dock Panel                     │
│  DaemonS │  Labels: app=nginx                       │
│  StatefS │  IP: 10.0.0.5                            │
│  Jobs    │  Containers: 1/1 ready                   │
│  CronJob │                                          │
│ Network  │                                          │
│  Svc     │                                          │
│  Ingress │                                          │
│ Config   │                                          │
│  CM      │                                          │
│  Secrets │                                          │
├──────────┴──────────────────────────────────────────┤
│ [Status Line] keybindings hint                      │
└─────────────────────────────────────────────────────┘
```

## 資源範圍（第一版）

| 分類 | 資源 | 表格欄位 |
|---|---|---|
| Cluster | Namespaces | Name, Status, Age |
| Cluster | Nodes | Name, Status, Roles, Version, Age |
| Workloads | Pods | Name, Ready, Status, Restarts, Age, Node |
| Workloads | Deployments | Name, Ready, Up-to-date, Available, Age |
| Workloads | DaemonSets | Name, Desired, Current, Ready, Age |
| Workloads | StatefulSets | Name, Ready, Age |
| Workloads | Jobs | Name, Completions, Duration, Age |
| Workloads | CronJobs | Name, Schedule, Suspend, Active, Last Schedule |
| Network | Services | Name, Type, Cluster-IP, External-IP, Ports, Age |
| Network | Ingress | Name, Class, Hosts, Ports, Age |
| Config | ConfigMaps | Name, Data count, Age |
| Config | Secrets | Name, Type, Data count, Age (metadata only) |
| Events | Events | Type, Reason, Object, Message, Age |

## 功能需求

### Phase 1 — Foundation
- [ ] Go module init + project structure (cmd/, internal/ui, internal/k8s, internal/config, internal/theme)
- [ ] K8s client connection (kubeconfig loading, cluster info)
- [ ] Bubble Tea app skeleton (root model, basic layout)
- [ ] Sidebar model (resource tree navigation, j/k/Enter)
- [ ] Table model (resource list, j/k scroll, column headers)
- [ ] Vim keybinding system (j/k/h/l/gg/G/Esc) with programmatic tests
- [ ] Status bar (cluster, namespace, context display)

### Phase 2 — Resource Implementation
- [ ] All 13 resource types: list fetcher + table column mapping
- [ ] Detail panel (metadata, labels, annotations, status)
- [ ] Namespace switching (all namespaces + per-namespace)
- [ ] Panel focus system (Tab / Ctrl+h/j/k/l between sidebar, table, detail)

### Phase 3 — Advanced Features
- [ ] Pod logs streaming (dock panel, follow mode)
- [ ] Table search/filter (/ to enter search, Esc to clear)
- [ ] YAML edit flow ($EDITOR → diff → kubectl apply)
- [ ] Help overlay (? to show keybinding reference)

### Phase 4 — Polish
- [ ] Responsive layout (WindowSizeMsg handling, proportional panels)
- [ ] Cross-platform build (macOS/Linux/Windows)
- [ ] Error handling and user feedback (status line messages)
- [ ] Config file support (~/.config/km8/config.yaml)
- [ ] Color theme system (lipgloss-based, configurable)

## 驗收標準

- [ ] `go build .` 成功，支援三平台交叉編譯
- [ ] 連接本地 K8s cluster（OrbStack）並顯示資源
- [ ] 所有 13 種資源可在 Sidebar 切換並以表格顯示
- [ ] Vim motion 操作正常（j/k/h/l/gg/G）— 有對應 model tests
- [ ] 選取資源可查看 Detail Panel
- [ ] Namespace 切換功能正常
- [ ] Pod logs 可串流顯示
- [ ] 任意資源可開啟 YAML 編輯並 apply
- [ ] Table 搜尋/篩選功能
- [ ] Terminal 視窗縮放時 UI 自動配適
- [ ] 無 runtime panic，錯誤有明確提示

## 開發環境

- **Go:** 1.26+ (darwin/arm64)
- **K8s:** OrbStack K8s local cluster
- **OS:** macOS (Apple Silicon)
- **Terminal:** tmux 環境

## 從 project-zero/km8 學到的教訓

1. **tview keybinding 不可靠** — InputCapture 行為不透明，Widget/App 層級事件傳播不明確
2. **Claude Code 無法互動測試 TUI** — Bash tool 無 TTY，TUI 無法啟動或接收輸入
3. **選用 bubbletea 解決測試問題** — Elm 架構允許程式化送 KeyMsg 並 assert model 狀態
4. **TUI 開發不適合無盡 loop** — 但 K8s 層、config、model 邏輯可以自主測試
5. **gocui 也是選項** — lazydocker/lazygit 使用，但 bubbletea 的測試故事更強
