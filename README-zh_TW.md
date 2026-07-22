# kbu — KubeUI

<p align="center">
  <img src="docs/icon.svg" width="128" alt="kbu icon" />
</p>

[![GitHub Release](https://img.shields.io/github/v/release/vulcanshen/kbu)](https://github.com/vulcanshen/kbu/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/vulcanshen/kbu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)
[![Kubetools](https://img.shields.io/static/v1?label=Curated&message=Kubetools&color=2a7f62)](https://collabnix.github.io/kubetools/#cluster-with-core-cli-tools)
[![Charm in the Wild](https://img.shields.io/static/v1?label=Listed%20in&message=Charm%20in%20the%20Wild&color=6B5CE7)](https://github.com/charm-and-friends/charm-in-the-wild#cloud-and-devops)

**Language**: [English](README.md) · 繁體中文

> [!WARNING]
> **v2.0 改名說明**：kbu 就是 v1.7.x 以前叫做 **km8** 的工具。使用方式全數保留 — 指令 binary 現在是 `kbu`、config 目錄從 `~/.config/km8/` 搬到 `~/.config/kbu/` 首次啟動會自動 migrate、`$KM8__*` 環境變數仍會 fallback 讀、永久保留向下相容（見環境變數表）。升級無需手動步驟。

**一個視窗搞定 Kubernetes** — `Tab` / `Space` / `Enter` / `Esc` 四鍵驅動一切，不用背快捷鍵、不用設定、零學習成本。Relatives 關聯導覽、YAML compare、常駐 shell 全都內建；其他你信任的 terminal 工具靠那個 shell 都能掛進來一起用。

> _遇事不決，就按_ **`Space`**。

## Demo

### 認識 kbu

![basics](docs/demo-basics.gif)

### 順著 Relatives 走訪 Kubernetes

![relatives](docs/demo-relatives.gif)

### 透過 Space menu 編輯叢集中的資源

![yaml-edit](docs/demo-yaml-edit.gif)

### 並排比對兩個 resource

![compare](docs/demo-compare.gif)

### Helm 是第一級的 resource

![helm](docs/demo-helm.gif)

### TUI + 常駐 shell 都在同一個視窗

![alterm](docs/demo-alterm.gif)

## 四個鍵就能操作 kbu

| 鍵 | 行為 |
|---|---|
| **`Tab`** | 切換 panel 焦點（也可以直接按 `1` / `2` / `3` 跳轉）|
| **`Enter`** | 鑽入 / 確認選擇 |
| **`Space`** | *這裡能幹嘛？* — 在每個 panel、每個 tab 上開啟對應的 menu 或 cheatsheet |
| **`Esc`** | 退回 — 回上一層 / 關閉 popup |

不知道下一步該按什麼時，按 `Space` 就對了。進階快速鍵（`P` pin / `S` sort 或 shell / `D` drag-pin 或 delete / `Alt+Shift+S` panel 2 sort / `C` compare 或 context / `Y` YAML / `E` edit / `N` ns / `>` settings）只是加速器，每一項都能透過 `Space` menu 抵達 — 想記再記，不想記也沒關係。

**滑鼠也能用**（v1.6 起）：左鍵點 panel 切焦點 + 移 cursor，雙擊鑽入，右鍵開 context menu，滾輪半頁滾動。按 `>` 開 Settings popup 可以關掉滑鼠改成純鍵盤。

## 安裝

### Quick Install（macOS/Linux）

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/install.sh | sh
```

### Quick Install（Windows PowerShell）

```powershell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/install.ps1 | iex
```

### Homebrew（macOS/Linux）

```bash
brew install vulcanshen/tap/kbu
```

### Scoop（Windows）

```powershell
scoop bucket add vulcanshen https://github.com/vulcanshen/scoop-bucket
scoop install kbu
```

### 從原始碼安裝

```bash
go install github.com/vulcanshen/kbu/cmd@latest
```

### 本地編譯

```bash
git clone https://github.com/vulcanshen/kbu.git
cd kbu
go build -o kbu ./cmd/
./kbu
```

### 解除安裝

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

kbu 會連到當前 kubeconfig 的 context。按 `Enter` 鑽入、`Space` 叫出 context menu、`Esc` 退回、`Tab` 切 panel。

靈感來自 [Lens IDE](https://k8slens.dev/)、[lazygit](https://github.com/jesseduffield/lazygit)、[lazydocker](https://github.com/jesseduffield/lazydocker) 與 [k9s](https://github.com/derailed/k9s)。以 Go 與 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 建構。

---

> 以下是操作手冊 — 想看完整功能、所有 keybinding、設定細節，繼續往下讀。

## Features

- **零學習成本** — 所有操作都會在 `Space` menu 揭露。進階快捷鍵（`P` pin / `S` sort / `C` compare / `Y` YAML / `E` edit / `N` ns / `>` settings / ...）只是加速器、你可以完全略過 — `Space` 會在當下 context 帶你走同樣的選單。完整 onboarding 文件：*「遇事不決，就按 Space」*
- **Compose，不重做** — Alterm（內嵌常駐 shell、`Alt+t`）讓任何你原本得跳出 kbu 才能跑的 terminal 工具都能在 kbu 裡同居。kbu 負責導覽和檢查，需要 write 動作時用你信任的工具 — 沒有 scrollback 切割、沒有 context switch，Alterm 在 `Alt+t` 切換之間 env / cwd / shell history 全程保留
- **Pinned resource kinds（`P` + `D` drag-and-drop）** — Panel 1 sidebar 頂端新增 Pinned 區段。在任何 resource row 按 `P` 切換 pin / unpin，順序會持久化到 config。Pin 採「**移動**」語意，不是複製 — 被 pin 的 kind 會從原本類別消失、出現在 Pinned 區，所以每個 kind 永遠只有一個位置。兩個以上 pinned kind 時，在 pinned row 按 `D` 進入 modal drag-and-drop：`j` / `k` 跟相鄰 pinned 互換、`Enter` 或 `D` 確認新順序、`Esc` 或任何其他輸入 revert 回進入時的 snapshot。Header 顯示 `Pinned 󰩐 [D]rop`、dragged row 染成 lavender、sticky toast 全程帶著鍵盤提示；drag 中按 `Space` 會開一個只剩 Drop entry 的 mini menu，給忘記合約的時候用。pin / sort / 其他 per-kind 設定共享同一個 config 區塊，CRD 短暫消失（operator 重裝等）時 pin 跟 sort 會被靜默保留、CRD 回來時自動還原
- **YAML Compare popup（`C`）** — Panel 2 列級的 diff。`C` 在 row 上把它標成 **compare anchor**（status bar 出現 glyph 標示鎖定哪一列）；`C` 在同 kind 的另一列開啟 side-by-side 或 unified 的 YAML diff；`C` 在 anchor 自己身上則取消 — 同一個鍵切三態（mark / diff / cancel）。Diff popup 有自己的 action menu（`Space`）可以即時切 layout，預設 layout（Unified）也會持久化。Compare YAML 已預先 clean（status / managedFields / resourceVersion / uid 等都拔掉），diff 聚焦在使用者真正寫的東西上
- **List-view sort（sidebar 上的 `S`、panel 2 上的 `Alt+Shift+S`）** — per-kind 多欄排序、跨 restart 持久化。選 column → 方向 → picker 自動 swap 回 column 步驟讓你直接加下一個 tier，不用重 trigger flow。每個 tier 在 panel 2 header 顯示優先順序跟方向（`Name (1) ↑ · Restarts (2) ↓ …`）；單 tier chain 收摺成只有箭頭、跟 v1.6 視覺一致。Reset row 一次清掉整條 chain；direction step 的 `Unset` 只移除一個 tier。`Esc` 是唯一的離開方式 — picker 操作中不會自動關。Comparator 自動依型別選擇：`Age` / `Updated` 用底層的 timestamp（不是顯示出來的 "5d3h" 字串）；`Ready` 解析 "N/M" 成兩個整數；`Restarts`、`Desired`、`Current`、`Up-to-date`、`Available`、`Active`、`Rev` 走 int 比較，所以 "10" 會排在 "2" 上面。Unknown column 靜默 skip，stale config 不會破壞排序。沒有 sort 設定 = `(namespace, name)` ascending，跟 kubectl 跨 namespace 的預設一致
- **滑鼠支援** — 點 panel 切焦點 + 移 cursor、雙擊鑽入（synth `Enter`）、右鍵開 row 的 context menu（synth `Space`）、滾輪半頁滾動（synth `u` / `d`）。13 個 popup 都吃滑鼠：list popup 左鍵 commit、viewer popup（YAML / Compare / App Log / Help）保留滾輪、confirm dialog 刻意把左鍵設為 no-op，避免誤觸觸發破壞性的 delete / quit / rollback。可在 Settings popup（`>`）關閉 mouse，以及用 `scroll_direction: natural | reverse` 翻轉滾輪方向
- **Settings popup（`>`）** — app-level 設定 popup，cog glyph 標題。目前包含 Mouse on/off + Scroll Direction；未來 global 設定都會放這。Popup 自己是逃生口：即使 Mouse 設為 off，popup 內仍然能用滑鼠點 — 才不會把使用者鎖在「mouse off 之後沒辦法用 mouse 打開」的死局
- **Popup layer 染色（v1.7.4）** — 每個 popup 的邊框顏色從 `lavender → sapphire` 漸層按巢狀深度取色：L1（第一層 popup）用 lavenphire25、L2（疊在第一層上的 sub-popup，例如 Compare 裡的 Diff menu、Breadcrumb 上的 Confirm）用 lavenphire50、L3 lavenphire75、L4+ sapphire。視覺上一眼就知道「這個比下面那個更上層」、不用想。Toast warn 保留 Catppuccin Peach 當警告專用色；Alterm 的 user-footprint identity 移到 statusbar marker（還是 lavender）、popup 邊框跟其他 overlay 一致
- **內建 27 種 resource + CRD 支援** — 啟動時動態探索 Custom Resources，分為 Cluster / Workloads / Network / Config / Storage / RBAC / Autoscaling / Helm 類別。Helm 類別僅在 `helm` CLI 存在於 `PATH` 時才會出現
- **即時 Watch 更新** — 透過 Kubernetes Watch API 自動更新 resource
- **Vim 風格導覽** — `j`/`k`、`u`/`d` 換頁、`gg`/`G`、`/` 搜尋
- **3-panel lazygit 風格佈局** — 有編號的 sidebar、list、detail panel，附捲動指示器
- **鑽入式導覽** — Deployment / DaemonSet / StatefulSet / Job → Pods → Containers；CronJob → Jobs；HPA → 目標 workload；PVC → 掛載中的 Pods；PDB → 受保護的 Pods；Helm Release → chart 所部署的每個原生 K8s 物件
- **Relatives tab — Lens 風格的關聯導覽** — 每個 detail panel（除了 Namespaces）都列出該 resource 可以跳轉的 reference（owners、selected pods、scaleTargetRef、mounted-by pods 等）。`Enter` 鑽入游標指向的 ref — panel 會重新繪製顯示「那個 resource」的 Relatives，形成一條鑽入鏈（Deployment → Pod → ConfigMap → 使用該 ConfigMap 的 Pods、...）。`Esc` 退回一層。`Space` 開啟 breadcrumb popup，讓你直接跳回鏈條中任何上層節點（會先確認）。Tab 標題在 depth>1 時會顯示 `Relatives N`。`Y` 開啟游標所在那筆的 YAML。內建 cycle 偵測，阻擋重複造訪祖先；fetch 失敗會 toast 通知但不改變 panel 狀態。27 種 resource 已覆蓋 26 種 — ConfigMaps / Secrets / ServiceAccounts 顯示「反向」reference（哪些 Pods 用我、哪些 RoleBindings 把這個 SA 當 subject、...）；Helm release 顯示 `Deployed Resources`，讓 chart 部署出來的每個 K8s 物件一鑽即達
- **Helm releases（當 `helm` 在 `PATH` 時）** — 專屬的 `Helm > Releases` sidebar 類別列出叢集中所有 release（每 3 秒輪詢 `helm list -A`；Helm 沒有 watch API）。Panel 2 欄位：`NAME / NAMESPACE / CHART / APP VER / REV / STATUS / UPDATED`。在 release row 上按 `Space` 開啟文件 menu（Manifest / Creator Notes / User Values / Merged Values / Hooks）；`Enter` 選定後透過 `helm get ...` fetch 並在 YAML popup 中顯示。Menu 保持在 YAML 後方，所以連續查不同文件不用重開 menu。Panel 3 把 Events 換成 `History` tab — 表格顯示每次 revision（REV / STATUS / DATE / CHART / DESCRIPTION），當前已部署的版號以 `●` 標記。在非當前 row 上按 `Space` 會問是否要 rollback；確認後彈出 popup 顯示確切的 `helm rollback` 命令並非同步執行，結果以 toast 通知。Helm 管理的 K8s 物件（label `app.kubernetes.io/managed-by: Helm` 或 annotation `meta.helm.sh/release-name`）在 panel 2 會標記 `` glyph，並擋掉 `E`（kubectl edit）顯示「Helm-managed (read-only)」toast — 請改用 `helm upgrade` / `rollback`。在非 Releases 的 panel 2 list 按 `.` 可隱藏所有 helm-managed 物件（panel 2 左下角永遠有 `.: toggle helm` 提示）
- **YAML popup（`Y`）** — 全螢幕 overlay 顯示 `kubectl get -o yaml` 的原始輸出，支援 `j/k/u/d/gg/G` 捲動、`/` 搜尋（`n`/`N` 跳到下/上一個 match、整列高亮）、`y` 把整份 YAML 複製到剪貼簿、`E` 直接從 popup 觸發 `kubectl edit`。YAML 放在 popup 而非 detail panel，避免長 YAML 在直向 layout 中換行擠成一團
- **Pod log 串流，自動 follow** — 多 container 支援，格式為 `<container>|<log>`；Logs tab 預設黏在底部。Logs tab 標題後面帶一個 Nerd Font glyph 表示 follow 狀態 — `▶`（live、U+F0753）正在追、`⏸`（paused、U+F0754）使用者捲上去暫停了。Glyph 不論這 tab 是不是 active 都會掛上、所以切 tab 時整列 tab bar 寬度不會浮動。往上捲（`k`/`↑`/`u`/`gg`）會暫停 follow 讓你看歷史；按 `G` 跳到最新並恢復 follow
- **Deployment 的聚合 log** — 選到 Deployment 時，會把當前 ReplicaSet 中的**每個 pod** 的 log 串到同一個 Logs tab（也是 Deployment detail 的預設 tab）。每行的前綴 `<pod-hash>│<container>│<text>`，各段都有穩定獨立顏色，rollout 時可以一眼看出哪個 pod 在丟錯，不必鑽下去。rollout 中 pod 變動：stream 是 row-select 當下的 snapshot；重新選 Deployment row 即可刷新。若無法查到 current ReplicaSet（如 RBAC 不允許讀 ReplicaSet）會退回用 Deployment selector
- **Status 欄位異常著色（abnormal-only）** — panel 2 的 Status 欄（每個有 Status 的 kind — Pod / Node / Namespace / PVC / PV / Helm Release）跟 Events 的 Type 欄**只**對異常值上色。**黃色**：transitional / degraded（Pending / Terminating / SchedulingDisabled / Released / pending-* / Init:*）；**紅色**：failure（Failed / Error / CrashLoopBackOff / ImagePullBackOff / NotReady / Lost / Warning）。Healthy 值（Running / Bound / Active / Deployed / Normal）維持 row 預設前景色。Color = signal、不是裝飾 — 視線只會被需要注意的 row 吸引。Cursor / lock row 落上時自動換 Catppuccin Latte 暗色版避免在淺底色洗白
- **Row 切換 debounce（300ms）** — Panel 2 j/k 連按時不再每按一下都觸發一次 detail fetch + log-stream Start。每次 row 切換 bump 一個 sequence counter、排程 300ms 後再 dispatch；連續捲過 49 row 只會發 1 次 fetch（落在你停下那個 row）。Panel 2 體感不變：cheap state mutation（停掉前一個 stream、清掉 retry throttle）還是 inline 做、只有 expensive 的 work 延後。跟 sidebar `switchSeq` debounce window 一致以維持肌肉記憶
- **透過內嵌 PTY 進行 edit / shell exec** — `E` 執行 `kubectl edit`、`S` 執行 `kubectl exec -it -- /bin/sh`，兩者都在 in-app 的 virtual terminal 中跑，編輯器和 shell session 都不會污染 host terminal 的 scrollback。Editor 遵循 `$KUBE_EDITOR` / `$EDITOR`（或 `config.yaml editor`）
- **Alterm 內嵌終端機** — `Alt+t` 在 kbu 內切換一個 embedded shell（login shell、完整 env / cwd）— 像在 popup 裡 `ssh localhost`。可以執行 `kubectl apply -f`、`helm`，所有平常你會跳出 kbu 才能跑的東西。這個 shell 是**常駐**的：在 popup 顯示中按 `Alt+t` 只會隱藏而不殺 shell；再按一次恢復（cwd、history、env、背景 job 全部保留）。Status bar 右側的 `Alterm` chip 顯示 shell 是否在背景活著。與 `kubectl edit` / `kubectl exec` 獨立 — 你可以同時讓 Alterm 跑著、又在另一個 popup 編輯 resource 或 exec 進 container
- **PTY popup 邊框走 layer 染色** — Alterm、`kubectl edit`、`kubectl exec` 全部都用 popup-layer 邊框色（深層 popup 自然取深色、沿著 `lavender → sapphire` scale）。Alterm 的「你自己的常駐 shell」identity 移到 statusbar marker（還是 lavender、user-footprint accent）、popup 邊框本身跟其他 overlay 一致。標題（`Alterm: hostname` vs `Edit: pod/foo` vs `Shell: pod/foo → ctnr`）區分用途
- **PTY scrollback** — 所有 PTY popup（Alterm、shell exec、edit）都有 10k 行歷史。`PgUp` / `PgDn` 翻頁、`Home` / `End` 跳到頂端 / live。在 alt-screen 應用（vim、less、htop）中停用，讓那些應用自己處理翻頁
- **每個 container 的 log label 各自上色** — 多 container pod 在 log 中可以逐行區分；穩定 per-container-name 上色
- **資源刪除** — `D`（大寫，hotkey 和 `Space` menu 都可觸發），附確認 dialog
- **搜尋 / 過濾** — `/` 在 sidebar、table panel、以及 namespace / context picker popup 中搜尋。Sidebar 搜尋也會比對類別名稱（例如打 "cluster" 會展開 Cluster 類別）。焦點移到其他 panel 時搜尋自動清除 — 選取會保留，filter 不會
- **剪貼簿複製（`y`）** — 透過 OSC 52 複製焦點 panel 的內容（可穿透 tmux/SSH，不需 `xclip`/`pbcopy`）。在 App Log popup（`!`）中 `y` 複製整份 log；在 YAML popup 中 `y` 複製整份 YAML
- **分級 Toast 通知** — info（1 秒、popup-layer 邊框 + `󰵅 kbu` 標題、hint 寫 `auto-dismiss`）用於像「Copied!」這類確認；warning（2 秒、Catppuccin Peach 邊框 + `󰀦 kbu` 標題）用於被擋掉的動作如 Relatives cycle 偵測、drill 失敗；sticky 版本用在 modal state 契約（drag mode hint）— hint 換成 `Esc: close`、不自動消失
- **Namespace 與 context 切換** — `N` 切 namespace、`C` 切 context（大寫 — trigger key 一律大寫以避免在 `/` 搜尋輸入時誤觸）
- **Session-local context** — 在 kbu 中切 context 不會碰 `~/.kube/config`。可以同時在另一個終端機跑 `kubectl` 而不互相影響
- **面板感知的選取樣式** — 有焦點的 panel cursor row 用明亮的 lavender chip；*無焦點* 的 panel 選取 row 保留柔和的 bg + 粗體，這樣不管你在哪個 panel 工作，都能看清楚每個 panel「記得」哪個 resource。無焦點 panel 整體 dim 到 overlay 灰，視覺重心自然落在當前焦點 panel 上，但其他 panel 的「記憶位置」仍然可見
- **Detail tabs** — panel 3 的 tab list 依 kind 而不同。**Workload kinds**（Pods / Deployments / StatefulSets / DaemonSets / Jobs / CronJobs）的 tab 從 `Logs` 開頭、因為換 row 時最常想看的就是「這東西現在在幹嘛」— Relatives 是刻意的 drill 動作、值得多按一下 tab 切換。順序為 `Logs` / `Relatives` / `Events` / `Conditions`（Conditions 只在有 `.status.conditions` 的 kind 出現）。非 workload kind 仍以 `Relatives` 開頭、這樣 `Space` 從 Relatives entry 跳回時會落在你進來時那個 tab。Helm release：`Relatives` / `History`。Panel 3 沒有 `/` 搜尋 — cursor-based tab（Relatives / History）不適合 row 過濾、Logs 也是直接看 follow-tail 比較順；要搜大段內容請用 `Y` + 你的編輯器
- **長字串自動換行、不截斷** — YAML、Events、Logs 都適用；panel 大小變動時換行點會重新計算
- **Panel 全螢幕展開** — `z` 切換有焦點的 Table 或 Detail panel 全螢幕；再按一次 `z` 還原 3-panel 配置
- **Theme 系統** — 在 config 目錄丟一個 `theme.yaml` 覆寫顏色
- **Help 與 App Log overlay** — `?` / `!` 在主 UI 上方彈出 popup
- **錯誤通知** — status bar badge + status line 訊息
- **Crash 記錄** — panic 寫入 kbu log 目錄
- **Audit 記錄** — 每次 `kubectl edit` 與 `kubectl delete` 都記到 `audit-*.log`

## Key Bindings

### 主要互動：四個鍵

大多數時候，你只用這四個鍵就能操作 kbu：

| 鍵 | 行為 |
|---|---|
| **`Tab`** | **Panel** — 把焦點移到下一個 panel（也可以直接按 `1` / `2` / `3` 跳轉）|
| **`Enter`** | **Into** — 鑽入選中的 resource / 確認 popup 選擇。**不會**把焦點推到其他 panel（要切焦點請用 `Tab` / `1` / `2` / `3`）|
| **`Space`** | **Menu** — 在當下焦點處開啟對應 popup：sidebar cheatsheet + Pin / Unpin / Sort / Drag 動作（panel 1）、每列的 action menu 分上下半 item 操作 + panel 級 Sort entry / container Shell menu / 空 list 提示（panel 2）、Logs / Events / Relatives-drill / Relatives-breadcrumb / History rollback（panel 3 各 tab）。也可關閉任何已開啟的 popup（鏡像式開關）|
| **`Esc`** | **Back** — 退回一層 / 關閉 popup |

只要有 context menu 存在的位置，`Space` 就足夠了 — 不需要記每個動作的 hotkey。

Tab 導覽還支援 `h`/`l`（或 `[`/`]`）切換 panel 3 的 tab。

### 滑鼠（v1.6 起）

| 操作 | 行為 |
|---|---|
| **左鍵** 點 panel row | 切焦點到該 panel + cursor 移到該列 |
| **雙擊** | Synth `Enter`（鑽入 cursor 那列）|
| **右鍵** 點 row | Synth `Space`（開該列的 context menu / cheatsheet）|
| **滾輪** 上 / 下 | Synth `u` / `d`（半頁移動）。方向可在 Settings popup 切換 `scroll_direction: natural | reverse` |
| **左鍵** 點 list popup 的列 | Commit 該列（等同於 cursor + `Enter`）|
| **右鍵** 點任何 popup | 關閉它（等同於 `Esc`）|

Menu 類 popup（panel 2 menu、sort picker、namespace / context picker、breadcrumb、helm doc menu、hint、settings、confirm）忽略滾輪 — 內容短、半頁滾動沒意義。Viewer popup（YAML / Compare / App Log / Help）**會**吃滾輪。Confirm dialog 刻意把左鍵設為 no-op，避免誤觸觸發破壞性的 delete / quit / rollback — 確認只能用鍵盤 `Enter` / `y`。

可以在 Settings popup（`>`）關閉滑鼠；popup 本身在 mouse off 時還是可以滑鼠操作，方便切回 on。

### 加速器 — cursor 與 power trigger

```
 cursor    j k         u d         gg G        / (在當前 panel 內搜尋)
 trigger   Y YAML      E edit      N namespace
 panel 1   P pin       S sort      D drag-and-drop pinned (modal)    C context
 panel 2   S shell     Alt+Shift+S sort    D delete    C compare anchor
 expand    z           z 切換當前 panel 全螢幕
 helm      .           . 切換 panel 2 中 helm-managed 物件顯示
 settings  >           > (shift+.) 開啟全域 Settings popup
```

`S` / `C` / `D` 是 panel-aware 雙重綁定 — 同字母在不同 panel 做不同事，跟 `P` 只在 panel 1 有意義是同邏輯。Panel 2 的 sort 需要 `Alt+Shift+S` 組合鍵、因為單 `S` 已經是 Shell — 加 modifier 騰出 panel 2 的 sort 鍵位、不破壞 Shell 既有 muscle memory。Trigger 鍵刻意設成大寫，避免在 `/` 搜尋輸入時誤觸。

### 全域

| 鍵 | 動作 |
|---|---|
| `>` | 開啟全域 Settings popup（mouse on/off、scroll direction；未來更多設定）|
| `Alt+t` | 切換 Alterm（spawn / 顯示 / 隱藏；shell 在隱藏時保持存活）|
| `y` | 把焦點 panel 內容複製到剪貼簿（OSC 52）|
| `!` | App log |
| `?` | Help |
| `q` | 結束 kbu（會確認）|
| `Ctrl+C` | 立即結束 kbu（不確認）|

### Panel 1 sidebar Space menu

當條件成立時，action 區會分成兩個帶 header 的群組：

| 鍵 | 動作 |
|---|---|
| `P` | **item operation** — Pin / Unpin cursor 那個 resource kind。被 pin 的 kind 會出現在頂端 "Pinned" 區、並**從**原本類別移走。順序 per-context 持久化到 config |
| `S` | **item operation** — 對 cursor 那個 kind 開啟 Sort flow。Column picker 跟 direction picker 之間互相 loop，多欄 chain 不用反覆 trigger，Reset row 一次清整條 chain |
| `D` | **panel operation** — 進入 drag-and-drop 重排模式。只有當 cursor 在 pinned row 且有至少另一個 pinned kind 可交換時才會出現 |

### Panel 2 context menu（在任一 row 按 `Space`）

依 resource 提供對應動作的 per-row menu — `Y` YAML / `E` Edit / `S` Shell / `D` Delete，加一個情境感知的 **`C` Compare** entry（Mark anchor / Compare to anchor / Unmark anchor，依目前狀態）。下方再用分隔線切出 **panel operation** 區：`[Alt][S]ort panel 2 list` 開啟跟 panel 1 Space menu 同一條的 column picker、scope 是目前 panel 2 顯示的 kind。用 `j`/`k` + `Enter`，或直接按字母觸發。Helm-managed row 會隱藏 `E`/`D`（Rule A：read-only — 即使編輯也會被 `helm upgrade`/`rollback` 蓋掉）；沒有 container 的 resource 會隱藏 `S`。

### Compare mode

Anchor 已設時，panel 2 左下角會顯示 `esc: exit compare` 提示。被鎖定的 row 用 Mocha lavender（粗體反白）上色 — 跟 Pinned items 和 Settings ON toggle 同色、屬於「user-set 在這 row 的狀態」。Status bar 上固定寬度的 `<icon> Compare` chip 標示 mode 開著、不再把 resource 名硬塞進去（popup 自己會顯示 `left vs right`）。Compare lock 在焦點離開 panel 2、或鎖定的 row 從 watcher 流中消失（被刪除 / namespace 過濾掉）時自動清除。

### Helm 專用

| 鍵 | 位置 | 動作 |
|---|---|---|
| `Space` | Panel 2、Release row | 開啟文件 menu — 選 `Manifest` / `Notes` / `User Values` / `Merged Values` / `Hooks` |
| `Space` | Panel 3、History tab、非當前 row | rollback 到該版本（確認 popup 會顯示確切的 `helm rollback` 命令）|
| `.` | 任何非 Releases 的 panel 2 list | 切換 helm-managed 物件的可見性 |

### PTY popups（Alterm、edit、shell exec）

| 鍵 | 動作 |
|---|---|
| `PgUp` / `PgDn` | 歷史以一頁為單位捲動 |
| `Home` / `End` | 跳到歷史頂端 / 回到 live |
| 其他任何鍵 | 跳回 live、按鍵轉發給 subprocess |

當 full-screen app（vim、less、htop）透過 alt-screen 接管 PTY 時，scrollback 會停用 — 那些按鍵會轉發給 app，讓 app 自己處理翻頁。

## 編輯 Resource

在 resource 上按 `E`（或從 `Space` menu 選 `Edit`）會在 embedded PTY popup 中執行 **`kubectl edit <kind>/<name> -n <ns> --context <ctx>`**。行為與在 terminal 中跑同樣的指令完全一致：strategic merge patch、`resourceVersion` 衝突偵測、沒有 `last-applied-configuration` annotation 的副作用。

Editor 由 kubectl 自己依以下順序決定：

1. `$KUBE_EDITOR`（如果 `config.yaml` 設了 `editor`，kbu 會自動 export）
2. `$EDITOR`
3. `vi`（Linux/macOS）或 `notepad`（Windows）

Editor 結束時，popup 關閉、table 透過 resource watch 自動刷新 — 不需手動 reload。

### 為什麼要用內嵌 PTY？

早期版本的 kbu 透過 `tea.ExecProcess` 跑 editor，再用 `kubectl apply -f` 套用結果。那個做法會在離開 kbu 後把 kubectl 的確認訊息漏進 host terminal 的 scrollback，而 apply 與 edit 的語意差異也常讓習慣 `kubectl edit` 的使用者感到困惑。PTY popup 讓一切留在 kbu 內，並且直接用 `kubectl edit`，所以行為跟使用者預期一致。

### nvim 使用者注意

如果你的 nvim 在 popup 內有明顯的退出延遲（LSP attach/detach、plugin teardown），可以在 `config.yaml` 設 `editor: "nvim --noplugin"`，只在 kubectl-edit session 中跳過 plugin 載入。你平常的 `nvim` 不受影響。

## Context 隔離

kbu 維護自己的 **session-local** context。在 kbu 內用 `C` 切 context **不會** 改動 `~/.kube/config`，也不會影響其他終端機的 `KUBECONFIG` 環境變數。

kbu 啟動的所有 `kubectl` subprocess（edit、delete、shell exec）都會帶上明確的 `--context <name>` flag，所以它們永遠對著 kbu 顯示中的 cluster — 與 `kubectl` 預設 context 是什麼無關。

所以你可以放心地一邊用 kbu、一邊在另一個終端機用 `kubectl`，兩邊 context 互不干擾。

## 設定

設定檔放在 OS 對應的 config 目錄。設 `XDG_CONFIG_HOME` 可以在任何平台覆寫：

| OS | 預設路徑 |
|---|---|
| Linux | `$XDG_CONFIG_HOME/kbu/` 或 `~/.config/kbu/` |
| macOS | `~/Library/Application Support/kbu/` |
| Windows | `%APPDATA%/kbu/` |

Log（crash 與 audit）寫到 config 目錄下的 `logs/` 子目錄。

### config.yaml

```yaml
default_context: ""      # kubeconfig context（預設：current-context）
default_namespace: ""    # namespace 過濾（預設：all namespaces）
editor: ""               # 以 $KUBE_EDITOR 形式 export 給 kubectl
                         # （預設：kubectl 會 fallback 到 $EDITOR → vi / notepad）
alterm_shell: ""         # Alterm 啟動的 shell（預設：$SHELL → /bin/sh）。
                         # 純名字（如 `fish`）在 popup 開啟時走 $PATH 查找
                         # （Go `exec.Command` 語意）、絕對路徑直接使用。
                         # 可在 alterm 內用 fish 但 host shell 維持 zsh。
alterm_login_shell: false # 設 true 時 Alterm 用 `-l` 啟動、會 source
                         # ~/.zprofile / ~/.bash_profile / /etc/profile。
                         # 預設 false 對齊 v1.7.2 基線（non-login interactive
                         # — .bashrc/.zshrc 仍會跑、但不會被 /etc/profile
                         # 強塞 PS1）。當 kbu 是從非 login 父 shell 啟動
                         # （Raycast、Alfred、cron、非預設 tmux）且 PATH
                         # 在 .zprofile 而不是 .zshrc 時請開啟。

# Compare popup 預設值（v1.6+）。`layout` 選 diff render：
# "unified"（預設）是單欄附 -/+ 標記，"split" 是左右並排。
compare:
  layout: unified

# 滑鼠設定（v1.6+）。兩個欄位都是 optional，省略則 fallback 到下列預設。
mouse_opt_config:
  enabled: true                # 設 false 關閉 click + double-click + right-click + wheel
  scroll_direction: natural    # "natural": 滾輪上 = cursor 上。"reverse" 翻轉對應。

# Per-kind 偏好設定（v1.6+）。Key 是 kubectl name
# （"pod" / "deployment" / "configmap" / ...）。每個 entry 都 optional，
# 未知 kind 在 rewrite 時會被保留，CRD 短暫消失時 pin / sort 不會掉。
#
# Sort 自 v1.7 起變成多 tier chain — tier 0 為主排序、tier 1 為第一
# tiebreaker，依此類推。v1.6 單一 mapping 形式仍接受，載入時自動 lift
# 為 1-tier chain，下次 save 時以新 sequence 形式寫回。
resource_kind_config:
  pod:
    pinned:
      order: 10              # sparse — 10 為增量，方便手動在兩個 pin 之間插入
    sort:
      - column: Restarts     # 該 kind 在 panel 2 顯示的 column title
        direction: desc      # "asc" 或 "desc"
      - column: Name         # tier 1 — Restarts 相同時的 tiebreaker
        direction: asc
  configmap:
    pinned:
      order: 20
    sort:                    # 單一 tier chain 也合法
      - column: Age
        direction: desc
```

### 環境變數

這些變數會 override 對應的 config 欄位，用於不改 YAML 的一次性執行 — 適合 CI、demo 腳本、臨時試另一個 shell 的場合。

> **v2.0 改名說明**：下表的 `KBU__*` 是 pre-v2.0 `KM8__*` 的新名。舊 `KM8__*` 仍會 fallback 讀（永久保留、向下相容）— 舊 `~/.zshrc` 裡的 `KM8__CONFIGPATH` 可以繼續用。同時設 `KBU__` 與對應 `KM8__` 時，`KBU__` 勝出。

| 變數 | 作用 | 優先順序 |
|---|---|---|
| `KBU__CONFIGPATH` | 改用這個檔案作為 config file，繞過預設 layout（`$XDG_CONFIG_HOME/kbu/config.yaml` 等）。Theme file 路徑**不受影響**、仍在 OS config 目錄下。建議用絕對路徑；相對路徑會在 load/save 當下對 CWD 解析。 | `KBU__CONFIGPATH` > 預設 layout |
| `KBU__ALTERM_SHELL` | 改用這個 binary 作為 Alterm 的 shell。純名字會在 popup 開啟時走 `$PATH` 查找（Go `exec.Command` 語意）、絕對路徑直接 exec。前後空白會被 trim。 | `KBU__ALTERM_SHELL` > `alterm_shell` config > `$SHELL` > `/bin/sh` |
| `KBU__ALTERM_LOGIN_SHELL` | 強制 Alterm shell 進入或退出 login mode（`-l`）。Truthy 值：`true` / `1` / `yes`（大小寫都接受）。其他值關閉 login mode。當從非 login 父 shell 啟動而 PATH 在 `.zprofile` 時使用。 | `KBU__ALTERM_LOGIN_SHELL` > `alterm_login_shell` config > `false` |

範例：

```sh
# 不改 config.yaml、臨時在 Alterm 試 fish
KBU__ALTERM_SHELL=/opt/homebrew/bin/fish kbu

# 指向專案內 config（例如 commit 到 repo 的 .kbu.yaml）
KBU__CONFIGPATH="$PWD/.kbu.yaml" kbu
```

### theme.yaml

放一份 `theme.yaml` 即可自訂顏色。只需覆寫你想動的欄位 — 未指定的欄位會用預設。

```yaml
sidebar:
  background: ""                       # 留空 = 終端機透明
  foreground: "#cdd6f4"
  selected_bg: "#bac2de"               # 焦點 panel 的 cursor bg（reverse-video）
  selected_fg: "#1e1e2e"
  unfocused_selected_bg: "#353648"     # 其他 panel「記住」的選取 bg
  unfocused_selected_fg: "#cdd6f4"
  category_fg: "#89b4fa"

table:
  header_bg: "#313244"
  header_fg: "#89b4fa"
  row_fg: "#cdd6f4"
  selected_row_bg: "#bac2de"           # 焦點 panel 的 cursor bg（reverse-video）
  selected_row_fg: "#1e1e2e"
  unfocused_selected_row_bg: "#353648" # 其他 panel「記住」的選取 bg
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

## 需求

- **kubectl** 在 `$PATH` 上（給 edit、delete、shell exec 用）
- 有效的 **kubeconfig**（`~/.kube/config` 或 `$KUBECONFIG`）
- 一個運作中的 Kubernetes cluster
- **Nerd Font 的 Mono 變體**（例：JetBrains Mono Nerd Font Mono、FiraCode Nerd Font Mono）。kbu 的 popup title + row marker 使用 Material Design 系列的 NF glyph，Mono 變體把每個 glyph 設計成正好 1 cell，column / border 對齊穩定。非 Mono（proportional）變體或 East-Asian-Ambiguous=double 的 terminal（部分 tmux + iTerm2 的 CJK 設定）會把這些 glyph 畫成 2 cell — kbu 還能跑、但 helm-managed row + popup 頂邊可能差 1 cell。看到對齊跑掉就換 Mono 變體或把 ambiguous-width 設成 single。

## License

[GPL-3.0](LICENSE)
