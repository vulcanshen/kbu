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

**終端機裡的一頁式 Kubernetes 工作區。** 左邊是資源種類，中間是游標所在那一種的資源，下面是游標那一列的一切 —— 它的
log、它連到哪些東西、它的 events —— 跟著叢集即時更新。在哪裡都可以按 `Space` 看能做什麼，按 `?` 看按鍵，按 `Esc` 退一步。
Relatives 導覽、YAML 比對與內嵌的 shell 都是內建的；kbu 沒做的事，用你慣用的工具在那個 shell 裡做。

> _不確定的時候，按_ **`Space`**。

![basics](docs/demo-basics.gif)

## 特色

### 找到路

- **Relatives** —— 每個資源都列出它連到的東西：它的 owner、Service 選到的 Pod、HPA 擴縮的 workload、掛載 PVC 的 Pod、
  用到 ConfigMap 或 Secret 的 Pod。`Enter` 跟著連結走，`Esc` 退回一步，`[B]readcrumb` 把 panel 1、2 切到你走過的鏈上任何一點。
- **Drill-down** —— Deployment / StatefulSet / DaemonSet / Job → Pods → Containers、CronJob → Jobs、HPA → 它的目標、
  Helm release → chart 部署的每一個物件。
- **28 種內建資源，加上你的 CRD** —— 啟動時找出 Custom Resources，每一份清單都透過 Kubernetes 的 watch 即時更新。
- **一次看好幾個 namespace** —— 在 `[N]amespace` picker 勾任意幾個，或是 All Namespaces。kbu 記得你的選擇。
- **釘選與排序** —— 把常用的種類釘在 panel 1 最上面、拖成你要的順序；任何清單都能依一欄或好幾欄排序。兩者都依種類保存。
- **篩選** —— `/` 在 panel 1、panel 2 邊打邊篩，namespace 與 context 的 picker 也可以。
- **從上次離開的地方接著用** —— 離開再開：同一個 context、namespace、種類、那一列、panel 與 tab。

### 看仔細

- **Logs** —— 跟著最新的一行，往上捲就停，`G` 再跟上。在 workload 上，它跑的**每一個 Pod** 的 log 合成一份，每個 Pod 與
  container 各有自己的顏色，rollout 時看得出是哪一個出錯。
- **Events** —— workload 自己的 events 加上它的 Pod 的（CronJob 再加上它的 Job 的），新的在前。
- **Conditions** —— `.status.conditions` 的表，跟 `kubectl describe` 看到的一樣；events 過期之後還在。
- **YAML viewer**（`[Y]AML`）—— 有行號與語法顏色；`/` 搜尋，`n` / `N` 到下一個、上一個符合，`v` 選取，`y` 複製。
- **Compare** —— 比對同一種的兩個資源，unified 或左右並排（`L` 切換，kbu 會記住）。status 與 server 管的欄位會先拿掉，只看寫下的內容。
- **問題一眼看得到** —— 狀態欄只在需要注意時上色：pending 或降級中，或失敗。
- **Helm releases** —— `helm` 在 `PATH` 上時：每個 release 的 manifest、values、notes 與 hooks；revision 歷史與 rollback。
  chart 管理的物件有標記、不讓你誤改，想看清楚時按 `.` 藏起來。
- **KubeConfig contexts** —— 唯讀的 kubeconfig；在 context 上按 `Enter` 讓 kbu 切過去。憑證一律不顯示。

### 把事情做完

- **Edit**（`[E]dit`）—— 在 kbu 裡的 terminal 跑真正的 `kubectl edit`，用你的 editor。
- **Shell**（`[S]hell`）—— `kubectl exec` 進 container，一樣在 kbu 裡。
- **Delete**（`[D]elete`）—— 一定先問；刪 namespace 會多一句警告。
- **Alterm**（`Alt-t`）—— 住在 kbu 裡的 shell。用同一個鍵藏起來、叫回來：目錄、歷史與背景工作都還在。`Alt-Esc` 結束它。
  kbu 沒做的事，你慣用的工具在這裡做。
- **複製**（`y`）—— 那一列或整個畫面到剪貼簿（OSC 52，所以 ssh 與 tmux 裡也能用）。
- **Audit log** —— 從 kbu 做的每一次 edit 與 delete 都記下來。
- **你的 context 只屬於你** —— 在 kbu 裡切 context 從不動 `~/.kube/config`；另一個終端機的 `kubectl` 不受影響。
- **全螢幕**（`z`）—— 一個 panel 用滿整個畫面；再按 `z` 回到三個。

## 安裝

### 需求

- **kubectl** 在 `PATH` 上：edit、delete、shell 與你在 Alterm 裡跑的都要用。
- 一份 **kubeconfig**（`~/.kube/config` 或 `$KUBECONFIG`）與連得到的叢集。
- 終端機用 **Nerd Font**。有些字型把 icon 畫成兩格寬，有些連 powerline 符號（框上膠囊的兩端）也畫成兩格；kbu 啟動時會問
  終端機、照實際的寬度排版。要自己指定：`KBU__ICON_WIDTH` 與 `KBU__POWERLINE_WIDTH`（`1` 或 `2`）只給 kbu，或是
  `TERMINU__ICON_WIDTH` 與 `TERMINU__POWERLINE_WIDTH` —— terminu 家族共用：設一次，家族每個 app 都讀得到；kbu 在家族其他
  app 的 terminal 裡跑時，外層的 app 會替它設好。
- **truecolor 終端機**（24-bit 色）：柔和的顏色與疊起來的 popup 之間的明暗，在 256 色下分不出來。

### 快速安裝（macOS / Linux）

```bash
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/install.sh | sh
```

### 快速安裝（Windows PowerShell）

```powershell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/install.ps1 | iex
```

### Homebrew（macOS / Linux）

```bash
brew install vulcanshen/tap/kbu
```

### Scoop（Windows）

```powershell
scoop bucket add vulcanshen https://github.com/vulcanshen/scoop-bucket
scoop install kbu
```

從原始碼建置寫在 [`docs/dev-remarks.md`](docs/dev-remarks.md)。

### 第一次啟動

kbu 連上 kubeconfig 的 current context，先顯示 Pods。在你釘選、排序或離開之前什麼都不寫；那時才建立 `~/.config/kbu`
（見[設定與資料](#設定與資料)）。

### 移除

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.ps1 | iex
```

兩個都會先問，才刪你的設定。

## 快速開始

```bash
kbu
```

用 `j|k` 移動。panel 2 跟著 panel 1 游標所在的種類，panel 3 跟著 panel 2 游標那一列。`Tab` 到下一個 panel（或 `1`–`3`），
`Enter` 對游標下的東西做最直觀的事，`Space` 列出這裡能做的一切，`Esc` 退一步，`q` 離開。

`kbu version` 印出版號，`kbu help` 印出怎麼用。

## 使用方式

### 每個 panel 都一樣的鍵

| 鍵 | 做什麼 |
|---|---|
| `Space` | 這裡能做什麼：先是這一項，再是這個 panel，最後是 `Global operation`（namespace、context、Alterm、App log、離開） |
| `?` | 最前面那個東西的按鍵：panel，或最上層的 popup。再按 `?` 或 `Esc` 關掉 |
| `Tab`、`Shift-Tab`、`1–3` | 下一個 panel、上一個、照編號跳 |
| `Enter` | 最直觀的動作：drill 進去（workload 進它的 Pods）、打開 YAML、切到某個 context、shell 進 container、rollback 到某個 Helm revision。在 panel 1 上是到 panel 2；在 Logs、Events、Conditions、Info 上是全螢幕 |
| `Esc` | 退一步：清掉篩選、放掉比對的基準列、從 drill 退回一層。永遠不會離開 kbu |
| `q`、`Ctrl-C` | 離開 kbu（會記住你在哪裡） |
| `j\|k`、`u\|d`、`gg\|G` | 下、上，半頁，到頭、到尾。panel 3 上 `h\|l` 切 tab |

其他的鍵都是 `Space` menu 裡某一列的捷徑，每個 panel 的 `?` 都列得出來。最常用的：

| 在哪 | 鍵 | 做什麼 |
|---|---|---|
| Panel 1 | `P` | 釘選或取消釘選這個種類 |
| Panel 1 | `D` | 把釘選的種類拖到位置：`j\|k`，然後 `Enter`；`Esc` 放回原處 |
| Panel 2 | `Y`、`E`、`S`、`D` | YAML、edit、shell、delete |
| Panel 2 | `C` | 把這一列標成比對的基準；在另一列上按就比對兩者 |
| Panel 2 | `Alt-S` | 依一欄、再選方向排序清單 |
| Panel 2 | `.` | 顯示或藏起 Helm chart 管理的物件 |
| 任何 panel | `z` | 全螢幕；再按 `z` 回到三個 |
| Panel 3 | `B` | 在 Relatives 走深之後：breadcrumb |
| panel 上任何地方 | `N`、`C`、`Alt-t`、`!` | namespace、context（在 panel 2 上 `C` 是 compare）、Alterm、App log |

popup 疊在另一個 popup 上時，只有你正在用的那一個是亮的，底下的都變暗。

### Terminal（Alterm、edit、shell）

| 鍵 | 做什麼 |
|---|---|
| `Alt-Esc` | 結束程式，會先問 |
| `Alt-t` | 只有 Alterm：藏起來，shell 繼續跑 |
| `PgUp\|PgDn` | 往回捲、看捲出畫面的內容 |
| 其他鍵 | 送給程式 |

### Helm

在 panel 2 的 release 上按 `Space`，`[Y]AML` 旁邊列出它的文件：`Manifest`、`Creator Notes`、`User Values`、`Merged Values`、
`Hooks`。在 panel 3 的 History tab 上，`Enter` 把 release rollback 到游標那個 revision（目前部署的那個不行）；確認框會寫出確切的
`helm rollback`。

### Edit

`[E]dit` 先問，再在 terminal popup 裡跑 `kubectl edit <kind>/<name> -n <namespace> --context <context>`：跟在任何終端機裡一樣
—— strategic merge patch、衝突檢查、不會多出 `last-applied-configuration`。editor 由 kubectl 決定：`$KUBE_EDITOR`（`config.yaml`
有 `editor` 時 kbu 會設它）、`$EDITOR`、最後 `vi`（Windows 是 `notepad`）。editor 結束後，清單會經由 watch 自己更新。

nvim 在 popup 裡關得慢（plugin 在收尾）的話，`editor: "nvim --noplugin"` 只在 kbu 的 edit 略過 plugin。

## 設定與資料

kbu 存的東西都在 **`~/.config/kbu`**（有設 `$XDG_CONFIG_HOME` 時是 `$XDG_CONFIG_HOME/kbu`；Windows 是
`%USERPROFILE%\.config\kbu`）。`KBU__CONFIG` 可以改到別的目錄。

| 檔案 | 是什麼 |
|---|---|
| `config.yaml` | 你的設定（見下面）。釘選、排序、切換比對版面時 kbu 會寫它 |
| `state.yaml` | 上次停在哪裡；離開時寫入。`KBU__STATE` 可以放到別的目錄 |
| `theme.yaml` | 狀態欄的顏色（見下面） |

kbu 的 log 在 **`~/.kbu/logs`**（`KBU__DATA` 可以改到別的目錄）：`audit-<日期>.log` 記從 kbu 做的每一次 edit 與 delete，
`crash-<日期>.log` 是 kbu 當掉時的紀錄。

舊版的 kbu 把設定放在 `~/Library/Application Support/kbu`（macOS）或 `%APPDATA%\kbu`（Windows）。只有那個目錄在的時候，
kbu 繼續用它，App Log（`!`）會提一句；把它搬到 `~/.config/kbu`，就跟家族其他成員一樣了。

### config.yaml

```yaml
default_context: ""       # 啟動用的 context（預設：kubeconfig 的 current-context）
default_namespace: ""     # 啟動用的 namespace（預設：全部）
editor: ""                # 以 $KUBE_EDITOR 交給 kubectl
alterm_shell: ""          # Alterm 的 shell（預設：$SHELL，再來 /bin/sh）；只寫名字時從 PATH 找
alterm_login_shell: false # true 時 Alterm 的 shell 加 -l，讀 ~/.zprofile / ~/.bash_profile
compare:
  layout: unified         # "unified" 或 "split"；在比對的 popup 按 L 會寫回來
input_tab_width: 4        # viewer（YAML、log）裡一個 tab 佔幾格；terminu 家族共用

# 依種類的 kubectl 名稱分開記。暫時不在的種類（重裝中的 CRD）
# 釘選與排序照樣留著。
resource_kind_config:
  pod:
    pinned:
      order: 10           # 每個差 10，手改時可以插在兩個中間
    sort:                 # 先依重啟次數，多的在前；再依名稱
      - column: Restarts
        direction: desc
      - column: Name
        direction: asc
```

### theme.yaml

狀態欄的顏色 —— 畫面其他地方用的是 terminu 家族的配色：

```yaml
status:
  pending: "#f9e2af"   # pending、建立中、終止中……
  error: "#f38ba8"     # 失敗、crash loop、not ready……
```

### 環境變數

| 變數 | 做什麼 |
|---|---|
| `KBU__CONFIG` | 設定目錄，取代 `~/.config/kbu` |
| `KBU__STATE` | `state.yaml` 放的目錄，取代設定目錄 |
| `KBU__DATA` | log 放的目錄，取代 `~/.kbu` |
| `KBU__ALTERM_SHELL` | Alterm 的 shell，排在 `alterm_shell` 與 `$SHELL` 前面 |
| `KBU__ALTERM_LOGIN_SHELL` | `true` 或 `false`：Alterm 的 shell 是不是 login shell，排在 `alterm_login_shell` 前面 |
| `KBU__ICON_WIDTH`、`KBU__POWERLINE_WIDTH` | `1` 或 `2`：一個 icon、一個 powerline 符號佔幾格（見[需求](#需求)） |
| `TERMINU__ICON_WIDTH`、`TERMINU__POWERLINE_WIDTH` | 同上，給 terminu 家族每個 app；kbu 會替它的 terminal 裡跑的程式設好 |

## 限制

- **滑鼠目前沒有作用。** 一切都在鍵盤上。
- **Alterm、edit 與 shell 要 macOS 或 Linux**（WSL 也可以）：在 Windows 上 kbu 開不了內嵌的 terminal。
- **框線歪了一格的話**，是 kbu 啟動時問終端機得到的答案不對：`kbu iconwidth` 印出它量到的；設 `KBU__ICON_WIDTH` 與
  `KBU__POWERLINE_WIDTH`。Windows 不會量：字型把 icon 畫成兩格時，在那裡設這兩個變數。
- **Helm 要有 `helm` 指令**：`helm` 在 `PATH` 上才會出現 Helm 分類，release 清單每 3 秒重讀一次（Helm 沒有 watch）。
- **workload 的 log 跟的是你選那一列時已經有的 Pod。** rollout 之後，重新選那一列才會換成新的 Pod。
- **有些 edit 與 delete 不提供。** Events 與 node 不能從 kbu 刪；Helm chart 管理的物件請用 `helm upgrade`、`rollback`、`uninstall` 改。
- **panel 3 沒有 `/` 篩選。** 用 `Y` 打開 YAML，在那裡搜尋。
- **太長的名稱從尾端截**，panel 窄的時候，同一個 Deployment 的 Pod 可能看起來一樣：按 `z` 看完整的名稱。
- **改了視窗大小之後**，panel 2 等清單下一次變動時才照新的寬度排欄。

## 相關連結

- [CHANGELOG.md](CHANGELOG.md) —— 每一版改了什麼
- [`docs/dev-remarks.md`](docs/dev-remarks.md) —— 它怎麼運作、為什麼這樣做、建置與測試

## terminu 家族

kbu 遵循 [terminu design](https://github.com/vulcanshen/terminu)：跟家族其他成員一樣的鍵、一樣的 menu ——
[filu](https://github.com/vulcanshen/filu)（檔案）、[sshu](https://github.com/vulcanshen/sshu)（ssh）、
[webu](https://github.com/vulcanshen/webu)（網頁）、[locku](https://github.com/vulcanshen/locku)（鎖定螢幕）、
[kamonu](https://github.com/vulcanshen/kamonu)（家族的 mark）與 [ccdrawu](https://github.com/vulcanshen/ccdrawu)
（Claude Code 的 sidebar 與 status line）。

## License

[GPL-3.0](LICENSE)
