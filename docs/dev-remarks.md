# kbu 開發者備忘

開發 kbu 時要提醒自己、以及與 AI 協作時記下的決策。kbu 遵循
[terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.10.2/principle)（tdp v0.10.2），畫面建在
[terminu-go](https://github.com/vulcanshen/terminu-go)（v0.5.2）上；使用者要知道的在 README，這裡只放開發者需要的。

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
  | `list.go` | panel 2：欄位、排序、drill、compare（lib 的 `CompareAction`）、helm 的篩選、zoom |
  | `detail.go` | panel 3：tab、Logs 與 Events、Relatives 的鏈、History、Conditions、Info |
  | `textbody.go` | panel 3 沒有 cursor 的 tab 用的 body：折行、表頭不捲、追尾、`N-M of T` |
  | `popups.go` | YAML viewer（搜尋）、namespace 與 context picker、兩步排序、breadcrumb |
  | `ops.go` | 複製、edit、shell、delete、rollback、helm 文件、切換 context |
  | `pty.go` | Alterm |
  | `applog.go` | 寫進 lib 的 App Log（`app.Log`） |
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
| namespace picker | checkbox popup（`App.Checks`）；第一列是 lib 的 `Select all` |
| context picker | select（`App.Select`） |
| YAML、helm 文件 | viewer（`App.Viewer`）：行號、折行、選取模式 `v`、kbu 自己的鍵（`/`、`n`、`N`、`y`、`E`） |
| compare | lib 的比較模式與 DIFF（`App.CompareAction`）：kbu 給列的名稱（`rowLabel`）與 YAML（`rowYAML`），UNIFIED／SPLIT 記在 state |
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
- **離開不先問**（tdp K9：由 app 決定）：在 panel 上（含模式）跟舊版一樣直接離開，離開時存 state。有 popup 開著時第一下 `q`／`Ctrl-C`
  先開 `Quit kbu?`（K9-6，KB29，user 2026-10-22 定、app 不能關掉；lib v0.4.10 自動做）。
- **Compare 是模式**（user 2026-10-22 定，KB27 寫進 tdp v0.9.6 的 panel「比較模式」、note「比較」）：整套是 lib 的
  `App.CompareAction`（terminu-go v0.4.7）：模式、anchor 整列 Yellow、移動鍵、`C|Enter` 比對、只列差異的 DIFF、UNIFIED／SPLIT。kbu
  只給 `Name`（`diffLabel`，namespace/name）、`Text`（`MarshalItemYAMLForCompare`，用手上已有的 item，同步不打叢集）與 `Split`
  （`m.compareSplit`，存在 state.yaml 的 `compare_split`，note-84：選了哪一種是狀態）。舊的 `config.yaml` `compare.layout` 不再讀：
  帶過來的話分不出「舊的寫 split」和「之後選了 UNIFIED」，所以從 UNIFIED 重新開始一次。kbu 自己的 diff（`compare.go`）、`L`、
  anchor 狀態都拿掉了。
- **Esc 在 panel 2 的上一層**：退出 drill（tdp K4 一次一層）；Compare 模式的 `Esc` 由 lib 處理。panel 3：Relatives 的鏈退一層。
- **Logs 與 Events 是串流**（tdp T2），panel 3 失焦時不變暗；tab 名後面的 glyph 說它現在是不是跟著最新的一行（播放 / 暫停，
  同寬，tdp L2）。舊版 Events 失焦會變暗、寫成偏離；換到 lib 之後照 tdp T2 做，偏離拿掉。
- **Alterm 的隱藏只有 `Alt-t`**：舊版另有一個不揭露的 `Ctrl-T`（錄 demo 的 VHS 送不出 Alt），但它在 panel 上不會叫出 Alterm，
  是半套的別名（tdp K7），一起拿掉；舊版「`Ctrl-T` 不揭露」的偏離也就沒了。README 的 demo 不錄 Alterm。
- **zoom（`z`）是 `App.ForceNarrow`**：只畫 focus 的 panel、用滿畫面；`Tab` 照樣換 panel。它是版面的切換，不是模式（術語「模式」），
  再按一次 `z` 還原。這就是 tdp v0.8.0 的 zoom 預設（components/layout/screen：跟窄寬一樣，換到的 panel 也放大）。
- **窄寬的門檻是 60 欄**（`Screen.NarrowBelow`）。**暫定**：panel 1 要 24 欄、panel 2 至少要放得下 Namespace 與 Name。
- **最低尺寸是 40×15**（`App.MinWidth`、`MinHeight`，比家族預設的 80×40 低；tdp L1 允許調低）：終端機最常見的 80×24 與分割視窗
  都要能用，窄寬版面（一次一個 panel）在 40×15 還放得下清單、Space menu 捲得動。比這個小時照 tdp L1 只畫名稱與需要的大小。
- **panel 2 的欄**：namespaced 的種類第一欄是 Namespace，接著 Name，再一個沒有標題的 helm 標記欄 —— 選了好幾個 namespace 時，
  同名的物件要靠 Namespace 分；helm 標記欄沒有標題，是因為它只是一個標記、不是值。欄寬照 components/layout/panel（tdp v0.8.2，KB22）：
  長的值的欄（Name、Events 的 Message、Image、Hosts……，`longColumns`）標 `Column.Shrink`，分短欄剩下的寬度、太長就截；短欄照最寬的值。
  放不下時先把長欄縮到最少寬度（Name 是 `nameRoom` 16 格：從中間截還認得出 workload 與 pod 的 hash；其他照 lib 的 12 格），還放不下
  才整欄收掉（`Column.Drop`，`listColumns`）：先收 IP、Node 這類次要欄（從右邊起），再依序收 Restarts、Age、Ready、Status、Type、
  Reason、helm 標記欄、Namespace，再收 Name 以外的長欄（它們會縮，所以在短欄之後收，從右邊起）；Name 永遠不收。排序中的欄最後才收，排序的箭頭盡量留著。只選一個 namespace 時每一列的
  Namespace 都一樣，它第一個收。名稱從中間截（`Column.Middle`：`demo-app…66-gm72n`，前面是 workload、後面的 hash 分得出是哪一個
  pod；KB19）。實機（OrbStack，有一個 52 格的 pod 名稱）：120 欄收掉 Node、Name 24 格；100 欄再收 IP；80 欄再收 Restarts、Age，
  Name 16 格，Ready、Status 都在。
  drill 下去時 label 是種類、`Panel.Name` 是那一層的名字（`[2] PODS: nginx-7d4f`），整條路徑寫在位置列（`List.Location`，
  `deployment/web > pods`；S16）。數字欄（Restarts、Age……）靠右、照數值排；Age 照建立時間排（升冪 = 舊的在前，跟
  `kubectl --sort-by` 一樣），`N/M` 照兩個數字。沒有選排序時照 namespace、名稱。
- **Relatives 是一般清單加分組**：組是 section 的標題，兩欄 `NAME`、`TARGET`（多欄的清單一定有表頭）；能進去的那一列尾端一個箭頭。
  沒有 ref 的列照樣停得到，`Enter` 停用。**暫定**：欄名。
- **狀態欄的顏色是 kbu 的內容**（tdp P4）：只有需要注意的才上色，pending 與 error 兩色，寫死在 `internal/theme`。
  `theme.yaml` 整個拿掉（KB3，user 2026-10-19）：介面元素是家族的配色，改不了；之後要有 theme，是全家族一起重新設計的題目。
- **namespace picker 的全選**：tdp v0.9.11 起每個 checkbox 組由 lib 自帶 `Select all` 列，app 自己放「All」選項是用錯，所以 kbu
  的 All Namespaces 選項拿掉（KB31）。每一個都勾著寫回時讀成整個 cluster，之後新建的 namespace 也算（tdp 讓 app 自己解讀）；一個都沒勾
  也是整個 cluster：沒有東西可以看，全部最接近使用者的意思。開啟時是整個 cluster，就每一個都勾。拿掉的理由：自己的 All 勾著時再勾
  `default`，寫回的還是全部，`default` 被默默丟掉。
- **不用「被指到的 panel」**（tdp v0.9.16，`List.Points`，app 選用）：那是給一列代表畫面上一個 panel 的清單（sshu 的 sessions 與
  它的格子），每一列指不同的格子。kbu 的 panel 1、panel 2 是 details 的關係（components/layout/panel）：每一列都對著同一個下一格，
  用了只會讓 panel 2、panel 3 在 focus 在前一格時一直是 Lavender，不多說什麼，所以不設。
- **context**：從 Contexts 清單按 `Enter` 先問；從 `[C]ontext` 的 select 選定不再問（tdp F6：經由 picker 明確選定的那一次算已確認）。
  切換時清掉 CRD 再重新探索，正在看的種類是能 drill 的或新叢集沒有的，就回到 Pods。
- **錯誤與 App Log**（tdp F5、K1）：App Log 是 lib 的，panel 上按 `!` 打開，toast 與 error popup 自己會進去，footer 寫還沒看過的錯誤、
  警告數。kbu 用 `app.Log` 記其他的事：使用者做的動作與結果（切換種類、標 anchor、edit、exec、切 context）、背景的事（連上叢集、
  找到的 CRD、namespace 選擇的修正、log 串流的狀況）、外面讀不到的細節。toast 已經記的不再記一次；toast 是短字，kubectl 或 API
  給的多行訊息只放第一行，整段記進 App Log（`Model.toast`；error popup 是 `Model.errorPopup`，tdp v0.9.15 components-32）。kbu 自己的
  字不放 `\n`：好幾句的訊息一句一列給（`app.Error(title, rows...)`，例如 `Not moved`；components-30、31），所以畫面上紅色的 `\n`
  一定是外面來的。tdptest 的 Driver 預設會抓紅色的 `\n`，故意用 `Tricky` 當名字的測試設 `AllowLineBreaks`。每一筆跟 toast 一樣大寫開頭（note-78）：kbu 寫的字大寫開頭，名稱開頭的
  句子改成動詞在前（`Started kbu`），名稱照原樣；Go error 整句拿來用的（watch 的 `listing pods: …`）由 `capital` 把第一個字改大寫。
  舊版自己的 App Log、statusbar 的錯誤數、`Clear the App log` 都拿掉（lib 的只放在記憶體）。watch 失敗而 panel 2 是空的時，panel 中間照 components/layout/panel 寫兩句。
- **控制字元**（tdp v0.9.0 components「控制字元」）：清單的格子、statusbar、menu、toast 交給 lib，lib 照「名字與短字」畫；kbu 的顏色
  改成 spans（`Row.Styled` 的狀態色、Relatives 的箭頭、History 的 ●；statusbar 回傳 `[]Span`）。menu 的列沒有
  上色的接口，breadcrumb 目前那一層的 Lavender 拿掉（color-9 現在只管位置列）。kbu 自己畫的 panel 3 表格（Events、Conditions）與
  Info 的值是叢集給的字，用 `terminu.Escape`。pod 的 log 是拿來讀的內容，用 lib 的 `terminu.Reading` 畫（`logRows`，KB24）：
  `\n` 拆成兩列、`\r` 只留最後的狀態、顏色碼照著上色、Tab 是 `input_tab_width` 格空白、其他控制字元畫成紅色的名字；容器的前綴與
  折行是 kbu 的 body 自己加的（換成 Pager 會失去它們）。
- **statusbar 的順序**（tdp L2：一列裡最後一個動態欄位可以不補空白）：會出現、消失的 chip（`[Alt-t]erm`、`[C]ompare`）在最前面，
  接著 `[C]ontext`（固定 24 格，太長從中間截），`[N]amespace` 排最後、照名稱長度（最多 16 格）。舊版 context 與 namespace 都照名稱
  長度、後面的 chip 跟著移，是 user 2026-09-28 裁定的偏離（固定 24 格時短名字後面空一大格、80 欄把 chip 擠出去）；tdp L2（v0.8.0）
  補了這個做法（KB6，user 2026-10-19 選 C），偏離拿掉。
- **讀不了時**（tdp E3）：`config.yaml`、`state.yaml` 或 `kinds.yaml` 讀不了（格式錯、沒有權限），main 先用預設值，啟動時依序開
  `MOVE ASIDE`（lib 的 `App.MoveAside`，`askAside`）：`Enter` 改名成 `.bak`，像第一次打開一樣照常開始（E3-18）：設定用預設值、狀態與釘選
  排序從空的開始。kbu 第一次打開一個檔都不建，所以這時也不建，等第一次存檔才有：狀態在離開時、釘選排序在第一次釘選或排序（設定只有
  釘選排序還留在 config.yaml 時才寫）（資料只有 `kinds.yaml` 一個檔，沒有一筆一個檔的，E3-22 不適用）；`Esc` 離開，
  `Run` 回傳 `tui.NotRead`，main 把路徑與原因印到 stderr，不寫 crash log。kbu 沒有快取。
- **不再用的檔**（tdp E3-23）：`theme.yaml`（KB3 拿掉，畫面用家族的顏色）與 km8 的目錄（v2.0 複製到 kbu 的之後留在原地）還在時，
  每次啟動由 lib 的 `App.NoLongerUsed` 在 App Log 記一筆 INFO（路徑來自 `config.NoLongerUsed`），不刪、不跳 toast。`theme.yaml` 找
  設定目錄的：舊位置搬家時它跟著其他設定逐項搬過去，搬不動時 kbu 照舊用舊目錄，`ConfigDir` 兩種都對。km8 的目錄看平台的與
  `~/.config` 兩處；kbu 的設定目錄還沒有時不記，那時它還要拿來複製。`config.yaml.old.<版本>`（v1.7.5 改 key 前留的備份）是留給
  使用者的，跟 `.bak` 一樣不算。
- **設定、狀態、資料怎麼分**（tdp E3）：`config.yaml` 是設定，`state.yaml` 是狀態（上次停在哪裡），log 與釘選、排序是資料。釘選與
  排序是使用者一個種類一個種類寫給 kbu 的清單，就算內容是 kbu 怎麼排的選項，照 tdp v0.9.1 也是資料（user 2026-10-20 定）；排序
  是每個種類一串多層的欄位，不是一個清單只記一個排序欄的單一選項。兩者放在 `~/.kbu/kinds.yaml`（`config.KindData`，跟舊的
  `resource_kind_config` 同一個形狀，一個種類一筆）；`config.yaml` 只剩 kbu 自己一個一個的選項。
- **釘選與排序搬出 `config.yaml`**（tdp E3「換了位置時」）：`config.MoveKindData` 在啟動時、km8 的設定目錄複製之後跑：
  `config.yaml` 有 `resource_kind_config` 就寫進 `kinds.yaml`，再用 `yaml.Node` 拿掉那個 key、整份寫回（其他 key 與註解照留）；
  寫回失敗就刪掉剛寫的 `kinds.yaml`。`kinds.yaml` 已經在時不搬，error popup 寫兩個路徑讓使用者合併；搬不動時照舊讀寫
  `config.yaml` 裡的（`config.KindDataInConfig`、`Model.saveKinds`），error popup 寫兩個路徑。`Config.ResourceKindConfig` 因此
  留著：舊位置的那份，存設定時不會被丟掉。
- **設定位置照 tdp E3**：`~/.config/kbu`。舊版放在系統的設定目錄（macOS `~/Library/Application Support/kbu`、Windows
  `%APPDATA%\kbu`）。照 tdp E3（v0.8.1）的「換了位置時」逐項搬（`config.MoveOldPlaces`，啟動時、讀設定之前）：舊目錄裡的每一項
  用 `os.Rename` 搬到 `~/.config/kbu`，`logs/` 搬到資料目錄；新位置已經有的那一項留在舊位置（設定、資料都不刪）；搬完把空了的
  舊目錄拿掉。Info toast 說從哪搬到哪，留下來的開 error popup 寫兩個路徑。設定搬到一半失敗（例：跨磁碟、權限）時，已經搬的
  搬回去，照舊讀寫舊目錄（`ConfigDir` 的退路），error popup 寫兩個路徑。uninstall.sh、uninstall.ps1 照樣問舊位置。
- **log（crash、audit）放資料目錄 `~/.kbu/logs`**（`KBU__DATA`）：tdp E3 照「刪掉會怎樣」分，log 刪了就找不回來，是資料（KB15，
  user 2026-10-19）。舊版寫在設定目錄 `logs/` 的 log（Linux 也是），第一次啟動時一起搬到資料目錄。
- **確認一律用 `Action.Ask`**：edit、shell、delete、rollback、切換 context 的問句在排 panel 的動作時就照游標那一筆建好（換列時重排），
  從 Space menu 執行時 `Esc` 回到 menu（tdp F4），`Enter` 之後 menu 留或清照下一條。只有 YAML viewer 的 `E` 用 `App.Confirm`，
  viewer 留在底下。
- **menu 的 source 留或清照 tdp T1 的表**（照「確定之後的動作」判）：terminu-go v0.2.0 從 menu 執行、打開了東西的動作預設保留 menu
  —— YAML、compare、helm 的文件、App Log、edit（S4），看完、改完 `Esc` 回到 menu。開 shell、drill、delete、切換 context、Alterm、
  breadcrumb 的跳轉設 `Action.Clear`；沒打開東西的（rollback、複製、標 anchor）照 lib 關掉 menu。要非同步才打開的（Relatives 的 `Y`、
  helm 的文件）設 `Action.Stay`，等它打開時 menu 還在。兩步排序選完方向用 `App.CloseTop(1)` 只關自己，回到欄位那一步，欄位的
  說明用 `Action.Live` 寫剛排好的順位（KB9）。從 YAML viewer 按 `E` 編輯，viewer 留著，編輯結束時重讀那一筆（內建的種類向 API 拿，
  CRD 用 watch 到的；KB10）。
- **YAML viewer 的搜尋**：`/` 與 `n|N` 是 kbu 的，命中交給 viewer 畫（`Viewer.FindText`、`Hits`、`Current`：每一筆加底線，目前那一筆
  是 cursor 色塊；KB21）；打字列用 `TextInput.Search`，貼上的控制字元只畫不擋（KB11）。
- **namespace picker 一按就開**（`App.LoadChecks`）：標題轉 loading，namespace 清單到了再填進去（KB8）；抓不到時 error popup 蓋在
  picker 上。
- **啟動回到上次的 panel**：`App.Focus`（tdp K3：啟動是使用者按的，回到上次的地方是他預期的；KB5）。

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

- **滑鼠**：家族只用鍵盤（tdp X1 允許；KB4，user 2026-10-19），連帶 Settings popup 拿掉。`config.yaml` 的 `mouse_opt_config` 照樣
  讀進來、不作用。
- **PTY**（components/dialog/terminal「哪些 PTY 有這兩種模式」）：Alterm 與 `kubectl exec` 是互動 terminal（`Terminal.Interactive`）：
  `PgUp` 進歷史模式（`j|k`、`u|d`、`gg|G` 捲、`Esc` 回即時）、`Alt-v` 選取。`kubectl edit` 開的是編輯器，不標，按鍵全給它（`PgUp`
  回到 vim）。`Home`／`End` 屬於 shell（tdp K12），kbu 不另外綁（KB7，user 2026-10-19）。
- **PTY 只在 unix**：lib 的 terminal 在 Windows 開不了（舊版用的 creack/pty 在 Windows 也一樣）。
- **彙總 log 的 Pod 汰換**：串流在選列當下取快照，rollout 之後要重新選那一列。
- **Helm 沒有 watch**：release 清單每 3 秒 poll 一次。
- **helm 指令不帶 `--kube-context`**：用 helm 自己的 current-context，kbu 的 context 對 helm 不生效。
- **panel 1 的篩選只比對種類的名稱**：舊版連分類名稱也比對（打 `network` 會列出整個分類）；lib 的 panel filter 只看列的內容。
- **未做**：`:` command palette、port-forward、describe（用 Conditions 與 Events 代替）、scale、restart。

## 偏離 tdp

目前沒有。舊的兩條（statusbar 依名稱長度、split compare 不折行）在 tdp v0.8.0 都成了規範允許的做法；compare 後來整個交給 lib。
還沒符合的也沒有了：`docs/kbu-terminu-fix.md` 全部修完，拿掉（tdp E7）。

## 設計文件導讀

kbu 沒有另外的設計文件；每個功能的理由在本文件，每一版改了什麼在 [`CHANGELOG.md`](../CHANGELOG.md)，畫面與按鍵的規則照
[tdp](https://github.com/vulcanshen/terminu/tree/v0.10.2/principle)，畫面的做法照 terminu-go 的 README 與 `docs/`。

| 檔案 | 內容 |
|---|---|
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
make dev          # build 再跑 ./kbu（讀真正的設定與資料）
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
- `tdp_test.go` 跑 terminu-go 的 tdptest：`Check` 照第一次啟動在很多尺寸、三種 icon 寬度下走過每個 panel 的 Space menu、global
  operation、App Log、`?`，再縮到最低尺寸以下；另一個用 tdptest 的 `Tricky` 當 pod 名稱。kbu 自己的計時器（兩個 debounce）用
  `app.Tick`，測試推得動時間。顏色碼漏在格子裡會畫成紅色的 `\x1b`，tdptest 抓不到，由 `TestNoColourCodeShows` 看畫面。
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
