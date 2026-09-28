# kbu — terminu fix

kbu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.13/principle)（tdp v0.1.13）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-28（對照 tdp v0.1.13；前一版清單 2026-09-27 對照 v0.1.4，初次盤點 2026-09-26 對照 v0.1.0）。以
`terminu-fix` 分支的 `65d9804` 為準（程式碼與 `main` 相同），拿 v0.1.13 的 rules 全文逐條（不只 CHANGELOG）對程式碼核對過；
位置寫檔案與函式，不寫行號。statusbar 折行與 popup 寬度是在 scratch 複本裡 render 量的（沒有動 kbu 的工作樹）。

v0.1.4 之後 tdp 改了很多：`?` 只讀（v0.1.2 起）、global 那一列不加標題（M2，v0.1.7）、popup 分六類與多步驟一步一 popup
（F1，v0.1.8–v0.1.9、v0.1.13）、popup 一種寬度與定高（F7）、最上層以外全部 dim（F8、D2）、模式裡 `Space` 不作用（K11，v0.1.10）、
內容區 panel 的 `Enter`（K3，v0.1.11）、loading icon（F7、D3）、truecolor（D6）。kbu 的程式碼從上一份清單之後沒有動，舊的 15 條
重新核對後大多仍成立（改寫成對照新條文），另外新增 F1、F3、F5、F7、F8、K3、K11、S3、T1、D6 的條目。舊清單「先看」說 S3 已符合是錯的
（第 21 條）；舊的待確認裡 statusbar 會不會折行已實測成立（第 20 條）。


## 先看

四個已經對齊的 app（locku、webu、sshu、filu，都在 `main`、對齊到 v0.1.12 / v0.1.13）修的時候學到的，適用 kbu 的：

- **清單先定案、先 commit，再動程式。** 這份清單由 terminu session 寫好放在 kbu 的工作樹，待確認已在 2026-09-28 與 user 定案
  （標「已定案」的條目）。先讀完再 commit；程式的 commit 跟清單分開（locku、filu）。工作樹裡另有 2026-09-26/27 還沒 commit 的
  文件改動（兩份 README、`dev-remarks.md`、刪掉的六張 demo gif 與 `kbu-implementation.md`），一起讀過、先 commit 文件。
  commit 只加自己改的路徑，不要 `git add -A <目錄>`（filu 把沒讀過的清單一起 commit 過）。
- **「現況」拿程式碼核對過才算數，照內容比對、不照行號。** 前幾條一改，後面的位置就漂；清單之間也會互相吃掉條目
  （第 3 條刪掉 `comparemenu.go`，第 25 條表裡它的三處註解就跟著消失）。換註解前先 `grep` 現況。
- **修完清單，拿 tdp 全文再逐條對一次。** 四個 app 每一輪都在這一步多找到清單沒列的（locku 的 F4 / F3、webu 的 M3 / K5、
  filu 的 M9 / F5 / K3 / F6）。
- **建議的順序**：第 1、2 條（疊層只有一份順序、`owns()`）是其他條的地基，先做；接著做會新增或拆掉 popup 的條目（第 3–14 條：
  global operation popup、各 key reference、sort 拆成兩步、拿掉 compare menu 與拖曳的 drop menu）；F7 尺寸（第 18 條）再來；
  F8 dim（第 19 條）最後做，這時 popup 都到齊了（locku：先做新增 popup 的那條，再做 dim，否則要改兩次）。
- **「最上面」只能有一個答案**：按鍵路由、`Esc` 關哪一層、繪製順序、誰是亮的（F8）、滑鼠點到誰 —— 全讀同一份由下往上的清單
  （filu `stackOrder()`、sshu / webu `floats()`）。kbu 現在這幾處各是一份順序（第 1 條）。
- **判斷一層「還在不在」用 `owns()`（開啟中或已開），不用含關閉中的 `IsActive()`**；能不能接鍵用 `IsInteractive()`。
  路由、層數、F8 的亮暗都用同一個判斷（locku、sshu、webu）。
- **共用 helper**：popup 寬度一個函式（filu `popupInnerWidth()`、locku `popupInnerW()`、webu `popupW()`）；高度開框時定
  （filu `openRows`）；dim 一個函式、在合成最上層之前過一次整張畫面，不要每個 popup 各畫一個暗的版本。
- **dim 搬 filu 的 `internal/ui/dim.go`**：改寫已畫好的字串裡每個 SGR 色碼，前景背景都往 base 淡化，**絕不變亮**（每個通道取
  原值與淡化值較小的那個，v0.1.12；filu 的 `dimRGB()` 現在已有這一步）。不要去色重畫 —— locku、sshu、webu 第一版都這樣做，
  膠囊、cursor bar、選取反白一 dim 就不見，v0.1.11 才寫進 F8 禁止。搬參考實作前先逐條對一次最新的 D2（webu 的教訓）。
- **把 `View()` 印出來看。** 在測試裡暫時 `t.Log(ansi.Strip(m.View()))`，看完就刪：疊層順序、框有沒有貼邊、dim 之後 panel 的
  powerline 膠囊背景還在不在（變暗、沒消失）、menu 在 80 × 40 有多高（filu 的 `[1]` menu 曾高到 49 列，規則讀再多遍也看不出）。
- **測顏色要開顏色**：測試不是 TTY，lipgloss 不出色；`lipgloss.SetColorProfile(termenv.TrueColor)`、`t.Cleanup` 還原。量背景
  `48;2;…`，不只量前景；lipgloss 轉 24-bit 會差 1（`#89b4fa` → `137;179;250`），比對容許 ±2。
- **每修一處補 model test，逐處 mutation**：把修正單獨改回舊行為，確認對應的測試會紅。常見的假象：mutation 編譯不過不算抓到；
  被上限夾住（寬度 mutation 被 120 蓋掉、超長內容被截成同寬）；fixture 太單純（只有一列、只有一行）；用被測的函式或常數算預期值
  （filu 同源自比三次）—— 預期值一律寫死成條文算出來的數字。量「開著時不變高」要同時釘開框時的高度。mutation 在背景跑時別
  commit，中斷後回頭確認檔案還原。測「關閉中」要用不跑完動畫的送鍵（locku 的 `sends`）。
- **守舊規則的測試要改寫，不是刪掉**：改名並反轉斷言，名稱寫出新規則。測試 helper 別靠會被規則改掉的字串定位（sshu 的
  `beforeGlobal()` 靠 `global operation` 標題，v0.1.7 一拿掉就找不到；改靠 key）。
- **新 popup 的接線清單**：欄位、`NewAppModel()`、`WindowSizeMsg` 的 `SetSize`、`AnimTickMsg` 的 `HandleTick` 串列、按鍵路由、
  滑鼠路由、疊層清單、`View()`、`popupDepth()`、`closeAllBlockingPopups()`、`isMenuPopupActive()`、測試裡共用的 popup 表。
  拿掉一個 popup 時同一份清單逐處拿掉，`grep` 名字到零筆（sshu 拿掉 `modeKeys` 共十二處）。
- **Go 的 value receiver**：`Update()` 回傳新的 model，判斷「這一列有沒有開出新框」要看回傳的 model，不要在舊的上面判斷或關閉
  （locku、webu）。`tea.Quit` 可能被包在 `tea.Batch` 裡，測試的「有沒有離開」要遞迴看進 `BatchMsg`（locku）。
- **global operation 一份清單；key reference 從 Space menu 的列產生**（只收按得出來的鍵），兩邊不會不一致（sshu、webu）。
  label 已寫出鍵的列（`[/] Search`）別讓 `bracketHotkey()` 再括一次（webu 踩過 `[/] [/] Search`）。
- **K3 會一路擴散**：問 user 時一次把所有 panel / tab 列出來（filu，第 16 條）。
- **比畫面位置用顯示寬度**（`ansi.StringWidth`），不用位元組：loading icon 是 UTF-8 四個位元組的 PUA 字元（webu）。
- **loading icon 讀時鐘、不數 tick**（`frames[(now / 90ms) % 8]`；留一個 `loadingNow` 讓測試釘時鐘）。
- **patch 腳本與 commit message 先寫進檔案再執行**（`git commit -F`）：`zsh -c '…'` 裡英文的撇號（`panel's`）會截斷外層引號。
  commit 前 `gofmt -l`。
- **不發版。** 家族全部 app 與 tdp 都穩定之前不發 release；修完的條目記進 CHANGELOG `[Unreleased]`，README 與 dev-remarks 描述
  行為的段落跟修正放同一個 commit。改釘 tdp 連結時，帶日期的歷史敘述不改。
- 完整紀錄在 terminu repo 的 `.local/family-fix/{locku,webu,sshu,filu}/README.md`（本機）。


## popup 盤點（現況）

17 個 popup（各有一個 `PopupAnimator`；splash 不是 popup）。W、H 是終端機寬高；F7 的目標是除了 terminal 類以外一律
`min(W − 2, 120)` 寬、置中，高度依內容、開框時定（第 18 條）。

| popup | 檔案 | F1 類別（現在） | 寬度（現在） | 高度（現在） |
|---|---|---|---|---|
| panel 2 Space menu（`panel2Menu`，含 container 版） | `panel2menu.go` | menu | 依內容，上限 85% W | 開框時依列數定 |
| panel 1 Space menu、各 tab 的唯讀小抄、拖曳的 Drop menu（`hintPopup`） | `hintpopup.go` | 有動作時 menu + note 混在同一個框；唯讀時是 note | 75% W，夾在 50–80 | 開框時定 |
| Helm 文件 menu（`helmDocMenu`） | `helmdocmenu.go` | menu（Helm Release 列上 `Space` 開的，不是 Space menu） | 依內容，上限 85% W | 開框時定 |
| sort picker（`listPicker`） | `listpicker.go` | menu；欄位、方向兩步在同一個框換內容 | 依內容，上限 85% W | 換步驟時跟著變 |
| breadcrumb（`breadcrumbPopup`） | `breadcrumb.go` | menu | 依內容，上限 85% W | 開框時定（長 label 折行） |
| Settings（`settingsPopup`） | `settingspopup.go` | menu | 依內容，上限 85% W | 開框時定 |
| namespace picker（`namespacePicker`） | `namespace.go` | menu（多選）；`/` 打字時是附候選清單的 input | 固定 44 | loading 時一列，資料到了長到最多 10 列；`/` 多三列搜尋框；篩選後變少 |
| context picker（`contextPicker`） | `context.go` | menu；`/` 同上 | 固定 54 | 最多 10 列；`/` 多三列 |
| confirm（`confirm`） | `confirm.go` | confirm（問句 + 一行明細） | 40 起依內容，上限 70% W | 開框時定 |
| key reference（`help`） | `help.go` | note（全 app 一份） | W − 2（最少 60），兩欄 | 內容區固定 H − 8 |
| App log（`appLog`） | `applog.go` | note（熱鍵 `y`、`D`） | 70% W（最少 40） | 固定 60% H |
| YAML（`yamlPopup`） | `yamlpopup.go` | note（熱鍵 `/` `n` `N` `y` `E`；`v` 是模式；`/` 打字是輸入態） | W − 2 | 固定 H − 2，跟內容無關 |
| Compare（`comparePopup`） | `comparepopup.go` | note | W − 2 | 固定 H − 2 |
| Compare 的版面 menu（`comparemenu`） | `comparemenu.go` | menu，疊在 note 上、畫在 Compare 自己的框裡 | 依 hint | 開框時定 |
| toast（`toast`，含拖曳模式的 sticky 版） | `toast.go` | toast | 依訊息，最少 28、沒有上限 | 置中（不在下方） |
| Alterm（`shellPty`） | `ptyview.go` | terminal | 外框 W − 4 | 外框 H − 3 |
| `kubectl edit` / `exec`（`txPty`） | `ptyview.go` | terminal | 外框 W − 4 | 外框 H − 3 |

scratch 複本實測：W = 200 時 key reference 198 欄寬、YAML 198 × 38（空內容也是 38 列）；namespace picker 不論 W 都是 44。

修完之後會多出：global operation popup（menu，第 6 條）、每個 surface 的 key reference（note，第 11、12 條）、sort 的方向那一步
（menu，第 9 條）；少掉：`comparemenu`（第 3 條）、`hintPopup` 的混合用法與拖曳的 Drop menu（第 5、14 條）。


## 19. 最上層以外沒有 dim —— F8、D2

**現況**：`app.go` `View()` 把 popup 一層層 `overlay.Composite` 疊上去，底下的 panel、串流中的 log、下層 popup 都照原色畫；整個
專案沒有任何 dim 的程式。兩個 PTY 同時畫出來時（Alterm 在 `kubectl edit` 底下）底下那個也照原色。

**規則**：F8 —— 有 popup 開著時，最上層那個 popup 以外的一切（底下的 popup 與整個 base 畫面，串流內容與警示色也一起）都用 dim 色畫；
dim 的做法是把每一個顏色（前景與背景）往底色淡化，形狀與版面原封不動，不可以剝色重畫、丟背景、把前景統一成一個 dim 色；底下 popup 的
邊框是自己層色的 dim 版本；toast 不觸發 dim。D2 —— `dim(c) = c × 0.45 + base × 0.55`（base `#1e1e2e`），前景背景都算，16 / 256 色先換
RGB；每個通道取原值與淡化值較小的那個，絕不變亮；輸出一律 24-bit；沒有指定前景的文字給 `dim(Text #cdd6f4)`；bold、reverse、文字不動。

**怎麼改**：搬 filu 的 `internal/ui/dim.go`（`dimANSI()`），搬之前逐條對一次 D2。`View()` 照第 1 條的清單，合成最上層（最後一個 `owns()`）
之前把已經畫好的畫面過一次 `dimANSI()`；toast 畫在 dim 之後（sticky toast 若還留著，也不算一層）；不動任何 popup 的 render。測試開
truecolor，量背景：panel 的 powerline 膠囊、cursor 列、panel 3 active tab 膠囊、compare anchor 的 lavender 列，在 popup 底下是「自己的
顏色淡化」，不是消失；預期值手算寫死；「上層開始關時下層亮回來」用不跑動畫的送鍵量。印一次開著 popup 的 `View()`。dev-remarks
「依層數決定的 popup 邊框」補一句 dim。


## 已經符合、不用修的（對照 v0.1.13）

- **K1（letter hotkey 不佔 core key）**：`q` 只用在離開，`Space`、`?`、`Tab`、`Enter`、`Esc` 沒有被字母熱鍵借用；confirm 的 `y` / `n`、
  picker 的 `n` / `c` 不是 core key（`n` / `c` 兼關閉見第 27 條）。
- **K2**：畫面上 `Tab` / `Shift-Tab` 在三個 panel 之間輪替（`cyclePanel()`）；popup 開著時 `Tab` 不會漏到底下切 panel（路由先回傳）。
  沒有 input group、沒有灰字提議，單一輸入框的 `Tab` 規則（v0.1.6）不適用；沒有多行文字輸入（`kubectl edit` 的 editor 是 PTY，歸
  K10），v0.1.5 的縮排不適用。
- **K3 的 input 部分**：沒有 input popup；輸入只有單一欄位的搜尋列（panel 1、2 與 YAML 的 `/`），`Enter` 送出該欄（保留篩選、離開打字），
  送出不會失敗。picker 的 `/` 見第 10 條。
- **K4 的「永不離開 app」**：`Esc` 從不離開，到最上層什麼都不做；compare + drill 的「一次兩層」見第 26 條。
- **K6、K8 的輸入態**：panel 搜尋中（`app.go` 的 `searching` 分支）與 picker、YAML 的搜尋打字中，`?`、`q`、字母都進搜尋字串（空白見
  第 15 條）。
- **K10 保留的鍵**：`PgUp` / `PgDn` / `Home` / `End` 只在非 alt-screen 時攔下，並揭露在下框 hint（dev-remarks「PTY 裡的鍵」）；Alterm 的
  出口 `Alt-t` 常駐在下框 hint。
- **M1**：footer（`statusline.go`）固定一列、永遠列出 `? help` 與 `Space menu`，拖曳中也在。
- **M9**：`C` 在 panel 2 是 Compare、其他 panel 是 context；statusbar 的 `[C]ontext` 與 `[C]ompare` chip 依 focus 的 panel 一亮一暗
  （`ViewFull()`）。
- **L5**：focus 是雙線 `╔═╗` + Blue、非 focus 是圓角 `╭─╮` + Surface2，兩套框線同寬（`renderPanelWithScroll()`），切換不位移。
- **F1 的 confirm**：confirm 帶一行明細（`kubectl edit …`、`helm rollback …` 指令），v0.1.13 起就是 confirm。YAML 的 `/` 搜尋、`y`、`E`
  是 note 自己的熱鍵（F1 的例子正是 YAML 檢視），打字中 `Esc` 先取消搜尋、再按才關，一次一層。
- **F2**：每個 popup 都有 `PopupAnimator` 的開啟與關閉動畫（含 toast 與 PTY）。
- **F4 的大部分**：panel 2 Space menu 執行一列不關自己（`panel2Menu.commit()`），YAML、confirm、Compare、sort picker 疊在它上面；
  helm 文件 menu → YAML、breadcrumb → confirm 都留住 source。例外見第 9 條（panel 1 的 menu）。
- **F5 的大部分**：`appLog.Error` / `Warn` 立刻出現在 statusbar 的 badge 與 footer 右側的訊息，不擋住 app；namespace 抓取失敗、Relatives
  drill 失敗、存檔失敗另有 toast。例外見第 22 條。
- **F6**：問句寫出動作與對象（`Edit resource?` + 指令、`Rollback X to revision N?`、刪 namespace 的加強警告）；`Enter` / `y` 接受、
  `Esc` / `n` 取消；滑鼠左鍵刻意不接受（`HandleMouse()`）。breadcrumb 選定後仍 confirm，是 app 的選擇（F6 允許 picker 算確認，但不要求）。
- **F7 的錯誤列**：沒有送出會失敗的 input popup，不用預留。
- **T1 的 context-shift**：`kubectl edit` / `exec`、Alterm、drill-down 的 entry handler 先呼叫 `closeAllBlockingPopups()` 清掉整疊。
- **T2 的 Logs**：panel 3 失焦時 Logs 不變暗、保留 pod / container 的顏色（`detail.go` `buildLogLines()`）。Events 見第 28 條。
- **X1、X2**：沒有滑鼠也能做完所有事；每個滑鼠動作都對應到鍵（點 panel = focus + 移動 cursor、雙擊 = `Enter`、右鍵點列 = `Space`、
  popup 上右鍵 = `Esc`、滾輪 = `u` / `d`）。疊層上的命中順序見第 1 條。
- **S1、S2、S4、S5**：`V` 只在主 switch（panel 上）打開 splash，popup、輸入態、PTY、拖曳都先攔下；啟動時不播；help、各小抄、footer、
  README 都沒有提到 splash；圖案由 `docs/icon.svg` 產生（`splash.go` 的 logo）。S3 見第 21 條。
- **D2 的層色**：`theme.PopupLayerColor()` 的 Lavenphire25 / 50 / 75 / Sapphire 就是 D2 表上的四個色碼；Lavender 留給使用者足跡，popup
  邊框不用它。


## 待確認

沒有。2026-09-28 在 terminu session 與 user 逐題定案：compare 鎖的 `Esc`（第 26 條）、開啟熱鍵兼關閉（第 27 條）、menu 的 `Enter` /
`Esc` 列（併入第 6 條）、各處的 `Enter`（第 16 條）、Events 失焦變暗（第 28 條，寫成偏離）。舊清單的 statusbar 折行（L3）已實測
成立、整畫面寬度測試（L4）併入第 20 條。
