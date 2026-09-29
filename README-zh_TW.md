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

![basics](docs/demo-basics.gif)

## Features

### 到處走走

- **Relatives** — 每個 resource 都會列出它連到哪些東西：它的 owner、Service 選到的 Pods、HPA 在 scale 的 workload、掛載某個 PVC 的 Pods、用到某個 ConfigMap 或 Secret 的 Pods。`Enter` 順著連結走、`Esc` 退回一步，`B`（**Breadcrumb**，`Space` menu 裡也有）可以直接跳回走過的鏈上任何一點。
- **鑽入** — Deployment / StatefulSet / DaemonSet / Job → Pods → Containers，CronJob → Jobs，HPA → 目標 workload，Helm release → chart 部署出來的每個物件。
- **內建 28 種 resource，外加你的 CRD** — Custom Resource 啟動時自動探索，所有列表透過 Kubernetes Watch API 即時更新。
- **多 namespace 檢視** — 在 `N` picker 勾選任意幾個 namespace，或選「All Namespaces」。kbu 會記住你的選擇。
- **Pin 與排序** — 把最常用的 resource kind 釘到 sidebar 最上面、拖曳排好順序；任何列表都能依一欄或多欄排序。兩者都按 kind 分別保存。
- **搜尋** — `/` 可以過濾 sidebar、resource 列表，以及 namespace / context picker。
- **從上次離開的地方繼續** — 關掉再開，會回到同一個 context、namespace、resource、row、panel 和 tab。

### 看仔細一點

- **Logs** — 自動追最新一行，往上捲就暫停，按 `G` 回到 live。選到 workload 時，它底下**每個 Pod** 的 log 會匯流到同一個畫面，每個 Pod 和 container 各有自己的顏色 — rollout 時一眼就看得出是哪個在出錯。
- **Events** — 在 workload 上，會把它自己的 events 和底下 Pods 的 events 合併（CronJob 連 Jobs 的也一起），最新的排最前面。
- **Conditions** — 以表格呈現 `.status.conditions`，就是 `kubectl describe` 裡那一段。events 過期之後特別有用。
- **YAML 檢視（`Y`）** — vim 風格的 buffer：`h/j/k/l`、`w/b` 移動、`/` 搜尋、`v` 選取、`y` 複製。
- **比對（`C`）** — 比對同一種 kind 的兩個 resource，可選 unified 或左右並排（`L` 切換，下次打開沿用你的選擇）。status 和伺服器管理的欄位都會先拿掉，你看到的只有真正寫進去的內容。
- **有問題的一眼就看到** — status 欄只替需要注意的值上色：黃色是 pending 或降級、紅色是失敗。健康的 row 維持原色。
- **Helm releases** — `helm` 在 `PATH` 上時，release 有專屬的檢視：manifest、values、notes、hooks；revision 歷史，一鍵 rollback。chart 管理的物件會被標記、擋掉誤編輯，也可以按 `.` 隱藏。
- **KubeConfig contexts** — 唯讀檢視你的 kubeconfig；在 context 上按 `Enter` 把 kbu 切換過去。憑證永遠不會顯示。

### 動手做事

- **編輯（`E`）** — 在 kbu 內的 terminal popup 執行真正的 `kubectl edit`，使用你的 `$KUBE_EDITOR` / `$EDITOR`。
- **Shell（`S`）** — `kubectl exec` 進 container，同樣在 kbu 內。
- **刪除（`D`）** — 一定會先確認；刪 namespace 會多一道警告。
- **Alterm（`Alt-t`）** — kbu 內的常駐 shell。同一個鍵隱藏、再按一次叫回來，目錄、history、執行中的 job 都還在；`Alt-Esc` 結束它。kbu 沒做的事，交給你平常用的工具在這裡做。
- **複製（`y`）** — 透過 OSC 52 把目前這一列或整個畫面複製到剪貼簿，走 SSH 或 tmux 也能用。
- **Audit log** — 從 kbu 做的每次編輯與刪除都會留下紀錄。

### 配合你的環境

- **Session-local context** — 在 kbu 裡切 context 不會動到 `~/.kube/config`，另一個終端機的 `kubectl` 不受影響。
- **滑鼠** — 點擊切焦點與選取、雙擊鑽入、右鍵開 menu、滾輪捲動。偏好純鍵盤的話可以在 Settings（`>`）關掉。
- **全螢幕（`z`）** — 放大列表或 detail panel，再按一次 `z` 還原。
- **主題** — 用 `theme.yaml` 覆寫任何顏色。

## 安裝

### 需求

- **kubectl** 在 `$PATH` 上（給 edit、delete、shell exec 用）
- 有效的 **kubeconfig**（`~/.kube/config` 或 `$KUBECONFIG`）
- 一個運作中的 Kubernetes cluster
- **Nerd Font**。kbu 啟動時會量 icon 在你的終端機上佔幾格、照著排版，icon 畫成兩格寬的字型也對得齊。
- **支援 truecolor（24-bit 色）的終端機**。kbu 的淡色與疊起來的 popup 之間的明暗，在 256 色下分不出來。

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

從原始碼建置寫在 [`docs/dev-remarks.md`](docs/dev-remarks.md)。

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

## 四個鍵就能操作 kbu

| 鍵 | 行為 |
|---|---|
| **`Tab`** | 切換 panel 焦點（也可以直接按 `1–3` 跳轉）|
| **`Enter`** | 對選到的東西做最直觀的事：鑽入（workload → 它的 pods）、打開不能鑽入的種類的 YAML、切換到 kubeconfig context、shell 進 container、rollback 到 Helm 版本。在 panel 1 把那個種類顯示到 panel 2；在 Logs / Events / Conditions tab 把 panel 放到全螢幕 |
| **`Space`** | *這裡能幹嘛？* — 列出這個 panel 或 tab 能做的每一件事，最後一列 **Global operation** 是全域動作（namespace、context、Alterm、settings、app log、離開）|
| **`Esc`** | 退回 — 回上一層 / 關閉 popup |

不知道下一步該按什麼時，按 `Space` 就對了。進階快速鍵（`P` pin / `S` sort 或 shell / `D` drag-pin 或 delete / `Alt-S` panel 2 sort / `C` compare 或 context / `Y` YAML / `E` edit / `N` ns / `>` settings）只是加速器，每一項都是 `Space` menu 裡的一列（全域的在它的 **Global operation** 那一列裡）— 想記再記，不想記也沒關係。暫時不能執行的列會變暗，而不是藏起來；`?` 列出的按鍵也一樣。popup 疊上來時只有你正在操作的那一個是亮的，底下的一切都變暗。

**滑鼠也能用**：左鍵點 panel 切焦點 + 移 cursor，雙擊鑽入，右鍵開 context menu，滾輪半頁滾動。按 `>` 開 Settings popup 可以關掉滑鼠改成純鍵盤。

## Key Bindings

`h/l`（或 `[`/`]`）切換 panel 3 的 tab。

### 快速鍵

以下每一項在 `Space` menu 裡都找得到 — 快速鍵只是更快。

```
 cursor    j/k         u/d         gg/G        / (在當前 panel 內搜尋)
 trigger   Y YAML      E edit      N namespace
 panel 1   P pin       S sort      D drag-and-drop pinned (modal)    C context
 panel 2   S shell     Alt-S sort          D delete    C compare anchor
 expand    z           z 切換當前 panel 全螢幕
 helm      .           . 切換 panel 2 中 helm-managed 物件顯示
 settings  >           > (Shift-.) 開啟全域 Settings popup
```

`S`、`C`、`D` 依焦點所在的 panel 做不同的事。快速鍵一律大寫，在搜尋欄打字時才不會誤觸。

### 全域

| 鍵 | 動作 |
|---|---|
| `>` | 開啟全域 Settings popup（mouse on/off、scroll direction）|
| `Alt-t` | 切換 Alterm（啟動 / 顯示 / 隱藏；隱藏時 shell 保持存活）|
| `y` | 複製到剪貼簿（OSC 52）— 有 cursor 時複製那一列，否則複製整個內容 |
| `!` | App log |
| `?` | 最前面那個東西能按的鍵 —— focus 的 panel，或最上層的 menu、popup、模式。再按一次 `?` 或 `Esc` 關閉 |
| `q` | 結束 kbu（離開時會保存 session 狀態）— 在任何 panel、menu、popup 上都有效；在搜尋欄打字時只是一個字母 |
| `Ctrl-C` | 跟 `q` 一樣，打字時也有效 |

`N`、`C`、`Alt-t`、`>`、`!`、`q` 也都是 global operation popup 裡的列：每個 `Space` menu 的最後一列打開它。

### 滑鼠

| 操作 | 行為 |
|---|---|
| **左鍵** 點 panel row | 切焦點到該 panel + cursor 移到該列 |
| **雙擊** | 在 panel 2、3 等同 `Enter`；在 panel 1 只選列 |
| **右鍵** 點 row | 等同 `Space`（開那一列的 `Space` menu）|
| **滾輪** 上 / 下 | 等同 `u/d`（半頁移動）。方向可在 Settings popup 切換 `scroll_direction: natural | reverse` |
| **左鍵** 點 list popup 的列 | 選定該列（等同 cursor + `Enter`）|
| **右鍵** 點任何 popup | 關閉它（等同 `Esc`）|

可以在 Settings popup（`>`）關閉滑鼠；popup 本身在 mouse off 時還是可以滑鼠操作，方便切回 on。

### Helm 專用

| 鍵 | 位置 | 動作 |
|---|---|---|
| `Space` | Panel 2、Release row | `Space` menu 在 `YAML` 旁邊列出 release 的文件 — `Manifest` / `Creator Notes` / `User Values` / `Merged Values` / `Hooks`；每一份都開在 menu 上面，可以連看幾份 |
| `Enter` | Panel 3、History tab | rollback 到那個版本（`Space` menu 裡也有；目前部署的版本上不作用）；確認 popup 會顯示確切的 `helm rollback` 命令 |
| `.` | 任何非 Releases 的 panel 2 list | 切換 helm-managed 物件的可見性 |

### PTY popups（Alterm、edit、shell exec）

| 鍵 | 動作 |
|---|---|
| `Alt-t` | Alterm：隱藏（shell 繼續跑）|
| `Alt-Esc` | 關閉 terminal —— 先問，再結束那個 session：Alterm 的 shell 與裡面跑的東西會停掉；還沒存的 `kubectl edit` 會丟掉 |
| `PgUp/PgDn` | 歷史以一頁為單位捲動 |
| `Home/End` | 跳到歷史頂端 / 回到 live |
| 其他任何鍵 | 跳回 live、按鍵轉發給 subprocess |

當 full-screen app（vim、less、htop）透過 alt-screen 接管 PTY 時，scrollback 會停用 — 那些按鍵會轉發給 app，讓 app 自己處理翻頁。

terminal 用滿整個畫面，四邊各留一格；其他 popup 最寬 120 欄。

## 編輯 Resource

在 resource 上或它的 YAML viewer 裡按 `E`（或從 `Space` menu 選 `Edit`），先確認，再在 embedded PTY popup 中執行 **`kubectl edit <kind>/<name> -n <ns> --context <ctx>`**。行為與在 terminal 中跑同樣的指令完全一致：strategic merge patch、`resourceVersion` 衝突偵測、沒有 `last-applied-configuration` annotation 的副作用。

Editor 由 kubectl 自己依以下順序決定：

1. `$KUBE_EDITOR`（如果 `config.yaml` 設了 `editor`，kbu 會自動 export）
2. `$EDITOR`
3. `vi`（Linux/macOS）或 `notepad`（Windows）

Editor 結束時，popup 關閉、table 透過 resource watch 自動刷新 — 不需手動 reload。

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

kbu 另外會在 `config.yaml` 旁邊放一份 `state.yaml`，用來記住你上次離開的位置。它由 kbu 自動管理；要調整設定請改 `config.yaml`。

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
                         # 若 kbu 是從 launcher（Raycast、Alfred…）啟動、
                         # 而你的 PATH 設在 .zprofile，請開啟。

# Compare popup 預設值。`layout` 選 diff render：
# "unified"（預設）是單欄附 -/+ 標記，"split" 是左右並排。
# 在 popup 裡按 `L` 會把選擇寫回這裡。
compare:
  layout: unified

# 滑鼠設定。兩個欄位都是 optional，省略則 fallback 到下列預設。
mouse_opt_config:
  enabled: true                # 設 false 關閉 click + double-click + right-click + wheel
  scroll_direction: natural    # "natural": 滾輪上 = cursor 上。"reverse" 翻轉對應。

# Per-kind 偏好設定。Key 是 kubectl name
# （"pod" / "deployment" / "configmap" / ...）。每個 entry 都 optional，
# 未知 kind 在 rewrite 時會被保留，CRD 短暫消失時 pin / sort 不會掉。
#
# Sort 是多 tier chain — tier 0 為主排序、tier 1 為第一 tiebreaker，依此類推。
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
| `KBU__STATEPATH` | 改用這個檔案作為 session state file，取代 `<config-dir>/state.yaml`。適合想讓每次執行各有獨立 state、又不想動到真正 state file 的沙盒 / 測試場合。 | `KBU__STATEPATH` > 預設 layout |
| `KBU__ALTERM_SHELL` | 改用這個 binary 作為 Alterm 的 shell。純名字會在 popup 開啟時走 `$PATH` 查找（Go `exec.Command` 語意）、絕對路徑直接 exec。前後空白會被 trim。 | `KBU__ALTERM_SHELL` > `alterm_shell` config > `$SHELL` > `/bin/sh` |
| `KBU__ALTERM_LOGIN_SHELL` | 強制 Alterm shell 進入或退出 login mode（`-l`）。Truthy 值：`true` / `1` / `yes`（大小寫都接受）。其他值關閉 login mode。當從非 login 父 shell 啟動而 PATH 在 `.zprofile` 時使用。 | `KBU__ALTERM_LOGIN_SHELL` > `alterm_login_shell` config > `false` |
| `KBU__ICON_WIDTH` | Nerd Font 的 icon 在你的終端機上佔幾格：`1` 或 `2`。蓋過 kbu 啟動時的檢查；Windows 沒有檢查，要 `2` 只能靠它。其他值不理會。 | `KBU__ICON_WIDTH` > 啟動時檢查的結果 > `1` |

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
  header_bg: ""                        # 留空 = 直接落在 panel 底色上，只靠前景色區分 header
  header_fg: "#89b4fa"
  row_fg: "#cdd6f4"
  selected_row_bg: "#bac2de"           # 焦點 panel 的 cursor bg（reverse-video）
  selected_row_fg: "#1e1e2e"
  unfocused_selected_row_bg: "#b4befe" # 其他 panel「記住」的選取 bg — Catppuccin lavender chip
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
  background: ""                       # 留空 = 終端機透明
  foreground: "#cdd6f4"
  context_fg: "#89b4fa"

status_line:
  background: ""                       # 留空 = 終端機透明
  foreground: "#89b4fa"

status:
  running: "#a6e3a1"
  pending: "#f9e2af"
  error: "#f38ba8"
  unknown: "#7f849c"
```

## 限制

- **框線還是偏 1 格時**，是 kbu 啟動時的檢查（畫一個 icon、問終端機游標停在哪）沒得到回應或回應不對。`kbu iconwidth` 會印出 kbu 量到的格數；icon 畫成兩格寬就設 `KBU__ICON_WIDTH=2`，否則設 `KBU__ICON_WIDTH=1`。Windows 沒有這個檢查：字型把 icon 畫成兩格寬時要自己設。
- **Helm 需要 `helm` CLI。** `helm` 在 `PATH` 上時才會出現 Helm 分類，release 列表每 3 秒更新一次（Helm 沒有 watch API）。
- **workload 的 log 跟的是選到那一列當下存在的 Pod。** rollout 之後，重新選一次那一列才會接上新的 Pod。
- **有些刪除與編輯被擋下。** Events 與 Nodes 不能從 kbu 刪除；helm-managed 物件不能編輯或刪除 —— 請用 `helm upgrade` / `rollback` / `uninstall`。
- **panel 3 沒有 `/` 搜尋。** 用 `Y` 打開 YAML，在那裡搜尋。

## 相關連結

- [CHANGELOG.md](CHANGELOG.md) — 每個版本的變更
- [`docs/dev-remarks.md`](docs/dev-remarks.md) — 開發者備忘錄：怎麼運作、為什麼、建置與測試

## terminu family

kbu 遵循 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.21/principle)：跟家族其他成員一樣的按鍵、一樣的 menu —— [filu](https://github.com/vulcanshen/filu)（檔案）、[sshu](https://github.com/vulcanshen/sshu)（ssh）、[webu](https://github.com/vulcanshen/webu)（網頁）、[locku](https://github.com/vulcanshen/locku)（螢幕鎖）。

## License

[GPL-3.0](LICENSE)
