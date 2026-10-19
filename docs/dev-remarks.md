# kbu 開發者備忘

開發 kbu 時要提醒自己、以及與 AI 協作時記下的決策。kbu 遵循
[terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.7.4/principle)（tdp v0.7.4），畫面建在
[terminu-go](https://github.com/vulcanshen/terminu-go)（v0.1.13）上；使用者要知道的在 README，這裡只放開發者需要的。

---

## 運作方式

### 誰做什麼

- **terminu-go 管整個畫面與 core key**：panel 的框、膠囊、hint、footer、statusbar 那一列、`Space` menu、`?` 的 key reference、
  confirm、toast、viewer、finder（select、checkbox popup）、terminal popup、開關動畫、淡化、`Esc` 一次一層、離開流程、splash、
  icon 與 powerline 符號寬度的探測（tdp E5）。kbu 不再畫任何框。
- **`internal/tui` 登記 kbu 的東西**：三個 panel、它們的動作（鍵、名稱、說明、範圍、要不要先問）、popup 的內容，以及把叢集的資料
  餵進去。一個 `Model`（pointer），檔案依區域分：

  | 檔案 | 內容 |
  |---|---|
  | `tui.go` | `Model`、`New`、`Run`（lib 的 `Run` 回來後停 watcher、log、PTY 並存 `state.yaml`）、啟動、statusbar、global 動作、版面 |
  | `data.go` | 叢集資料的訊息與非同步指令、`onMsg` 分派、session state 的恢復與儲存 |
  | `kinds.go` | panel 1：分組清單、Pinned、Drag mode |
  | `list.go` | panel 2：欄位、排序、drill、compare anchor、helm 的篩選、zoom |
  | `detail.go` | panel 3：tab、Logs 與 Events、Relatives 的鏈、History、Conditions、Info |
  | `textbody.go` | panel 3 沒有 cursor 的 tab 用的 body：折行、表頭不捲、追尾、`N-M of T` |
  | `popups.go` | YAML viewer（搜尋）、compare、App Log、namespace 與 context picker、兩步排序、breadcrumb |
  | `compare.go` | diff 的兩種版面 |
  | `ops.go` | 複製、edit、shell、delete、rollback、helm 文件、切換 context |
  | `pty.go` | Alterm |
  | `applog.go` | App Log 的紀錄 |
  | `splash.go` | splash（`icon.svg` 是 `docs/icon.svg` 的副本，測試守住兩份一樣） |

- **`internal/k8s` 與 `internal/config` 沒換**：client-go 的包裝、各種資源的 fetcher / detailer / relatives、watcher、log 串流、
  helm；設定、狀態、log 檔。

### 三個 panel

- **panel 2 是 panel 1 的明細、panel 3 是 panel 2 的明細**（components/layout/panel 的「明細」）：cursor 一動，下一個 panel 立刻換成
  那一筆；為了不在 `j` 一路往下時每一列都開 watch、抓 detail、開 log，換種類與換列都等 cursor 停 300ms 才真的去抓
  （`kindDebounce`、`rowDebounce`），晚到的結果用序號或 UID 丟掉。
- **panel 1 的 `Enter`** 把 focus 送到 panel 2（tdp K3 唯一能用 `Enter` 換 panel 的地方，`Action.ToDetails`）。panel 2 的 `Enter`
  不換 panel：能 drill 的種類 drill、Contexts 切換 context（先問）、container 開 shell、其他打開 YAML。panel 3 的 `Enter`：Relatives
  drill、History rollback（先問），沒有項目的 tab 是 zoom。
- **watch**：`k8s.Watcher` 對目前的種類 list 加 watch，任何事件整份重新 list；panel 2 的資料每次到，游標那一列的 detail 也重抓
  （relatives、conditions、events 會跟著叢集變）。drill 下去的子清單是取一次的快照，watcher 照樣跑在 panel 1 的種類上。
- **Logs**：Pod 串自己的 container；workload 用 `PodsForWorkload` 找當下的 Pod 合成一條，失敗或沒有 Pod 時 10 秒內不再試；串流有
  編號，舊串流殘留的行丟掉；最多 1000 行。
- **Relatives 的鏈**：`Enter` 抓那一筆、把它的 relatives 疊在這一 tab（鏈上已經有的不再進去）；`Esc` 退一層；`[B]readcrumb` 列出
  鏈上每一筆，選了先問、再把 panel 1、2 切到那裡（`switchTo`）。

### popup 換成了什麼

| 舊的 kbu 自己畫的 | 現在 |
|---|---|
| Space menu、global operation popup | lib 的（從 panel 的 `Actions` 與 `App.Global` 產生） |
| key reference（`?`） | lib 的（從同一份登記產生） |
| sort 的欄位與方向兩個 picker | 兩個 menu（tdp F1：一步一個 popup） |
| namespace picker | checkbox popup（`App.Checks`）；第一列是 All Namespaces |
| context picker | select（`App.Select`） |
| YAML、compare、helm 文件、App Log | viewer（`App.Viewer`）：行號、折行、選取模式 `v`、kbu 自己的鍵（`/`、`n`、`N`、`y`、`E`、`L`、`D`） |
| breadcrumb | menu，每一列先問（`Action.Ask`） |
| confirm | lib 的（`App.Confirm`、`Action.Ask`） |
| toast | lib 的 |
| Alterm、kubectl edit / exec | terminal popup（`terminu.Terminal`，lib 自己的終端模擬器） |
| Settings（滑鼠） | 拿掉（KB4） |

### 叢集的動作

所有 kubectl 都帶 kbu 自己的 `--context`（`kubectlArgs`），kubeconfig 從不寫。delete、rollback 用 `Model.run`（測試換成假的，
測試絕對不會真的刪東西）。edit 在 terminal popup 裡跑 `kubectl edit`，`editor` 有設時以 `KUBE_EDITOR` 交給 kubectl；exec 跑
`/bin/sh -c "if command -v bash …; then exec bash; else exec sh; fi"`（探測的輸出關掉、shell 本身的不關：bash 把提示列寫在 stderr）。
edit 開始與 delete 成功寫進 `logs/audit-<日期>.log`。

## 設計決定

- **零學習曲線。** 每個動作都在 `Space` menu；熱鍵只是那一列的捷徑（tdp M3）。lib 讓 menu、`?`、hint 與按鍵出自同一份登記，
  不會對不上。
- **組合，不取代。** Alterm 讓平常要跳出 kbu 才用的工具都在裡面跑；kbu 管導覽與檢視。
- **edit 用內嵌的 terminal 跑真的 `kubectl edit`**，不用 `tea.ExecProcess` 加 `kubectl apply`：前者讓 kubectl 的訊息留在 kbu 裡，
  語意也就是 `kubectl edit`。
- **session 狀態與設定分檔**：`config.yaml` 是使用者的文件，`state.yaml` 每次離開都重寫。
- **panel 2 的動作作用在游標那一列**；名稱與說明寫對什麼做什麼（tdp M5）。helm 管理的物件 Edit、Delete 列出但停用（tdp M6）：
  「由 Helm 管」是那個物件現在的狀態，不是這個種類做不到。種類本身沒有的動作不列（Events、Contexts、Releases 的 Edit，加上
  Nodes 的 Delete）。
- **離開不先問**（tdp K9：由 app 決定）：跟舊版一樣直接離開，離開時存 state。
- **Esc 在 panel 2 的上一層**：先放掉 compare anchor，再退出 drill（tdp K4 一次一層）。panel 3：Relatives 的鏈退一層。
- **Logs 與 Events 是串流**（tdp T2），panel 3 失焦時不變暗；tab 名後面的 glyph 說它現在是不是跟著最新的一行（播放 / 暫停，
  同寬，tdp L2）。舊版 Events 失焦會變暗、寫成偏離；換到 lib 之後照 T2 做，偏離拿掉。
- **Alterm 的隱藏只有 `Alt-t`**：舊版另有一個不揭露的 `Ctrl-T`（錄 demo 的 VHS 送不出 Alt），但它在 panel 上不會叫出 Alterm，
  是半套的別名（tdp K7），一起拿掉；舊版「`Ctrl-T` 不揭露」的偏離也就沒了。README 的 demo 不錄 Alterm。
- **zoom（`z`）是 `App.ForceNarrow`**：只畫 focus 的 panel、用滿畫面；`Tab` 照樣換 panel。它是版面的切換，不是模式（術語「模式」），
  再按一次 `z` 還原。**暫定**：tdp 只說窄寬時這樣畫，zoom 要不要做成別的樣子由 app 決定。
- **窄寬的門檻是 60 欄**（`Screen.NarrowBelow`）。**暫定**：panel 1 要 24 欄、panel 2 至少要放得下 Namespace 與 Name。
- **panel 2 的欄**：namespaced 的種類第一欄是 Namespace，接著 Name，再一個沒有標題的 helm 標記欄 —— 選了好幾個 namespace 時，
  同名的物件要靠 Namespace 分；helm 標記欄沒有標題，是因為它只是一個標記、不是值。窄的時候先讓出空間的是 IP、Node、Image 這類
  （`keptColumns` 以外的），panel 窄到所有欄的自然寬放不下時，這些欄整欄拿掉（`fitColumns`）：八欄都被擠成三格的清單分不出是哪
  一筆（tdp L1）；排序中的欄不拿、也不讓，排序的箭頭要看得到。還不夠時 Name 先讓，讓到它的 MinWidth（20 格）為止，Namespace
  保持整欄（`nameGives`）—— 舊版的 Namespace 至少 15 格；Name 讓到底還不夠，才換 Namespace 先讓（lib 一次把它截成三格，KB17）。
  名稱從尾端截（tdp components/layout/panel），舊版從中間截、留住尾端的 pod hash，tdp 沒有這種截法（KB19）。數字欄（Restarts、Age……）靠右、照數值排；Age 照建立時間排（升冪 = 舊的在前，跟
  `kubectl --sort-by` 一樣），`N/M` 照兩個數字。沒有選排序時照 namespace、名稱。
- **Relatives 是一般清單加分組**：組是 section 的標題，兩欄 `NAME`、`TARGET`（多欄的清單一定有表頭）；能進去的那一列尾端一個箭頭。
  沒有 ref 的列照樣停得到，`Enter` 停用。**暫定**：欄名。
- **狀態欄的顏色是 kbu 的內容**（tdp P4）：只有需要注意的才上色，pending 與 error 兩色，寫死在 `internal/theme`。
  `theme.yaml` 整個拿掉（KB3，user 2026-10-19）：介面元素是家族的配色，改不了；之後要有 theme，是全家族一起重新設計的題目。
- **namespace picker 的 All Namespaces**：All 勾著就是全部，沒勾 All 就是勾起來的那幾個，一個都沒勾也是全部。理由：checkbox popup
  裡的 Green 讀成「按 `Enter` 就是這個值」（components/input/finder），All 跟幾個 namespace 都是 Green 時，結果只能是全部；
  一個都沒勾時沒有東西可以看，全部最接近使用者的意思。舊版「All 勾著時再勾一個就變成只有那一個」跟畫面對不上，改掉。
- **context**：從 Contexts 清單按 `Enter` 先問；從 `[C]ontext` 的 select 選定不再問（tdp F6：經由 picker 明確選定的那一次算已確認）。
  切換時清掉 CRD 再重新探索，正在看的種類是能 drill 的或新叢集沒有的，就回到 Pods。
- **錯誤**：toast（tdp F5）加 App Log；statusbar 右邊寫還沒看過的錯誤、警告數（Red、Peach 的字），打開 App Log 就算看過。
  watch 失敗而 panel 2 是空的時，panel 中間照 components/layout/panel 寫兩句。
- **設定位置照 tdp E3**：`~/.config/kbu`。舊版放在系統的設定目錄（macOS `~/Library/Application Support/kbu`、Windows
  `%APPDATA%\kbu`）。照 tdp E3 的「換了位置時」：新位置沒有、舊位置有，啟動時（`config.MovePlatformConfigDir`，在讀設定之前）
  用 `os.Rename` 把整個目錄搬過去，Info toast 說從哪搬到哪；搬不動（例：跨磁碟、權限）時照舊讀寫舊目錄（`ConfigDir` 的退路），
  開 error popup 寫出兩個路徑。舊目錄裡的 `logs/`（舊版的 log）跟著整個目錄搬，不另外搬到資料目錄。uninstall.sh、uninstall.ps1
  照樣問舊位置。
- **log（crash、audit）放資料目錄 `~/.kbu/logs`**（`KBU__DATA`）。**暫定**（KB15）：E3 沒有 log 這一類，它是 kbu 替使用者記下來的
  東西，最接近「資料」。舊的 log 留在原本的設定目錄，kbu 不會讀回 log，所以不搬。
- **確認一律用 `Action.Ask`**：edit、shell、delete、rollback、切換 context 的問句在排 panel 的動作時就照游標那一筆建好（換列時重排），
  從 Space menu 執行時 `Esc` 回到 menu（tdp F4）、`Enter` 清掉整疊（tdp T1）。只有 YAML viewer 的 `E` 用 `App.Confirm`（KB10）。
- **App Log 的清除在 Global operation**：note 只用來讀（tdp F1），清除是對整個 app 的動作。

### 環境變數

照 tdp E2：`KBU__CONFIG`（設定目錄）、`KBU__STATE`（狀態目錄）、`KBU__ALTERM_SHELL`、`KBU__ALTERM_LOGIN_SHELL`；icon 與 powerline
符號的寬度 `KBU__ICON_WIDTH`、`KBU__POWERLINE_WIDTH` 與家族共用的 `TERMINU__ICON_WIDTH`、`TERMINU__POWERLINE_WIDTH` 由 lib 讀
（`Probe("kbu")`），lib 也替 terminal 裡的程式設好兩個 `TERMINU__`。`KBU__ALTERM_LOGIN_SHELL` 用 `strconv.ParseBool`。測試用的
`KBU__TEST_K8S` 也照這個寫。會存檔的測試把 `KBU__CONFIG`、`KBU__STATE` 設到暫存目錄（`newTestModel`）。

## 已否決，不要重提

- 用 `tea.ExecProcess` 跑 editor 再 `kubectl apply -f`（見「設計決定」）。
- 把 session 狀態寫進 `config.yaml`。
- panel 3 的 `/` 篩選（有 cursor 的 tab 不容許過濾列；大量內容用 `Y` 的 viewer 搜尋）。
- kbu 自己畫 popup、自己做寬度函式與疊層：lib 已經做了，家族同一份（tdp E8）。

## 已知的牆與未做

- **滑鼠**：lib 沒有（KB4），連帶 Settings popup 拿掉。`config.yaml` 的 `mouse_opt_config` 照樣讀進來、不作用。
- **啟動恢復 focus 的 panel**：lib 沒有設定 focus 的 API（KB5），`start` 送一個數字鍵代替使用者按。
- **PTY 只在 unix**：lib 的 terminal 在 Windows 開不了（舊版用的 creack/pty 在 Windows 也一樣）。
- **彙總 log 的 Pod 汰換**：串流在選列當下取快照，rollout 之後要重新選那一列。
- **Helm 沒有 watch**：release 清單每 3 秒 poll 一次。
- **helm 指令不帶 `--kube-context`**：用 helm 自己的 current-context，kbu 的 context 對 helm 不生效。
- **panel 1 的篩選只比對種類的名稱**：舊版連分類名稱也比對（打 `network` 會列出整個分類）；lib 的 panel filter 只看列的內容。
- **未做**：`:` command palette、port-forward、describe（用 Conditions 與 Events 代替）、scale、restart。

## 偏離 tdp

- **statusbar 的 context 與 namespace 依名稱長度，不是固定寬度（tdp L2）。** context 最多 24 格、namespace 最多 16 格，超過從中間
  截斷（`capMiddle`，`internal/tui/tui.go` 的 `statusbar`）；換了 context 或 namespace，後面的 `[N]amespace`、`[Alt-t]erm`、
  `[C]ompare` 會跟著移。理由（user 2026-09-28 實機看過固定寬度的版本後裁定）：固定 24 格讓 `orbstack` 這種短名字後面空出一大格，
  80 欄左右還把後面的 chip 擠出畫面；切換是使用者自己按的、不常發生。改用 terminu-go 之後這一列仍是 kbu 自己畫的（`App.Status`），
  照舊保留，要不要留已回報（KB6）。
- **split 版的 compare 截斷、不折行（components/dialog/note 的 viewer 折行）。** 左右兩欄並排，各自截到欄寬（`splitDiff`，
  `internal/tui/compare.go`）。理由：兩欄並排的一列折下來，左右就對不齊，side-by-side 比對的意義也沒了；要看完整的一行，用 `L` 換成
  unified（折行）或 `Y` 看那一筆的 YAML。這是 components 的偏離，也是 tdp 的缺漏：viewer 沒有並排內容的方案。

## 設計文件導讀

kbu 沒有另外的設計文件；每個功能的理由在本文件，每一版改了什麼在 [`CHANGELOG.md`](../CHANGELOG.md)，畫面與按鍵的規則照
[tdp](https://github.com/vulcanshen/terminu/tree/v0.7.4/principle)，畫面的做法照 terminu-go 的 README 與 `docs/`。

| 檔案 | 內容 |
|---|---|
| [`kbu-terminu-fix.md`](kbu-terminu-fix.md) | 還沒符合 tdp、等 lib 或規範的地方（tdp E7） |
| [`icon.svg`](icon.svg) | kbu 的 mark（tdp S5：U 框、K、B 各一段註解），README 與 splash 用 |
| [`km8-icon.svg`](km8-icon.svg) | 改名前（km8）的圖示 |
| [`social-preview.png`](social-preview.png) | GitHub 的 social preview |
| [`demo-basics.gif`](demo-basics.gif) | README 唯一的 demo |

靈感來自 [Lens IDE](https://k8slens.dev/)、[lazygit](https://github.com/jesseduffield/lazygit)、[lazydocker](https://github.com/jesseduffield/lazydocker)
與 [k9s](https://github.com/derailed/k9s)。以 Go、[terminu-go](https://github.com/vulcanshen/terminu-go)（建在
[Bubble Tea](https://github.com/charmbracelet/bubbletea) v1 上）與 [client-go](https://github.com/kubernetes/client-go) 建構。

## 建置與開發

從原始碼安裝：

```bash
go install github.com/vulcanshen/kbu/cmd/kbu@latest
```

本地：

```bash
git clone https://github.com/vulcanshen/kbu.git
cd kbu
make build        # → ./kbu（CGO_ENABLED=0 靜態、-trimpath、strip；版本是 dev）
./kbu
./kbu iconwidth   # 啟動時量到 icon 佔幾格（tdp E5）
./kbu version
./kbu help
```

`make`（或 `make help`）列出所有 target，家族共用的在 `family.mk`（tdp project，不在 kbu 裡改），kbu 自己的接在最後的
`##@ kbu 自己的`（現在沒有）。**commit 前跑 `make check`**：`fmt-check`、`vet`、`test-race`，有 `go.work` 時加 `check-lib`
（關掉 `go.work`、照 `go.mod` 的 terminu-go 版本編譯與測試）。

開發時用不進版控的 `go.work` 接本機的 terminu-go：`go work init . ../terminu-go`。`go.mod` 寫的是 terminu-go 發布的 tag，不用 `replace`；
`make tidy` 關掉 `go.work` 跑 `go mod tidy`。

測試：

- `internal/tui` 的測試不連叢集：`newTestModel` 建一個沒有 client 的 kbu，`feed` 餵 panel 2 的資料，`press` 照 tdp 的鍵名送鍵，`drive`
  把 lib 回傳的指令（動畫、debounce）跑完再餵回去。會存檔的測試設好暫存的 `KBU__CONFIG`、`KBU__STATE`；`Model.run` 換成會讓測試
  失敗的假的，測試絕對不會真的跑 kubectl delete。
- 畫面測試（tdp L4）量 icon 與 powerline 符號佔一格、兩格的組合，在幾種尺寸、開幾種 popup 之後，每一列都剛好是終端機寬。
- `internal/k8s` 有些測試要連真的叢集（開發時用 OrbStack），沒有叢集時跳過。

在 tmux 裡實際跑：`.local/scripts/shot.sh -s 120x40 -- Tab Space`（不進版控；照 terminu-go 的 `scripts/tmux-shot.sh`，設定與狀態放在
暫存目錄，連的是目前的 kubeconfig context）。

**demo gif**：README 只放一張 `docs/demo-basics.gif`，用 VHS 錄（`make gif`，tape 是 `.local/demos/demo-basics.tape`，不進版控）。
tape 的 setup 會把 orbstack context 的 namespace 設成 default，錄完要 `kubectl config unset contexts.orbstack.namespace`。

## 發布

照 tdp project 的「發布」：`make check` → CHANGELOG 的 `## Unreleased` 改成 `## vX.Y.Z — tdp vA.B.C` → commit、打 tag、push。
`.github/workflows/release.yml`（家族範本，測試的 matrix 加了 `windows-latest`）在 ubuntu、macOS 跑 `go test -race`、Windows 跑
`go test`，過了由 goreleaser 建 linux / darwin / windows × amd64 / arm64，推 Homebrew tap 與 Scoop bucket，最後用 CHANGELOG 那一段
蓋過 release 的內文。版號由 user 決定。

踩過的坑：

- goreleaser 的 `--release-notes` 不可靠，workflow 最後一步用 CHANGELOG 強制覆寫 release body；`if: always()`，因為 tap push 401 這類
  後段失敗時 release 通常已經建好、只差 body。
- 版號沒變的覆蓋發布（刪 release、刪遠端 tag、重打）之後，`brew upgrade` 不會重裝，要 `brew reinstall kbu`。
- v3.0.0 與更早的 CHANGELOG 標題是 `## [vX.Y.Z] - 日期` 的舊寫法，照 tdp project 不改；之後的一版用 `## vX.Y.Z — tdp vA.B.C`，
  release.yml 用 `$2 == tag` 抓那一段（舊寫法的標題它抓不到，不影響新版）。
