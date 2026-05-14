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

## 設計決策

### 資料層
- **更新策略：** Watch（即時推送），非 Polling
- **Namespace：** 預設顯示所有 Namespace
- **Context 切換：** 支援切換 kubeconfig context，切換時整個畫面重繪
- **CRD：** v1 不支援，未來再開發

### UI 行為
- **Detail 區域：** 同一區域多個 Tab 切換
  - **Detail Tab** — 結構化欄位（metadata、status、spec 重點）
  - **Events Tab** — 該資源相關的 Events
  - **Logs Tab** — 僅 Pod 類資源顯示，多 container 以 `<container-name>|<log>` 格式混合顯示，選取特定 container 時只顯示該 container
- **Sidebar Events：** 獨立項目，顯示全域 Events
- **Detail 內容：** 結構化欄位，非 raw YAML
- **Panel 大小：** 固定比例，不支援拖曳調整
- **滑鼠：** 僅支援滾輪滾動，不支援拖曳操作

### 操作流程
- **YAML 編輯：** 使用 `kubectl edit`（開啟 $EDITOR，存檔即生效），不自行實作 diff + apply

### Theme 系統
- **機制：** 內建 default theme，使用者將 `theme.yaml` 放到 `~/.config/km8/theme.yaml` 即覆蓋
- **格式：** 獨立 `theme.yaml` 檔案，每個 UI 元素的顏色獨立定義
- **使用方式：** 類似 lazygit — 社群分享 theme 檔案（如 catppuccin-mocha、dracula），下載貼到 config 目錄即生效
- **`config.yaml` 不含 theme 設定**，theme 完全由 `theme.yaml` 控制

## 功能需求

### Phase 1 — Foundation
- [x] Go module init + project structure (cmd/, internal/ui, internal/k8s, internal/config, internal/theme)
- [x] K8s client connection (kubeconfig loading, cluster info)
- [x] Bubble Tea app skeleton (root model, basic layout)
- [x] Sidebar model (resource tree navigation, j/k/Enter)
- [x] Table model (resource list, j/k scroll, column headers)
- [x] Vim keybinding system (j/k/h/l/gg/G/Esc) with programmatic tests
- [x] Status bar (cluster, namespace, context display)

### Phase 2 — Resource Implementation
- [x] All 13 resource types: list fetcher + table column mapping
- [x] Detail panel (metadata, labels, annotations, status — structured tabs)
- [x] Events tab in detail panel (resource-specific events)
- [x] Namespace switching (all namespaces + per-namespace filter)
- [x] Panel focus system (Tab / 1/2/3 between sidebar, table, detail)

### Phase 3 — Advanced Features
- [x] Pod logs tab (multi-container: `<container>|<log>` format, single-container filter)
- [x] Table search/filter (/ to enter search, Esc to clear)
- [x] YAML edit via `kubectl edit` ($EDITOR, save to apply)
- [x] Help overlay (? to show keybinding reference)
- [x] Context switching (選單 + 全畫面重繪)

### Phase 4 — Polish
- [x] Responsive layout (WindowSizeMsg handling, fixed proportional panels)
- [x] Cross-platform build (macOS/Linux/Windows)
- [x] Error handling and user feedback (app log with ! key)
- [x] Config file support (~/.config/km8/config.yaml)
- [x] Theme system (`~/.config/km8/theme.yaml`, 內建 default, 支援社群 theme 檔案覆蓋)

### Beyond Plan — Additional Features
- [x] Lazygit-style panel borders with numbered titles
- [x] Drill-down navigation (Deployment/DaemonSet/StatefulSet/Job → Pods, CronJob → Jobs, Pod → Containers)
- [x] Search (/) in all 3 panels (sidebar, table, detail)
- [x] +/- expand detail to full screen
- [x] Dynamic tabs (Pods: Detail/Logs/Events, others: Detail/Events)
- [x] Container detail (image, state, ready, restarts, ports)

## 驗收標準

- [x] `go build .` 成功，支援三平台交叉編譯
- [x] 連接本地 K8s cluster（OrbStack）並顯示資源
- [x] Watch 即時更新資源狀態
- [x] 所有 13 種資源可在 Sidebar 切換並以表格顯示
- [x] Vim motion 操作正常（j/k/h/l/gg/G）— 有對應 model tests
- [x] 選取資源可查看 Detail Panel（結構化欄位）
- [x] Detail 區域 Tab 切換（Detail / Events / Logs）
- [x] Namespace 切換功能正常（預設 all namespaces）
- [x] Context 切換並重繪畫面
- [x] Pod logs 以 `<container>|<log>` 格式串流顯示
- [x] `kubectl edit` 開啟 $EDITOR 編輯資源
- [x] Table 搜尋/篩選功能
- [x] Terminal 視窗縮放時 UI 自動配適（固定比例）
- [x] 滑鼠滾輪滾動支援
- [x] 無 runtime panic，錯誤有明確提示（! 查看 app log）

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
