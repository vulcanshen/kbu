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


## 3. Compare popup 上按 `Space` 疊出一個 menu —— K5

**現況**：`comparepopup.go` `handlePopupKey()` 的 `case " "` 開 `comparemenu`（`menuItems()`：`Switch to <layout> view`、
`Close` 兩列），`comparemenu` 畫在 Compare 自己的框裡（`renderFullPopup()` 最後的 `m.menu.Render(frame)`）；下框 hint
`Space: menu  j/k: scroll  Esc: close`。

**規則**：K5 —— popup 自己的操作用熱鍵執行，揭露在 popup 下框的 hint 與該 popup 的 `?` key reference，不在 popup 上再疊一個
Space menu。

**怎麼改**（owner 已定案）：切換版面改成一個熱鍵，寫進下框 hint 與 Compare 的 key reference（第 12 條）；鍵由 kbu 決定
（D5 的慣例把小寫 `h` / `l` 留給移動，例如用 `L`）。`Close` 那一列拿掉（`Esc` 就是關閉）。`comparemenu.go` 與 Compare 裡轉送
tick、按鍵的 sub-popup 路由一起刪。README 兩份的 Compare 說明、dev-remarks「功能筆記」YAML Compare popup 條目（「diff popup 有自己
的動作 menu（`Space`）」）、「依層數決定的 popup 邊框」裡「Compare 裡的 Diff menu」的例子一起改。


## 4. `Space` 會關掉不是 Space menu 的 popup —— K5、F6

**現況**：下列 popup 的 `Update()` 都把 `" "` 當成關閉 / 取消：

- `confirm.go`：`case "esc", "n", " "`（取消 confirm），下框 hint `Enter/y: confirm  Space: cancel`
- `yamlpopup.go`：`case " "`（註解寫明是舊的肌肉記憶；visual 模式裡也一樣，見第 14 條）
- `breadcrumb.go`：`case " "`，hint `Space: close`
- `namespace.go`：loading 中與一般狀態兩處 `"esc", "n", "N", " "`
- `context.go`：`"esc", "c", "C", " "`，hint `Space: cancel`
- `listpicker.go`：`"esc", " "`（sort picker）
- `settingspopup.go`：`"esc", " ", ">"`
- `applog.go`：`"esc", "!", " "`，hint `Space:close`
- `help.go`：`"esc", "?", " "`，hint `Esc/?/Space:close`；key reference 的 `Space` 說明寫 `Open menu / close popup`（`helpContent()`）
- `helmdocmenu.go`：`"esc", " "`。它現在是 Helm Release 列上 `Space` 開的 menu；第 5 條把它併進 Space menu 後就是 K5 本身，若保留成
  從 Space menu 開出的另一個 popup，`Space` 就不能關它
- `comparemenu.go`（第 3 條拿掉後隨之消失）、`hintpopup.go` 的唯讀模式（第 5 條拿掉後隨之消失）

**規則**：K5 —— `Space` 只關它自己開的 Space menu；由 `Enter` 或熱鍵打開的 confirm、picker、note 上按 `Space` 不作用，由 `Esc` 或
自己的流程關閉。F6 —— confirm 由 `Enter` 接受、`Esc` 取消；`Space` 能關 confirm 就兼了 `Esc` 的「取消」（P4）。

**怎麼改**：以上各處拿掉 `" "`，只留真正的 Space menu（`panel2Menu` 與第 5、6 條組出來的各 panel Space menu；filu 用一個
`spaceToggle` 旗標標出哪些是）。global operation popup 不是 Space menu，`Space` 在上面也不作用。confirm 下框 hint 改成說出後果
的寫法（D3：`Enter <動詞> · Esc cancel`，動詞依 `ConfirmAction`，例如 `Enter delete`）；其他 popup 的 hint 拿掉 `Space`。
dev-remarks「功能筆記」多 namespace 選取的「`Esc`/`Space` 關閉」一起改。測試 `TestBreadcrumbPopup_SpaceCloses`、
`TestHelmDocMenu_SpaceCloses`、`TestHintPopup_SpaceCloses` 等改寫成「Space 不作用」，不要刪。


## 5. panel 上的 `Space` 不一定開 Space menu —— K5、M2、M7、F1

**現況**（`app.go` `Update()` 的 `case " "`）：

- panel 1：`hintPopup.OpenWithActions()` —— 上半是可執行的列（Pin / Unpin、Sort、Drag），下半接 `sidebarHintContent()` 的唯讀小抄，
  一個框同時是 menu 與 note；cursor 在分類標題上時只剩小抄。
- panel 2 空清單：`hintPopup.Open()` + `panel2EmptyHintContent()`，唯讀。
- panel 2 Helm Release 列：`helmDocMenu.Open()`（Manifest / Notes / Values / Hooks），不是 Space menu，沒有分區也沒有 global。
- panel 2 container drill：`panel2Menu.OpenForContainer()` 只有 `Shell`；pod 沒有 container 時 `Space` 沒有回應。
- panel 3 History：`Space` 直接對 cursor 列開 rollback confirm；cursor 在目前部署的 revision 上時什麼都不做。
- panel 3 Logs / Events / Conditions、Relatives depth 1：`hintPopup.Open()` 的唯讀小抄（`logsHintContent()` 等）。
- panel 3 Relatives depth > 1：開 breadcrumb popup。
- panel 3 KubeConfig Contexts 的 `Info` tab：沒有分支接住，`Space` 沒有回應。

**規則**：K5、M2 —— focus 在 panel 上時 `Space` 一定打開 Space menu，列出當前 panel 能做的所有事、可以直接執行，依
`item operation` → `panel operation` → `Global operation` 一列排列。M7 —— 沒有可執行的動作也照樣打開。F1 —— 一個 popup 同一時間
只屬於一類；唯讀的按鍵說明是 note，屬於 `?` 的 key reference（K6、M4），不進 Space menu。

**怎麼改**：每個 panel / tab 的 `Space` 都組一份 Space menu，最後一列 `Global operation`（第 6 條）：

- Helm Release 列：幾份文件是 item operation，跟 `[Y]AML` 同一區；選一份時 Space menu 留在 YAML 底下（F4），現在 helmDocMenu 的
  「連看幾份不必重開」照樣成立。`helmDocMenu` 拿掉。
- History：`Rollback to this revision` 是 item operation，目前部署的那一列上變暗（M6）；confirm 從那一列的 `Enter` / 熱鍵打開。
  panel 上的 `Enter` 見第 16 條。
- Relatives depth > 1：「跳回鏈上的祖先」是 panel operation，那一列 `Enter` 才打開 breadcrumb（Space menu 留在底下）。
- Logs / Events / Conditions / Info / 空清單：小抄裡能執行的（`y`、`z`、`G`、`/`、`.`、`Y`……）變成列，放進對應的區（第 7 條）；
  `N`、`C` 屬於 global operation popup；純導覽鍵（`j/k`、`u/d`、`gg/G`）搬到 key reference（第 11 條）。
- container drill：Shell 列；pod 沒有 container 時照樣打開（只剩 panel 與 global）。
- `hintPopup` 的「上半可執行、下半唯讀」混合體不再需要；`*HintContent()` 的鍵表改成各 panel key reference 的來源。拖曳模式的
  Drop menu 見第 14 條。
- 改完每個 menu 用 80 × 40 render 一次量高度（L1）。
- README 兩份「四個鍵」表 `Space` 的說明（menu 或 cheatsheet）、「Helm 專用」表的兩列 `Space`、滑鼠表「右鍵點 row」的說明、
  dev-remarks「功能筆記」Helm releases 與 Relatives 的 `Space` 描述一起改。


## 6. Space menu 沒有 global 那一列，也沒有 global operation popup —— M2、M4、M5、K9、F4

**現況**：

- `panel2menu.go` `buildPanel2MenuItems()` 只組 item / panel 兩區，而且只有這個種類有 Sort 欄位時才加區塊標題，否則整份扁平；
  `Open()` 在 drill 鏈裡把 `Esc ↖` 列插在最前面（有標題時在 `item operation` 標題之上）；`OpenForContainer()` 只有一列、沒有標題。
- panel 1 的 Space menu（`app.go` `case " "`）只有 Drag 存在時才分兩區加標題，否則扁平；它的列（`hintAction`）只有名稱、
  沒有說明。
- 沒有 global 那一列，也沒有 global operation popup。全域動作（`N` namespace、`C` context、`Alt-t` Alterm、`>` Settings、
  `!` App log、`q` 離開）只有熱鍵，唯一的列表是唯讀的 `help.go` `helpContent()` Global 段。

**規則**：

- M2 —— panel 上的 Space menu，item 與 panel 兩區一律加區塊標題（即使只剩一區）；最後固定一列 `Global operation`，**不加標題**
  （v0.1.7），跟上面用分隔線隔開；沒有對象就沒有那一區。
- M4 —— `Global operation` 列 `Enter` 打開 global operation popup：一種 menu，疊在 Space menu 上，列出**全部**全域動作，`j/k` 選、
  `Enter` 或熱鍵執行；離開 app 必須在這裡（K9）；`Esc` 回到 Space menu（F4）。
- M5 —— menu 的每一列是名稱 + 一句單行說明。

**怎麼改**：

- 全域動作定義成一份清單（sshu、filu 的 `globalActions`）：`[N]amespace`、`[C]ontext`、`[Alt-t]erm`、`[>] Settings`、
  `[!] App log`、`[q]uit`，每列一句說明；global operation popup 只讀這一份。kbu 只有一個畫面，沒有要照 M6 變暗的「目前畫面」列；
  `?` 是 core key，不列。原本擔心的 `C` 衝突（panel 2 的 Compare 與全域的 context）不在同一個 menu 裡：`[C]ontext` 是 global
  operation popup 自己的熱鍵。
- 所有 Space menu 走同一個組法：item、panel 兩區（非空才出現、一律加標題）+ 分隔線 + `Global operation` 一列（sshu
  `regions(item, panel)` + `withGlobal`、webu `withGlobal()`、locku `regions()` + `openMenu()`）；`panel2menu.go` 與 panel 1 各自
  組的「兩區 + 分隔線」收成一份。panel 1 的列補說明。
- **`Enter ↘` / `Esc ↖` 兩列**（已定案，2026-09-28）：現在名稱欄只有鍵名加箭頭，動作名稱寫在說明欄。M5 —— 左邊是動作名稱、
  右邊一句說明；D4 —— core key 直接寫進 label。改成 `[Enter] Drill in`（說明依種類寫出子項，例 `into its pods`，照
  `panel2DrillLabel()`）放在 item 區；`[Esc] Back`（說明寫出上一層，例 `to the Deployments list`）放在 panel 區，不再排最前面。
  確切措辭由 kbu 定。家族前例：webu `[Enter] Switch to`、`[Esc] Back to the page`，locku `[Enter] Edit`。
- `Global operation` 列沒有熱鍵：用一個按不出來的 key（locku 用字串 `"global operation"`），在 Space menu 分支裡攔下來開 popup。
- F4：從 global operation popup 開出的 namespace / context picker、Settings、App log 疊在它上面，`Esc` 回 global operation popup，
  再 `Esc` 回 Space menu；完成（選定 context）才清掉整疊（T1）。Alterm 照現在的 `closeAllBlockingPopups()`（context-shift）。
- 量「兩區」形狀的既有 menu 測試會紅：改靠 key 定位 global 列（sshu 的 `globalMenuKey`），不要靠字串，也不要刪。
- README 兩份「全域」表、dev-remarks「按鍵筆記」的 panel 2 context menu 段一起改。


## 7. 只能靠熱鍵的 panel 動作 —— M3

**現況**：下列 item / panel operation 只綁熱鍵，任何 Space menu 裡都沒有：

- `/` 搜尋（panel 1 `sidebar.go`、panel 2 `table.go` 的 `handleKey()`）
- `.` 切換 helm 管理物件的可見性（panel 2，`app.go` `case "."`）
- `y` 複製（每個 panel，`app.go` `case "y"`）
- `z` 放大（panel 2、3，`app.go` `case "z"`）
- `G` 回到 live（panel 3 Logs / Events，`detail.go` `handleKey()`）
- `Y`（panel 3：Relatives 的 cursor 那一筆、其他 tab 是目前這一層的 YAML，`app.go` `case "Y"`）
- `h` / `l`、`[` / `]` 切 panel 3 的 tab
- 離開 compare 模式（只能 `Esc`；`buildPanel2MenuItems()` 的註解寫明「Exit compare mode is NOT a menu entry」，`Unmark` 只出現在
  anchor 那一列）

**規則**：M3 —— 每個 panel 的每個 item operation 與 panel operation 都在該 panel 的 Space menu 裡；letter hotkey 是清單裡某一列的
捷徑。只能靠熱鍵觸發、哪裡都找不到的動作是違反。

**怎麼改**：以上放進對應 panel 的 Space menu：有 cursor 的地方（panel 1、2，panel 3 的 Relatives / History）`y` 是 item operation，
其他是 panel operation；`Y` 在 Relatives 是 item operation；其餘是 panel operation。panel 3 加一列切換 tab（filu 的 `Switch tab`）。
compare 鎖定中，panel 2 每一列的 Space menu 都有 `Exit compare mode`。README「以下每一項在 `Space` menu 裡都找得到」那段做完才成立。


## 8. 暫時不能執行的列被藏起來 —— M6

**現況**：

- `panel2menu.go` `buildPanel2MenuItems()`：helm 管理的列 `canEdit` / `canDelete` 為 false，`Edit` / `Delete` 直接不出現；panel 上
  按 `E` / `D` 則跳 toast「Helm-managed (read-only)」（`app.go` `case "E"`、`case "D"`）。
- 同一個函式：清單只有一列時（`compareCtxForMenu()` 的 `canLock = len(m.items) > 1`）`Mark as Compare anchor` 不出現。
- `app.go` panel 1 的 `case " "`：pin 少於兩個時 `Drag to reorder pinned item` 不出現。

**規則**：M6 —— 對象存在、但現在不能執行：列照樣出現、變暗，說明欄維持原本那句，不另外寫原因；cursor 可以停，`Enter` 與熱鍵都
不作用。對象不存在才不出現。

**怎麼改**：清單只有一列時 `Mark as Compare anchor` 變暗；pin 不足兩個時 Drag 變暗（cursor 不在 pinned 列時可以不列：對象不是
pinned 項目）。helm 管理的列照樣列出 `[E]dit` / `[D]elete` 並變暗，panel 上的 `E` / `D` 對 helm 列也不再跳 toast（locku、sshu
都拿掉了「按了跳原因」）。判斷參考：filu 的 `Favorite` 只在目錄上列出，user 裁定「檔案**永遠**不能收藏，不是現在不能」所以不列；
若 user 認定 helm 管理的物件在 kbu 裡永遠不能改（Rule A），可以維持不列，把這個判斷寫進 dev-remarks「設計決定」（Events、Contexts
的 Edit / Delete 屬於這種，本來就不列）。dev-remarks「按鍵筆記」的「helm 管理的列隱藏 `E`/`D`」一起改。


## 9. sort 流程：兩步擠在同一個 picker、panel 1 的 menu 先關掉自己 —— F1、F4、F7

**現況**（`app.go` `openSortColumnPicker()`、`openSortDirectionPicker()`、`commitSortFlow()`、`resetSortFlow()`；`listpicker.go`）：

- 選欄位 → 選方向是同一個 `listPicker` 原地換內容（已開著時 `Open()` 走 swap 動畫、`pendingItems`）；方向選定後
  `commitSortFlow()` 又換回選欄位那一步。框的高度跟著步驟變。
- 方向那一步按 `Esc` 關掉整個 picker（`ListPickerCancelMsg` 清掉 `sortFlowKind`），回不到選欄位那一步。
- 欄位那一步的 Reset 列只在已有排序時出現，排序一建立，開著的框就多一列。
- panel 1 的 Space menu 選 `Sort panel 2 list` 時，`hintPopup.commitAction()` 先 `Close()` 自己再送 `HintActionMsg`，sort picker
  底下沒有 source，`Esc` 回到 panel 而不是 menu（`Update()` 裡 listPicker 路由的註解說它疊在 sidebar Space menu 上，實際沒有）。
  panel 2 的 Space menu（`panel2Menu.commit()` 不關自己）沒有這個問題。

**規則**：F1 —— 多步驟的流程，每一步是自己的 popup，疊起來保留 source（F4），每一步有自己打開時定好的高度（F7）。F4 —— 從 popup
A 開出 popup B 時 A 留在底下，取消 B 回到 A。

**怎麼改**：方向另開一個 popup，疊在欄位 picker 上（filu 的 `sortDirMenu`）；`Esc` 回到選欄位那一步；方向選定後關掉方向那一步、
欄位 picker 的 badge 原地更新（kbu「連續疊加多個 tier」的做法照舊）。Reset 列一律在、沒有排序時變暗（M6；filu 同一招，免得開著時
長出一列）。Space menu 執行一列時不先關自己，由目標決定（照 `panel2Menu.commit()`；做了第 5 條的統一 Space menu 就一起解決）。
原地換內容的 swap 動畫不再需要；dev-remarks「列表排序」與「依層數決定的 popup 邊框」裡「原地換內容的 picker 保留原本的 layer」
一起改。


## 10. namespace / context picker 的 `/` 篩選沒照 F1 的階段 —— F1、K2、K3、K4

**現況**（`namespace.go`、`context.go` 的 `Update()` 與 `handleSearchKey()`）：

- `/` 進入打字（上方多一個三列的搜尋框）；打字時方向鍵移動候選、字元進搜尋字串；`Enter` 只是離開打字、保留篩選，要再按一次
  `Enter` 才勾選 / 切換；`Esc` 清掉篩選、回到清單。
- 清單上有篩選時 `Esc` 先清篩選、再按一次才關；`Tab` 沒有作用。
- 打字時 `Space` 打不出來（第 15 條）。

**規則**：F1 —— 一個 popup 可以依階段換類別（打字時是 input，清單取得 focus 時是 menu），`Tab` 在打字與清單之間切換 focus，
`Esc` 關掉整個 popup（K4：階段不是一層）；input 附候選清單時，可列印的鍵都是字元、只有方向鍵在候選之間移動，`Enter` 送出選中的
那一筆（K3）。

**怎麼改**：照 filu、webu 的 finder：打字中 `Enter` 直接對反白那一筆做清單的 `Enter`（namespace 勾選、留在打字；context 切換並
關閉）；`Tab` 在打字與清單之間切換；任何階段 `Esc` 關掉整個 picker（篩選隨之清掉）。下框 hint 依階段寫 `Tab` 的去向。dev-remarks
「功能筆記」多 namespace 選取的 `/` 說明一起改。


## 11. `?` 的 key reference 是全 app 共用一份，不是最前端 surface 的 —— K6、M4

**現況**：`app.go` `case "?"` 切換 `help.go` 的 `HelpModel`（標題 `󰘳 Keybindings`），內容是 `helpContent()` 的 Core / Navigation /
Global / Alterm 四段。它已經唯讀、可以捲動，但：

- 不論 focus 在哪個 panel、哪個 tab，都是同一份；註解寫明「Per-context trigger letters (Y/E/S/D) aren't listed」，panel 自己的鍵
  （`Y` / `E` / `S` / `D` / `C`、`P`、`Alt+Shift+S`、`.`、`G`……）一個都不在上面。
- `Core` 段只有 `Tab` / `Enter` / `Esc` / `Space`，`?` 與 `q` 放在 `Global`；`Space` 的說明是 `Open menu / close popup`（第 4 條）。
- footer（`statusline.go` `hints()`）寫 `Esc exit`，讀起來像「離開 app」；`Esc` 永遠不離開 app（K4）。

**規則**：K6、M4 —— `?` 打開**最前端那個 surface 的 key reference**：唯讀、可以捲動，沒有游標、不能執行。focus 在 panel 上時至少
列出這個 panel 能按的鍵與 core key；其餘列不列由 app 決定。popup 上見第 12 條。

**怎麼改**：key reference 依 focus 的 panel（panel 3 依 tab）組：先列這個 panel 的鍵 —— 從它的 Space menu 列產生（只收按得出來的鍵：
單一字元、`Enter`、label 裡寫出的 `[/]`；sshu、webu 的 `keyReference(items)`），加上第 5 條從小抄搬來的導覽鍵 —— 再接 core key
（`Tab`、`Enter`、`Esc`、`Space`、`?`、`q`；panel 自己寫了 `Enter` 時不重複通用的那一列）。全域熱鍵與 Alterm 的 scrollback 鍵要不要
留一段由 kbu 決定。寬度照 F7（第 18 條）。footer 的 `Esc exit` 換一個不會讀成離開的字（例 `Esc back`）。README 兩份「全域」表的
`?` 說明一起改。


## 12. `?` 在 popup 上沒有回應 —— K6、F6

**現況**：只有 `help.go` 處理 `?`（關掉自己）；其他 popup（Space menu、confirm、YAML、Compare、namespace / context picker、sort
picker、Settings、breadcrumb、helm 文件 menu、App log）按 `?` 都被 popup 吃掉、沒有反應。

**規則**：K6 —— `?` 在任何 surface 都有回應；focus 在 popup 上時（Space menu、global operation popup 也是 popup），打開**只有這個
popup** 能按的鍵的 key reference：唯讀、可以捲動。再按 `?` 或 `Esc` 關掉。F6 —— confirm 自己的熱鍵（`y` / `n`）要列在 confirm 的
`?` 裡。

**怎麼改**：每個 popup 給一份自己的鍵（menu：`j/k`、`g/G`、`Enter`、各列熱鍵、`Esc`，Space menu 另有 `Space`；confirm：
`Enter <動詞>`、`y`、`n`、`Esc`；YAML：移動鍵、`v`、`y`、`/` `n` `N`、`E`，visual 模式另一份（第 14 條）；Compare：捲動鍵、第 3 條的
版面熱鍵；picker：`Enter`、`/`、`Tab`、`Esc`；App log：`y`、`D`……），疊在最上層顯示。key reference 在第 1 條的清單裡永遠在最上面
（路由、繪製、亮暗、滑鼠都是）。


## 13. `q` 與 `Ctrl-C` 行為不同，在 popup 與輸入態上沒有作用 —— K9、K1、K8

**現況**：

- `app.go` 主 switch 的 `case "ctrl+c"` 直接 `tea.Quit`（只停 watcher 與 log 串流）；`case "q"` 走 `quitMsg`（停 PTY、寫
  `state.yaml`）。所以 `Ctrl-C` 離開不存 session 狀態，也不停 Alterm 的子程序。panel 搜尋打字中（`searching` 分支）的 `Ctrl-C`
  同樣直接 `tea.Quit`。
- popup 開著時，按鍵在 `Update()` 前段就被該 popup 吃掉，`q` 與 `Ctrl-C` 都沒有作用（PTY 依 K10 例外）；namespace / context picker、
  YAML 的搜尋打字中也一樣。拖曳模式裡 `q` 是取消拖曳（第 14 條）。
- 離開不在任何可執行的清單裡（第 6 條）。

**規則**：K9 —— `q` 與 `Ctrl-C` 做同一件事：進入離開流程；`q` 在每個非輸入態的 surface 都有效，`Ctrl-C` 連輸入態都有效；離開流程
進行中再按一次 `Ctrl-C` 立刻離開；離開列在 global operation popup 裡。離開流程由 app 決定。

**怎麼改**：`Ctrl-C` 改成走跟 `q` 同一個 `quitMsg`；兩者的處理提到 popup 路由之前、splash 之後（輸入態的 `q` 是字元；PTY 裡照 K10
屬於子程序）。global operation popup 的 `[q]uit` 也走 `quitMsg`。kbu 已定「`q` 直接離開、不確認」（`48e540b`），`Ctrl-C` 跟著。
日後若要加確認（例如 Alterm 還活著），用**自己的** quit confirm popup 疊在整疊最上面，它的 `?` 也是自己的（D3；locku、sshu、webu 的
`quitAsk` + `quitHelp`），確認開著時的 `Ctrl-C` 直接離開。README 兩份「全域」表的 `Ctrl+C` 說明一起改。


## 14. 模式裡的 core key：拖曳與 YAML 的 visual —— K11、M1、M3

**現況**：

- 拖曳（panel 1，`sidebar.go` `handleDragKey()`；`app.go` `case tea.KeyMsg` 開頭把 `Ctrl-C` 以外的鍵都送給 sidebar）：`j` / `k`
  移動、`Enter` / `D` 放下；`Space` 送 `SidebarDragRequestDropMenuMsg`，開一個只有 `Drop` 一列的 `hintPopup`（可以執行的模式
  按鍵清單）；其餘 —— `Esc`、`Tab`、`1–3`、`q`、`?` —— 一律取消拖曳；`Ctrl-C` 直接 `tea.Quit`（第 13 條）。模式的鍵只揭露在一個
  置中、不會自己消失的 sticky toast（`SidebarDragEnterMsg`：`Drag mode · j/k move · Enter or D drop · anything else cancels`）與
  panel 1 標題的 `[D]rop`。
- YAML popup 的 visual（`yamlpopup.go` `Update()` 的 `v`）：`Esc` 離開 visual 沒問題；但 `Space` 關掉整個 YAML popup，`?`、`q`、
  `Ctrl-C`、`Tab` 沒有反應。

**規則**：K11（v0.1.10 起）—— 模式裡 `Space` **不開任何 menu、不作用**；`?` 是這個模式的 key reference（唯讀）；`Esc` 離開模式、
回到進入前的狀態；`q` / `Ctrl-C` 照 K9 進入離開流程；`Tab` 可以暫停，但按了要有回應（說明先 `Esc`）。模式裡沒有任何可以執行的
按鍵清單；模式自己的鍵直接按，揭露在 `?` 與 footer / 下框 hint；footer 照樣有 `?`（M1）。

**怎麼改**：

- 拖曳：`Space` 不作用（不開 menu，也不取消）；`SidebarDragRequestDropMenuMsg`、`HintActionMsg` 的 `DropPinned` 與它的 `hintPopup`
  接線整個拿掉（`TestSidebarModel_Drag_SpaceOpensDropMenuNotCancel` 改寫成「Space 不作用、不取消」）。`?` 開拖曳模式的 key reference
  （`j` / `k`、`Enter` / `D`、`Esc`）；`q` / `Ctrl-C` 走離開流程（離開前照舊取消拖曳、還原順序）；`Tab` 與數字鍵回一個 toast
  「先 `Esc` 離開拖曳」。其他鍵仍取消拖曳可以保留（mouse 也是）。模式的鍵搬到 footer 或 panel 1 下框 hint（`?` 開頭、不必列
  `Space`），sticky toast 可以拿掉。
- YAML visual：`Space` 不作用；`?` 開 visual 模式的 key reference；`q` / `Ctrl-C` 照第 13 條；`Tab` 回 toast。
- dev-remarks「功能筆記」的拖曳段（Drop 精簡 menu、sticky toast 帶著鍵盤契約）、「分級的 toast 通知」的 sticky 版、YAML popup 段
  一起改。


## 15. 搜尋列打不出空白 —— K8

**現況**：各搜尋列的 `handleSearchKey()` 只把 `tea.KeyRunes` 當字元（`table.go`、`sidebar.go`、`yamlpopup.go`、`namespace.go`、
`context.go`）；Bubble Tea v1 的空白鍵是 `tea.KeySpace`，所以打字中按 `Space` 什麼都不發生。YAML popup 的 `/` 要找 `image: nginx`
這種含空白的字串時打不出來。

**規則**：K8 —— 輸入態下 `Space`、`?`、`q` 與 letter hotkey 一律當成字元輸入。

**怎麼改**：各搜尋列把 `tea.KeySpace` 也當成字元（`sidebar.go` 的拖曳分支已經兩種都認，照同一種寫法）。資源名稱本身不含空白，
但 YAML 內容會有。


## 16. `Enter` 沒有動作的 panel、tab 與列 —— K3

**現況**：

- panel 1 kind 列（`sidebar.go` `handleKey()` 的 `tea.KeyEnter`）：刻意 no-op。註解：滑鼠雙擊合成 `Enter`，以前的「focus 移到
  panel 2」會讓雙擊把 focus 帶走。
- panel 2：`app.go` `case "enter"` 只在可 drill 的種類（`SupportsDrillDown()`、Pod → container）有動作；其他種類（ConfigMap、
  Service、KubeConfig Contexts……）`enterDrillDown()` 回 `nil`；container 列（`m.drillDownPod != nil`）落到 `table.go` 的 `KeyEnter`，
  也是 no-op。
- panel 3（`detail.go` `handleKey()`）：只有 Relatives 的 `Enter` 會 drill；Logs / Events / Conditions / Info 是沒有項目的內容區、
  History 有 revision 的 cursor，`Enter` 都沒有動作（History 的 rollback 在 `Space`，第 5 條）。

**規則**：K3 —— `Enter` 對 focus 項目做最直觀的那個動作，同一種項目在同一個 app 裡永遠同一個動作；panel 本身是內容區、沒有項目
可選時（例：預覽、log），`Enter` 對整個 panel 做最直觀的動作，由 app 決定（例：開一個可捲動的檢視；v0.1.11）。

**已定案**（user，2026-09-28）：

- panel 1 kind 列：`Enter` 把 focus 移到 panel 2（以前的行為）；滑鼠雙擊在 panel 1 只選列、不合成 `Enter`（X2：滑鼠對應哪個鍵由
  app 決定）。`sidebar.go` 那段註解、README 兩份滑鼠表「雙擊 = `Enter`」、dev-remarks「滑鼠支援」一起改。
- panel 2 不能 drill 的種類：`Enter` 開 YAML popup，跟 `[Y]AML` 同一條路。
- panel 2 的 KubeConfig Contexts：`Enter` 切換到這個 context，**先跳 confirm**（問句寫出 context 名稱，hint `Enter switch ·
  Esc cancel`，D3）；接受後跟 `C` picker 選定走同一條路。已經是目前的 context 時不作用，Space menu 那一列變暗（M6）。
- panel 2 的 container 列：`Enter` 開 shell，跟 `[S]hell` 同一條路（`execShell()`，含它現有的「Exec into container?」confirm）。
- panel 3 內容 tab（Logs、Events、Conditions、Info）：`Enter` 跟 `z` 一樣放大這個 panel。
- panel 3 History：`Enter` 開 rollback confirm（現在 `Space` 的那條路；第 5 條把 `Space` 改回 Space menu）。目前部署的那一列
  `Enter` 不作用，Space menu 裡的 rollback 列變暗（M6）。

每個 `Enter` 動作同時是該 panel Space menu 的一列（`[Enter] …`，D4）；README 兩份「四個鍵」表的 `Enter` 說明與 dev-remarks 相關段落
一起改。每處補 model test。


## 17. `kubectl edit` / `kubectl exec` 的 PTY 沒有出口鍵 —— K10

**現況**：`ptyview.go` `Update()` 只有 `PtyKindShell`（Alterm）攔出口鍵 `Alt-t`；edit / exec 的 PTY 把所有鍵送給子程序，只能等
子程序結束才離開。下框 hint（`renderBottomBorder()`）對 edit / exec 只寫 `PgUp/Home:scroll`，alt-screen 時（`kubectl edit` 的 editor
正是這種）什麼都不寫。

**規則**：K10 —— app **至少**要有一個讓 focus 離開 PTY 的出口鍵，並在 focus 位於 PTY 時常駐揭露它；出口鍵以外保留的 app 組合鍵
（kbu 的 `PgUp` / `PgDn` / `Home` / `End` scrollback）由 app 決定、同樣常駐揭露；按了出口鍵之後 focus 落在哪裡由 app 決定。

**怎麼改**：edit / exec 也接出口鍵（`Alt-t` 或另一個）：exec 可以像 Alterm 一樣隱藏、之後再叫回來；edit 至少要能離開（例：先 confirm
「放棄這次編輯？」再結束子程序）。出口後 focus 回到 panel 2 原本那一列即可。下框 hint 在任何狀態（含 alt-screen）都寫出口鍵。README
兩份「PTY popups」表一起改。


## 18. popup 的尺寸與位置 —— F7（D3）

**現況**：見上方「popup 盤點」。每個 popup 各自算寬（依內容夾在 85% / 70% / 75% W、固定 44 / 54、W − 2 沒有上限）；YAML、Compare
固定 H − 2、key reference 內容區固定 H − 8、App log 固定 60% H，都跟內容無關；toast 置中（`View()` 的 `overlay.Center`），寬度依訊息、
沒有上限，長訊息會超出畫面；Alterm 與 `kubectl edit` / `exec` 的外框是 W − 4 × H − 3（`ptyDims()`）。sort picker 換步驟時高度會變
（第 9 條）。namespace picker 是唯一開著時 loading 的 popup：標題後面有 braille 轉圈（`namespaceSpinnerFrames`，十格、約 80ms、
計數驅動）。

**規則**：F7 ——

- 寬度 `min(W − 2, 120)`，水平置中；terminal 類例外，寬高用滿可用範圍（W − 2 × H − 2），不受 120 上限。
- 高度依內容，打開時定好，上限是畫面扣上下留白，超過就在框裡捲動；只有 loading 期間與使用者自己的操作可以改變高度。
- loading 時標題後面**一定**放一個輪轉的 loading icon（規格見 D3）。
- 位置垂直置中；toast 固定在畫面下方，寬度照同一條規則。
- 送出可能失敗的 input 預留一列錯誤列 —— kbu 沒有 input popup，不適用。

**怎麼改**：

- 一個共用的寬度 helper，每個 popup 的 render 只用它；說明太長時在框裡換行或截尾，不為它加寬（D4）。原本依內容算寬的程式刪掉
  （webu：固定寬度讓「依內容」的保護變成沒用的程式）。
- 高度依內容、開框時定：YAML、Compare、key reference、App log 改成依內容、上限畫面扣留白（filu `[2]` viewport 的修法）；App log 開著時
  新進來的 entry 不改高度。
- toast 固定在畫面下方（filu、webu、locku 都是 `overlay.Bottom` 再上移、不蓋 footer），寬度照同一條規則。
- terminal 類：外框用滿 W − 2 × H − 2（sshu 的做法），或內容區 W − 2 × H − 2、外框貼滿畫面（filu 的裁定），兩種都算符合。
- loading icon：namespace picker 已經符合 F7（標題後面、只在 loading 時轉）。D3 的規格（Nerd Font circle slice U+F0A9E–U+F0AA5、
  一格 90ms、時鐘取格、一格寬，braille 不用）是家族預設，不換不算違反；要跟家族一致就照 webu `internal/ui/theme.go` 的
  `spinnerFrames` 與 filu `internal/ui/loading.go` 的 `loadingIcon()`。
- 測試：寬度在 W = 80 與 200 各量一次（200 才看得到 120 上限）；預期值寫死成條文算出來的數字；量寬度用短內容。popup 上框用顯示寬度
  量標題（filu 第三輪：CJK icon 字型上 glyph 佔兩格時上框會多一格）。
- dev-remarks「依層數決定的 popup 邊框」與「Popup 的分類與結構」提到尺寸的地方一起改。


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


## 20. statusbar 的寬度跟著內容浮動，還會折成多列 —— L2、L3、L4

**現況**：`statusbar.go` `ViewFull()` 把 `[C]ontext: <name>`、`[N]amespace: <name>` 直接串起來，後面接 `[Alt-t]erm`、`[C]ompare` chip；
右邊的 badge（`! N errors`、`N warnings`、`✓ …`）寬度也隨內容變，切 context / namespace 時後面的 chip 左右位移。整段用
`barStyle.Width(m.width − badge 寬)` 渲染，超寬時 lipgloss **折行**：2026-09-28 在 scratch 複本實測 80 欄，context `kind-dev` +
namespace `kube-system` + 兩個 chip + `! 3 errors` 就折成 2 列；EKS ARN 那種長 context 折成 3 列，而且沒有 badge 的那幾列寬 68、
不等於終端機寬。`panelSizes()` 把 statusbar 當 1 列算，折行時整個畫面比終端機高。dev-remarks「Panel 外框」寫「statusbar 一列……列數
鎖死」，目前不成立。

**規則**：L2 —— statusbar 等動態文字用固定寬度的欄位或 padding。L3 —— 常駐列的列數鎖死，放不下就截斷或從尾端捨棄，不折行。L4 ——
每一列剛好等於終端機寬度。

**怎麼改**：context 與 namespace 各給固定寬度的欄位（過長中間截斷，跟 panel 2 的 Name 欄同一種做法），chip 與 badge 用固定寬度；
整列先截到終端機寬再 render，不交給 lipgloss 折行，放不下從尾端捨棄。補一個跨尺寸的整畫面測試（D6 的做法：多種終端機尺寸下每一列
剛好等於終端機寬，含 popup 疊上去與長 context 名稱）—— 舊清單待確認的 L4 那題併進這裡，statusbar 正是這種測試會抓到的例子。


## 21. splash 只認三個鍵 —— S3

**現況**：`splash.go` `Update()` 只有 `"esc", "enter", " "` 會關掉 splash；其他鍵（`q`、`Ctrl-C`、`?`、字母）被 `app.go` 的 splash 分支
吞掉、什麼都不做，splash 繼續開著；提示字 `Press Esc to close`。舊清單「先看」說 S3 已符合，只核對了路由順序（splash 在 `Ctrl-C`
之前），沒看 splash 自己收哪些鍵。

**規則**：S3 —— splash 開著時，**任何鍵**都只會關掉它，包括 `q`、`Ctrl-C`、`Esc`、`Space`、`?`；那個鍵不會再做別的事。

**怎麼改**：splash 開著時任何 `tea.KeyMsg` 都關掉它（路由位置不動，仍在 `Ctrl-C` 之前）；提示字改成「任何鍵關閉」的意思。測試：`q`、
`Ctrl-C`、`?`、一個字母各按一次，splash 關掉、app 沒有離開、沒有開出任何 popup。


## 22. panel 2 drill-down 失敗時沒有任何訊息 —— F5

**現況**：`app.go` `enterDrillDown()` 的非 Pod 分支，`k8s.FetchChildResources()` 回錯誤（或零個子項）時 cmd 回 `nil`：`Enter` 按下去
什麼都沒發生，App log 也沒有記錄。Relatives 的 drill 失敗（`relativeDrillFetchedMsg`、`resourceFetchedForDrillMsg`）有 warn toast 與
App log，這裡沒有。

**規則**：F5 —— 錯誤必須立刻看得到（toast 或 popup），`Esc` 可關，不能阻塞 app。

**怎麼改**：錯誤寫進 App log 並跳 warn toast（跟 Relatives 的 `drill failed` 同一種）；零個子項不是錯誤，要不要說一聲由 kbu 決定。


## 23. 刪除完成後 Space menu 還留在畫面上 —— T1

**現況**：panel 2 Space menu 選 `[D]elete` → confirm 疊在 menu 上；`Enter` 接受後 confirm 關掉、`deleteResource()` 在背景跑，menu 還開著，
標題與各列都指向剛刪掉的資源（`DeleteDoneMsg` 只寫 App log）。

**規則**：T1（概念）—— 預設保留 source；功能上判斷「完成 target 之後 source 已經失去意義」時清掉。家族預設（D3）是「取消回到
source；完成動作清掉整個 stack」。

**怎麼改**：delete confirm 接受時清掉整疊；取消照舊回到 menu（F4）。filu 的做法：每一個完成點各寫一次，各補一個「Space menu 在底下，
完成後它也不見」的測試。同一輪把 kbu 其他完成點對一次：context 選定（已關）、breadcrumb 跳轉（`SwitchToResourceMsg` 已關）、Mark /
Unmark anchor（已關）、Edit / Shell（context-shift 已清）、sort（刻意回到選欄位那一步）。


## 24. README 的需求沒寫 truecolor —— D6

**現況**：兩份 README 的需求段（`README.md`「Requirements」、`README-zh_TW.md`「需求」）只寫 Nerd Font。

**規則**：D6 —— 家族要求 truecolor terminal（24-bit 色）：catppuccin 的淡色與 D2 的層色漸變在 256 色下分不出來，dim 也一律輸出
24-bit（第 19 條）；README 的需求段跟 Nerd Font 並列寫明。

**怎麼改**：兩份 README 的需求段並列一行（filu、locku、sshu、webu 都已加）。這是文件，建議跟第 19 條同一輪做。


## 25. 註解與 dev-remarks 仍引用舊的 popup convention 與舊分類 —— 文件對齊

**現況**：

- `internal/ui` 的註解與測試訊息用 popup convention 的 § 編號（`§1.6`–`§1.10`，源自 VTP 時期的 kbu implementation 文件 §6）與
  「design-guide §3.2」。popup convention 在本機的 `.claude/rules/popup-convention.md`，那個目錄不進版控，公開的程式碼讀者找不到；
  VTP 時期的 implementation 文件已經退場。不影響行為。`internal/k8s/contexts.go` 的 `project rule §7` 指的是本機的
  `.claude/rules/project-rules.md`，不動。
- `dev-remarks.md`「Popup 的分類與結構」寫「四類（tdp F1）：menu、message、viewport、pty」；tdp v0.1.8 起是六類（menu、confirm、input、
  note、toast、terminal）。同一節與「設計文件導讀」指向本機不進版控的 `.claude/rules/popup-convention.md`。

**怎麼改**：照 [terminu `vtp/README.md` 的對照表](https://github.com/vulcanshen/terminu/blob/v0.1.13/vtp/README.md) 與各處註解的內容
換成 tdp 編號：`§1.10` context-shift 清掉 source = T1、`§1.8` source 保留 = F4、`§1.9` `Esc` 含自動消失 = F3、層色與警示色 = D2、
標題 glyph + 文字 = D3。在該檔找「現在」那串，數量跟下表一致就直接換（共 39 處）：

| 檔案 | 現在（處數） | 換成 |
|---|---|---|
| `app.go` | `§1.10`（11，含 `convention §1.10`、`(§1.10)`、`via §1.10`） | `tdp T1` |
| `app.go` | `§1.8`（2：listPicker 路由、Compare-to-anchor 留住 menu） | `tdp F4` |
| `app.go` | `§1.9`（1：`case "esc"` 的 toast） | `tdp F3` |
| `app.go` | `.claude/rules/popup-convention.md §1.6`（1：`popupDepth()` 上方，講層色） | `tdp D2` |
| `applog.go` | `popup-convention §1.7`（1：警示色） | `tdp D2` |
| `toast.go` | `popup-convention rule`（1：標題 glyph + 文字） | `tdp D3` |
| `comparepopup.go` | `see .claude/rules/popup-convention.md`（1：`menu` 欄位） | 第 3 條拿掉 `menu` 後隨之消失 |
| `panel2menu.go` | `§1.8 popup-convention`（1：`commit()`） | `tdp F4` |
| `statusbar.go` | `design-guide §3.2`（1） | dev-remarks「設計決定」的 glyph 子集 |
| `comparemenu.go` | `popup-convention`、`kbu popup convention`（3） | 第 3 條刪檔後隨之消失 |
| `app_test.go` | `§1.10`（7）、`§1.8`（3） | `tdp T1`、`tdp F4` |
| `panel2menu_test.go` | `§1.8`（3） | `tdp F4` |
| `sortflow_test.go` | `§1.8`（2） | `tdp F4` |
| `ptyview_test.go` | `popup-convention v2 §1.10`（1） | `tdp T1` |

`app_test.go` 與 `panel2menu_test.go` 有幾處是 `t.Error` 的訊息字串，不是註解，一起換。上一份清單把 `popupDepth()` 上方那處照 § 編號
對到 D3，照內容（層色）應是 D2。dev-remarks 的「四類」改成六類，各 popup 的歸類照上方「popup 盤點」與修完後的樣子。


## 26. compare 鎖定時一次 `Esc` 退兩層 —— K4（已定案）

**現況**：`app.go` `case "esc"`：panel 2 在 compare 鎖定中又在 drill 鏈裡時（例：Deployment `nginx` drill 進 Pods，在 Pods 裡標了
anchor），同一次 `Esc` 先 `clearCompareLock()`、再 `exitDrillDown()`，一次兩層；註解的理由是「按兩次不一致」。同一段裡的篩選卻是
一次一層（有篩選時 `Esc` 只清篩選）。`TestAppModel_EscOnPanel2WithCompareMode_ClearsLockAndPopsDrill` 守著現在的行為。

**規則**：K4 —— `Esc` 一次關一層。

**已定案**（user，2026-09-28）：

- compare 鎖是 panel 的狀態（跟篩選同一類），**不是** K11 的模式：鎖著時按鍵照原意，Space menu 照常開（`Compare to anchor` 就在
  裡面）。
- `Esc` 一次一層：第一次只解鎖、留在這一層；第二次才退 drill。照篩選的寫法，解鎖後就 return。
- 上面那個測試改寫成守「一次一層」（改名、反轉斷言），不要刪；`..._NoDrill_JustClearsLock` 照舊。


## 27. 開啟 popup 的熱鍵再按一次會關掉它 —— K7（已定案）

**現況**：namespace picker 的 `n` / `N`（`namespace.go` 兩處）、context picker 的 `c` / `C`（`context.go`）、App log 的 `!`
（`applog.go`）、Settings 的 `>`（`settingspopup.go`）在各自的 popup 裡等於 `Esc`；其他 popup 不認這些鍵（`n` 在 YAML popup 是
「找下一個」）。

**規則**：K7 —— 一個角色綁多個鍵時，別名必須在所有 surface 同樣有效；做不到就不要做別名。

**已定案**（user，2026-09-28）：拿掉，這四個 popup 只認 `Esc`（`Space` 在第 4 條已拿掉）。家族前例：filu 的 breadcrumb `b`。從 global
operation popup 開的，`Esc` 回到 global operation popup（第 6 條）；從 panel 直接按熱鍵開的，`Esc` 回到 panel。下框 hint 與 key
reference 不再列這些鍵；守「再按一次關閉」的測試改寫成「不作用」。`?` 再按一次關掉 key reference 是 K6 本身，不動。


## 28. Events tab 失焦變暗 —— T2（已定案：寫成偏離）

**現況**：panel 3 失焦時 Events 暗成 `TableDimRowStyle()`（`detail.go`），Logs 不暗。Events 有跟 Logs 一樣的 live ▶ / paused ⏸
追尾（`followEventsTail`），照 T2 的定義算串流內容。`detail.go` Logs 那段的註解說 Events「內容是靜態的」，跟 `followEventsTail`
的註解（「長時間觀察的人預設要看到最新的」）互相矛盾。

**規則**：T2 —— app 讓失焦的 panel 變暗時，串流內容失焦不變暗。

**已定案**（user，2026-09-28）：行為不改，Events 失焦照樣變暗。這是 kbu 的偏離，由 kbu 自己寫進 dev-remarks「偏離 tdp」（哪一條：
T2；在哪裡：panel 3 的 Events tab；為什麼：user 的理由是不在 focus 的都應該變暗）。`detail.go` 那段「Events 是靜態內容」的註解
改成指向這條偏離；dev-remarks「設計決定」的「Logs 失焦不變暗」一起對齊。


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
