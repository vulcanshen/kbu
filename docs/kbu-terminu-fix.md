# kbu — terminu fix

kbu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.17/principle)（tdp v0.1.17）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

> **v0.1.17（2026-09-29，這份清單寫完後才出）**：只補了 M5 三點，已照它調整本清單 —— menu 與 key reference **說明欄**裡提到的鍵
> 算句子，加方括號；**README** 內文的鍵用 Markdown code 標（`` `Enter` ``），不加方括號，鍵名與寫法照 M5；**別的工具自己的按鍵**
> （tmux 的 `prefix l`、`C-a x`）照那個工具的寫法。其餘條目照 v0.1.14–v0.1.16。

盤點日期：2026-09-29，依據 `main` 的 `7366dbc`（工作區乾淨）。這一輪**只對 v0.1.13 → v0.1.16 的改動**
（`git -C terminu diff v0.1.13 v0.1.16 -- principle/` 與 terminu CHANGELOG 的 v0.1.14–v0.1.16 段）：

- v0.1.14：F1 / F8 / K11 的 toast、K10 / D5 的家族 PTY 出口鍵、術語「模式」（zoom 不是模式）、M5 的範圍、M6 的 key reference。
- v0.1.15：M5 的鍵寫法全部定案 —— 鍵名、`/` 與 `–`、依位置的寫法（label / 句子 / hint 與 footer / key reference）、D2 的鍵色。
- v0.1.16：D5 的 `Alt-Esc` 一律先 confirm；M6 補「說明別的 surface 的一段照亮顯示」。

v0.1.13 全文的對照上一輪已經做完（dev-remarks「對照 tdp 時確認過的」），這裡不重做。每一處都照內容拿程式碼核對過；位置寫檔案與
函式，不寫行號。toast 與模式的 `Esc`、家族的 PTY 出口鍵（含 v0.1.16 的一律 confirm）、zoom 三項 kbu 已經照做（見「已經符合」）；
要修的是 M6 的 key reference（第 1、2 條）與 M5 的鍵寫法和顏色（第 3 條），另加 user 已定案的 loading icon（第 4 條）與 dev-remarks
的整理（第 5、6 條）。


## 先看

- **清單先 commit，再動程式**；程式的 commit 跟清單分開，commit 只加自己改的路徑。
- **「現況」照內容比對、不照行號**，動手前 `grep` 一次。
- **每修一處補 model test，逐處 mutation**：把修正單獨改回舊行為，確認對應的測試會紅；編譯不過不算抓到；預期值寫死成條文算出來的值。
  斷言畫面字串的舊測試要改寫（改斷言、名稱寫出新規則），不是刪掉。
- **同一個 commit 同步 README 兩份與 dev-remarks** 描述該行為的段落；CHANGELOG 記在 `[Unreleased]`。`[Unreleased]` 開頭那句
  「Following the terminu design principle … (tdp v0.1.13)」記這一輪時改成 v0.1.17（這份清單沒有動 CHANGELOG）。
- **建議順序**：第 1 條（key reference 的變暗）→ 第 2 條（YAML 的 `E`，用第 1 條加的欄位）→ 第 3 條（鍵的寫法與顏色；會碰到第 1、2
  條改過的 key reference 與 YAML hint，放後面免得改兩次）→ 第 4 條（loading icon，獨立）→ 第 5、6 條（文件）。
- **把畫面印出來看**：第 1 條修完在 truecolor 下看 `?` 的暗列；第 3 條修完在 80 × 40 看 footer、panel 2 / 3 的邊框 hint、各 popup
  下框，確認鍵與說明是兩個顏色、尾端捨棄的是整組。
- **修完拿 v0.1.17 全文再逐條對一次**（`rules-zh_TW.md`、`defaults-zh_TW.md`、`README-zh_TW.md` 的術語），至少把這一輪碰到的
  F1、F8、K4、K10、K11、M5、M6、D1、D2、D3、D4、D5 整節讀完。
- 修完照上一輪（`556962c`）刪掉這份清單，把「已經符合」與裁定搬進 dev-remarks「對照 tdp 時確認過的」。
- **不 push、不發版**（家族全部 app 與 tdp 穩定之前不發 release）。
- 把這一輪寫進 terminu repo 的 `.local/family-fix/kbu/README.md`（加一節：修了什麼、commit、踩過的坑，以及上一輪「tdp 可能還缺的」
  哪幾條 v0.1.14–v0.1.16 已經寫進去）。
- patch 腳本與 commit message 寫成檔案再執行（`bash -c '…'` 裡的英文撇號會截斷外層引號）。


## 1. `?` 的 key reference 沒有把現在不能按的鍵變暗 —— M6

**現況**：

- `help.go`：`helpRow` 只有 `header`、`key`、`desc`；`HelpModel.renderFullPopup()` 每一列都用同一套樣式（`DetailLabelStyle()` /
  `DetailValueStyle()`），沒有「暗」這個狀態。
- `keyref.go` `menuRows()`：panel 與 Space menu 的 `?` 從 menu 的列產生，`disabled` 的列有收進來（註解寫「Dimmed rows stay」），
  但 `disabled` 沒帶進 `helpRow`，畫出來跟按得了的鍵一樣亮。會碰到的（menu 裡都已經變暗，`?` 裡卻是亮的）：
  - panel 1：`D`（Drag to reorder pinned kinds；pin 不到兩個，`sidebarMenu()`）
  - panel 2：`E` / `D`（helm 管理的物件，`panel2ItemOps()`）、`C`（Mark as Compare anchor；清單只有一列）、`Enter`（Switch to this
    context；已經是目前的 context，`tableMenu()`）
  - panel 3：`l`（Switch tab；只有一個 tab，`detailMenu()`）、`Enter`（Roll back to this revision；目前部署的版本）
  - 以上在 panel 上按 `?`（`panelKeyRef()`）與 Space menu 上按 `?`（`keyRef()` 的 `&m.spaceMenu`）兩處都一樣。
- `panelKeyRef()` 寫死的 panel 3「move」段切 tab 的鍵（現在寫 `h l`，`previous / next tab (also [ ])`）不看 `TabCount()`：只有一個
  tab 時它不作用，照樣亮著（menu 的 Switch tab 在同一個條件下是暗的）。
- `keyRef()` 的 namespace picker（`filterPickerRows()`）：picker 在 loading 時（`NamespacePickerModel.loading`，`OpenLoading()` 到
  `SetNamespaces()` 之間）只有 `Esc` 有作用（`NamespacePickerModel.Update()` 的 loading 分支），`?` 卻把清單階段的 `j k`、`u d`、
  `gg G`、`Enter`、`/`、`Tab` 都亮著列出。

**規則**：M6（v0.1.14 新增、v0.1.16 補一句）—— 「`?` 的 key reference 照同一套：對象存在、現在不能按的鍵照樣列出、變暗；對象不存在
就不列。」「key reference 裡另外加標題、說明別的 surface 的一段……不算這個 surface 的鍵，照亮顯示。」

**怎麼改**：

- `helpRow` 加一個「暗」的欄位；`renderFullPopup()` 對它的鍵與說明都用 menu 變暗列的顏色（`spacemenu.go`
  `MenuPopupModel.renderFullPopup()` 的 `dimStyle`，Overlay0 `#6c7086`；`listpicker.go` 也是這個色），共用一個常數比較好。
- `menuRows()` 把 `it.disabled` 帶進去。
- panel 3 切 tab 的鍵用跟 Switch tab 同一個條件（`m.detail.TabCount() < 2`）變暗（第 3 條會把這一列拆開，兩列都要跟著這個條件）。
- namespace picker loading 時，清單階段的鍵除了 `Esc` 以外都變暗（picker 已經在、清單還沒到：對象存在、現在不能按）。
  `filterPickerRows()` 底下另有標題的「while typing」一段，說明的是打字那一階段（別的 surface：打字時 `?` 是字元，那裡看不到 key
  reference），照 v0.1.16 維持亮的，loading 也不變暗。`?` 是打開時算好的，清單在 `?` 開著時到了不會自己亮回來，重開才亮 ——
  loading 通常不到一秒，可以接受；要跟著亮就在 `NamespaceListMsg` 時重算（kbu session 決定）。
- 測試：每個條件開一次 `?`，找到那一列，斷言它是暗的；同一個 fixture 換成按得了的狀態，同一列是亮的；loading 時「while typing」
  那一段是亮的。再開 truecolor（`lipgloss.SetColorProfile(termenv.TrueColor)`，`t.Cleanup` 還原）量畫出來那一列的前景色，確認真的
  畫暗了、不只欄位設了。mutation：拿掉 `menuRows()` 帶 `disabled` 的那一行、拿掉 render 的暗色分支、panel 3 的條件改成恆假、
  namespace 的 loading 條件改成恆假、loading 時連「while typing」一起變暗，各自要紅。
- 同步：README 兩份的「A row that can't run right now is shown dimmed rather than hidden.」/「暫時不能執行的列會變暗，而不是藏起來。」
  補上 `?` 也一樣；dev-remarks「按鍵筆記」M6 那條（「暫時不能執行的列照樣列出、變暗……」）與「功能筆記」的「key reference（`?`）
  與 App Log（`!`）」補上 key reference 也變暗、另有標題說明別的 surface 的一段照亮。


## 2. YAML popup 的 `E`：helm 管理的物件在 `?` 裡不列 —— M6（已定案：`?` 列出、變暗，hint 維持不列）

**現況**：`yamlpopup.go` `YamlPopupModel.CanEdit()` 把三個條件綁成一個：`m.item.Name != ""`（看的是一個物件）、
`resourceAllowsEdit(m.resource)`（這個種類 kbu 會 edit）、`!k8s.IsHelmManaged(m.item)`（Rule A）。`keyref.go` 的
`yamlRows(canEdit)` 只在 `CanEdit()` 為真時加 `E` 列，`bottomBarStrings()` 的下框 hint 也一樣。前兩個條件是「對象不存在」，第三個是
「對象存在、現在不能按」（panel 的 Edit 列在同一個情況是變暗的），現在三種都不列。`f6_test.go` 的 `assertNoEditOffered()` 對 helm
管理的物件（`TestF6_YamlEditOffForHelmManaged`）與 Helm release 文件（`TestF6_YamlEditOffForReleaseDocs`）都斷言 hint 與 `?` 不列 `E`。

**規則**：M6 —— key reference：對象存在、現在不能按的鍵列出、變暗，對象不存在不列；下框 hint 只列現在按得了的也可以，由 app 決定。

**怎麼改**（user 已定案）：

- 判斷拆成兩個：「`E` 存在」＝ `item.Name != "" && resourceAllowsEdit(resource)`；「現在能按」＝ 存在且 `!IsHelmManaged(item)`
  （就是現在的 `CanEdit()`）。
- `?`（`yamlRows()`）：存在就列 `E`，不能按時用第 1 條的欄位變暗。`yamlRows` 的參數跟著改；`m5_test.go` 的
  `TestM5_OneModifierNotation` 呼叫 `yamlRows(true)`，一起改。
- 下框 hint（`bottomBarStrings()`）維持只在 `CanEdit()` 時列 `E`；`E` 鍵本身維持不作用。
- 測試：`assertNoEditOffered()` 拆成兩種 —— helm 管理的物件：hint 不列、`?` 有 `E` 而且是暗的；Helm release 文件與 kbu 不 edit
  的種類（例：Events）：hint 與 `?` 都不列。mutation：「存在」改回含 helm 條件（`?` 又不列）、拿掉變暗、「存在」拿掉
  `resourceAllowsEdit`（release 文件的 `?` 出現 `E`），三個各自要紅。
- 同步：dev-remarks「功能筆記」的「YAML popup（`Y`）」那條「下框 hint 與 `?` 也不列它」改成：helm 管理的物件 `?` 列出、變暗，下框
  hint 不列；kbu 不 edit 的種類與 Helm release 的文件兩邊都不列。README 沒寫 YAML 裡的 `E` 怎麼揭露，不用改。


## 3. 鍵的寫法與顏色沒照 v0.1.15 的 M5 —— M5、D1、D2、D3、D4

**規則**：M5（v0.1.15 定案）——

- 鍵名用鍵帽上的名字、大駝峰、不自創縮寫（`Esc`、`Tab`、`Enter`、`Space`、`Backspace`、`Delete`、`Home`、`End`、`PgUp`、`PgDn`；方向鍵
  `↑` `↓` `←` `→`）；字母照實際大小寫，`Ctrl` 後面的字母一律大寫；modifier 用 `-`；幾個鍵做同一件事用 `/`（`j/k`、`h/l`），範圍用
  `–`（`1–9`）。畫面上所有地方與 README 都一樣。
- 依位置：label 用括號標記（`[r]ename`、`[Alt-t]erm`）；句子（空狀態、toast、錯誤訊息）裡的鍵一律加方括號（`Press [A] or [Space]`、
  `see App Log [!]`）；hint 與 footer 寫 `鍵:說明`，冒號前後不空格、項目之間一個空格（`j/k:move Enter:run Esc:close`）；key reference
  兩欄，鍵不加括號、不加冒號。
- hint 與 footer 的鍵和說明兩個顏色。D2 的家族預設：hint、footer、key reference 的鍵 Blue `#89b4fa`；hint 與 footer 的冒號與說明
  Overlay0 `#6c7086`；key reference 的說明 Text `#cdd6f4`。D1 的 footer `Space:menu ?:help Tab/1–N:panels q:quit`、D3 的 confirm
  `Enter:<動詞> Esc:cancel`、D4 的 menu `j/k:move Enter:run Esc:close`。

**現況與怎麼改**（上一輪建議的 `j/k move · Enter run` 作廢，目標一律是 `鍵:說明`、一個空格分隔）。說明用字照現在的，要換字由 kbu
session 決定，只要形狀一樣。

### 3a. popup 下框 hint

| 檔案、函式 | 現在 | 改成 |
|---|---|---|
| `spacemenu.go` `MenuPopupModel.renderFullPopup()`（Space menu、global operation popup） | ` j/k: move  Enter: run  Esc: close ` | ` j/k:move Enter:run Esc:close `（D4 原文） |
| `listpicker.go` `ListPickerModel.renderFullPopup()`（sort 兩步） | ` j/k: move  Enter: pick  Esc: cancel ` | ` j/k:move Enter:pick Esc:cancel ` |
| `settingspopup.go` `SettingsPopupModel.renderFullPopup()` | ` j/k: move  Enter: toggle  Esc: close ` | ` j/k:move Enter:toggle Esc:close ` |
| `breadcrumb.go` `BreadcrumbPopupModel.renderFullPopup()` | ` j/k: move  Enter: switch  Esc: close ` | ` j/k:move Enter:switch Esc:close ` |
| `namespace.go` `NamespacePickerModel.renderFullPopup()`，清單 | ` Enter:toggle  /,Tab:search  Esc:close ` | ` Enter:toggle /:new filter Tab:filter Esc:close `（`/` 與 `Tab` 做的事不同 —— `/` 開新的篩選、`Tab` 回到打字、保留篩選 —— 分成兩項；`/` 本身是鍵，不能再用 `/` 跟 `Tab` 接起來） |
| 同上，打字中 | ` ↑↓ Enter:toggle  Tab:list  Esc:close ` | ` ↑/↓:move Enter:toggle Tab:list Esc:close ` |
| `context.go` `ContextPickerModel.renderFullPopup()`，清單 | ` Enter:select  /,Tab:search  Esc:close ` | ` Enter:select /:new filter Tab:filter Esc:close ` |
| 同上，打字中 | ` ↑↓ Enter:select  Tab:list  Esc:close ` | ` ↑/↓:move Enter:select Tab:list Esc:close ` |
| `help.go` `HelpModel.renderFullPopup()`（key reference 自己的下框） | ` j/k: scroll  ?/Esc: close ` | ` j/k:scroll ?/Esc:close ` |
| `applog.go` `AppLogModel.renderFullPopup()` | ` Esc:close j/k u/d y:copy D:clear ` | ` j/k:scroll u/d:page y:copy D:clear Esc:close `（`j/k`、`u/d` 現在沒有說明） |
| `comparepopup.go` `CompareYamlPopupModel.renderFrame()` | ` L: layout  j/k: scroll  Esc: close ` | ` L:layout j/k:scroll Esc:close ` |
| `yamlpopup.go` `bottomBarStrings()`，一般 | ` v:visual  y:copy  E:edit  /:search  Esc:close `（不能 edit 時少 `E:edit`） | ` v:visual y:copy E:edit /:search Esc:close ` |
| 同上，選取模式 | ` selecting  ?:keys  y:copy  v/Esc:leave ` | ` ?:keys y:copy v/Esc:leave `（`selecting` 不是鍵，放進 hint 會破壞 `鍵:說明` 的形狀；要留就移到標題，kbu session 決定） |
| 同上，窄版 | ` v  y  E  /  Esc `、` ?  y  v  Esc `（只有鍵） | 新的一般版 43 格，80 欄（L1 下限）時 YAML 下框可用 75 格，加範圍指示器（約 20 格）也放得下，窄版只在 80 欄以下才出現；留著就寫成同一個形狀，或拿掉（kbu session 決定） |
| `ptyview.go` `PtyView.renderBottomBorder()` | ` Alt-Esc:leave `、` Alt-t:hide  Alt-Esc:end `，不在 alt-screen 時再接 ` PgUp/Home:scroll ` | ` Alt-Esc:leave `、` Alt-t:hide Alt-Esc:end `，再接 ` PgUp/Home:scroll `（中間一個空格） |
| `confirm.go` `ConfirmModel.renderFullPopup()` | ` Enter <動詞> · Esc cancel ` | ` Enter:<動詞> Esc:cancel `（D3 原文，例：` Enter:delete Esc:cancel `） |

toast 的 ` auto-dismiss ` 不是鍵，不在範圍。

### 3b. panel 邊框 hint

| 檔案、函式 | 現在 | 改成 |
|---|---|---|
| `app.go` `tablePanelBottomLeft()` | `.: helm`、`esc: exit compare`（兩格空白接起來） | `.:helm`、`Esc:exit compare`（一個空格接起來） |
| `detail.go` `DetailModel.BorderBottomLeftHint()` | `enter: drill`、`enter: drill  esc: back`、`u/d: page  gg: top  G: live` | `Enter:drill`、`Enter:drill Esc:back`、`u/d:page gg:top G:live` |

鍵名小寫的 `esc` / `enter` 只出現在這兩處。窄版面 panel 內寬只有 28–38 格（dev-remarks「panel 邊框 hint 只放依 tab 而定的鍵」），新寫法
比較短（`u/d:page gg:top G:live` 22 格，原本 27 格）。

### 3c. footer（`statusline.go` `StatusLineModel.hints()`、`layoutLine()`）

| 狀態 | 現在 | 改成 |
|---|---|---|
| 一般 | ` ? help  Esc back  Space menu  Enter commit/into  Tab cycle panel  Alt-t Alterm  > settings `（鍵與說明之間一個空格，項目之間兩個） | ` ?:help Esc:back Space:menu Enter:commit/into Tab:cycle panel Alt-t:Alterm >:settings `（`renderedHints()` 用冒號接、`layoutLine()` 的分隔改一個空格，量寬的 `sepW` 跟著改）；`Tab` 那項可以照 D1 寫 `Tab/1–3:panels`，kbu session 決定 |
| 拖曳模式 | ` ? keys  j/k move  Enter drop  Esc cancel  drag mode ` | ` ?:keys j/k:move Enter:drop Esc:cancel `；最後的 `drag mode` 不是鍵（同 YAML 的 `selecting`），拿掉或改成別的揭露方式（panel 1 的 Pinned 標題已經有 `[D]rop`），kbu session 決定 |

80 欄時一般 footer 仍放不下最後一項（`>:settings`），照 D1 從尾端整組捨棄，跟現在一樣。

### 3d. key reference（`keyref.go`）

鍵欄：幾個鍵做同一件事改用 `/`，範圍用 `–`：

| 函式 | 現在的鍵欄 | 改成 |
|---|---|---|
| `menuMoveRows()`、`pickerRows()`、`filterPickerRows()`、`panelKeyRef()`、`dragKeyRef()`、App log、Compare | `j k` | `j/k` |
| `filterPickerRows()`、`yamlRows()`、`panelKeyRef()`、App log、Compare | `u d` | `u/d` |
| `filterPickerRows()`、`yamlRows()`、`panelKeyRef()`、Compare | `gg G` | `gg/G` |
| `pickerRows()`、App log | `g G` | `g/G` |
| `filterPickerRows()` 的 while typing | `↑ ↓` | `↑/↓` |
| `yamlRows()`、`yamlVisualRows()` | `h j k l`、`w b e`、`0 $` | `h/j/k/l`、`w/b/e`、`0/$` |
| `yamlVisualRows()` | `v Esc`、`q Ctrl-C` | `v/Esc`、`q/Ctrl-C` |
| `dragKeyRef()` | `Enter D`、`q Ctrl-C` | `Enter/D`、`q/Ctrl-C` |
| `panelKeyRef()` | `1 2 3` | `1–3` |
| `panelKeyRef()`（panel 3） | `h l` + 說明 `previous / next tab (also [ ])` | 兩列：`h/[` previous tab、`l/]` next tab（`[` `]` 是鍵，不再夾在說明的括號裡；變暗條件見第 1 條） |
| `panelKeyRef()` | `Tab` + 說明 `next panel (Shift-Tab: previous)` | 兩列：`Tab` next panel、`Shift-Tab` previous panel |
| `keyRef()` 的 confirm | `Enter` <動詞>、`y` same as Enter、`Esc` cancel、`n` same as Esc | 兩列：`Enter/y` <動詞>、`Esc/n` cancel |

說明欄裡提到的鍵是句子，加方括號：

| 函式 | 現在 | 改成 |
|---|---|---|
| `enterDesc()` | `shell into the container (same as S)`、`open the YAML (same as Y)`、`full-screen this panel (same as z)` | `(same as [S])`、`(same as [Y])`、`(same as [z])` |
| `yamlRows()` 的 `/` | `search; n / N next / previous match` | `search; [n]/[N] next / previous match` |
| `yamlRows()` 的 `v` | `select characters (a mode — ? there lists its keys)` | `… (a mode — [?] there lists its keys)` |

顏色已經符合（鍵 Blue、說明 Text，兩欄、鍵不加括號與冒號），見「已經符合」。

### 3e. 句子裡的鍵（toast、空狀態、提示、menu 的說明欄）

| 檔案、函式 | 現在 | 改成 |
|---|---|---|
| `app.go` `panelKey()` 拖曳中的 toast | `Esc leaves drag mode first` | `[Esc] leaves drag mode first` |
| `app.go` `Update()` YAML 選取模式的 toast | `Esc leaves the selection first` | `[Esc] leaves the selection first` |
| `app.go` `resourceFetchedForDrillMsg` 的 toast | `Drill failed — see App Log (!)` | `Drill failed — see App Log [!]`（M5 原文） |
| `relatives.go` `relativesPlaceholderEmpty` | `(no relatives to show — press Y for full YAML)` | `(no relatives to show — press [Y] for full YAML)` |
| `comparepopup.go` 截斷提示（三句） | `… (diff cap; Esc + Y on the row for full YAML)` | `… (diff cap; [Esc], then [Y] on the row, for the full YAML)` |
| `spacemenu.go` `panel2ItemOps()` 的 YAML 說明 | `view resource manifest (also Enter)` | `… (also [Enter])` |
| `menus.go` `tableMenu()` 的 container Shell、Release YAML 說明 | `kubectl exec -it into this container (also Enter)`、`the release record (also Enter)` | `(also [Enter])` |
| `menus.go` `detailMenu()` 的 Zoom、Switch tab 說明 | `full-screen this panel (also Enter)`、`next tab (h: the previous one)` | `(also [Enter])`、`next tab ([h] for the previous one)` |

menu 的說明欄是單行的句子（M5 的 label 只到名稱那一欄），所以歸在「句子」；key reference 裡由 menu 列產生的說明跟著一起變。
splash 的 `Press any key to close` 沒有指名哪個鍵，不用改。

### 3f. 顏色

| 位置 | 現在 | 改成 |
|---|---|---|
| popup 下框 hint（Space menu、global operation popup、sort、Settings、breadcrumb、namespace、context、key reference、App log、YAML、confirm） | 整串一個色：該層層色 bold（各 `renderFullPopup()` 的 `tStyle`） | 鍵 Blue、冒號與說明 Overlay0 |
| Compare 下框 hint | 整串 `#7f849c`（Overlay1） | 同上 |
| PTY 下框 hint | 整串邊框色 bold（`titleStyle`） | 同上 |
| panel 邊框 hint（`renderPanelWithScroll()` 的 `bottomLeft`） | 整串 panel 邊框色 bold（focus 時 Blue、非 focus 時 Surface2 `#585b70`） | focus：鍵 Blue、冒號與說明 Overlay0。非 focus：M5 要求鍵與說明兩個顏色，D2 的色是預設；要不要用較暗的一對跟著非 focus 的框，由 kbu session 決定 |
| footer | 鍵 Blue bold（`StatusLine.Foreground`）、說明 `#7f849c`（Overlay1） | 鍵照舊；冒號與說明改 Overlay0 |

- 做法：一個共用的 hint 繪製函式（例：吃 `[]hint{key, desc}`，`statusline.go` 已經有這個型別），鍵一個 style、`:說明` 一個 style、項目間
  一個空格；寬度用未上色的字串量（`lipgloss.Width`）。所有 popup 下框、PTY、panel 邊框、footer 都走它，hint 就只剩一種寫法與一種配色。
  bold 要不要留由 kbu session 決定（D2 只規定顏色）。F8 的 dim 會把底下各層的兩個色各自淡化，不用另外處理。

### 3g. README 兩份

| 位置 | 現在 | 改成 |
|---|---|---|
| 特色的 YAML viewer（兩份） | `` `hjkl` / `w` / `b` `` | `` `h/j/k/l`、`w/b` ``（英文版用逗號） |
| Four keys 表 `Tab` 那列（兩份） | `` `1` / `2` / `3` `` | `` `1–3` `` |
| Key Bindings 開頭（兩份） | `` `h` / `l` (or `[` / `]`) `` | `` `h/l` (or `[`/`]`) `` |
| Shortcuts 的程式碼區塊 cursor 那列（兩份） | `j k         u d         gg G` | `j/k         u/d         gg/G` |
| Mouse 表 Wheel 那列（兩份） | `` `u` / `d` `` | `` `u/d` `` |
| PTY popups 表（兩份） | `` `PgUp` / `PgDn` ``、`` `Home` / `End` `` | `` `PgUp/PgDn` ``、`` `Home/End` `` |

不用改的：modifier 都已經是 `-`（`Alt-t`、`Alt-S`、`Alt-Esc`、`Ctrl-C`、`Shift-.`）；開頭的 `` `Tab` / `Space` / `Enter` / `Esc` ``
是四個不同的鍵並列，不是同一件事；README 的句子用 Markdown 的 code 標出鍵（`` Press `Enter` to drill ``），方括號規則管的是畫面上
的句子；`cursor + Enter`、`Focus that panel + move the cursor` 的 `+` 是文句。

### 3h. 測試

斷言舊字串的要改斷言（不是刪）：

- `k5_test.go` `TestConfirm_HintNamesTheAction`（` Enter delete · Esc cancel ` 等五個）
- `detail_test.go` `TestDetailModel_BorderBottomLeftHint_RelativesDrillDepth`、`TestDetailModel_BorderBottomLeftHint_StreamingTabs`
- `comparepopup_test.go` `TestCompareYamlPopup_LSwitchesLayout`（`L: layout`）
- `k6_test.go` `TestM1_FooterShowsTheEntryKeys`（`? help`、`Space menu`、`Esc back`）；`TestK6_PopupKeyReferenceListsOnlyThePopupsKeys`
  （confirm 的 `Enter`、`y`、`Esc`、`n` 若合成 `Enter/y`、`Esc/n` 要跟著改；反向檢查的 `1 2 3` 改成 `1–3`，不然會變成永遠成立）
- `k11_test.go` `TestK11_DragFooterListsTheModesKeys`（`? keys`、`j/k move`、`Enter drop`、`Esc cancel`）
- `detail_test.go` 斷言 `no relatives to show` 的那個照樣成立；`k11_test.go` 的 `?:keys`、`f6_test.go` 的 `E:edit`、`ptyview_test.go` 的
  `Alt-t:hide` / `Alt-Esc:end` / `Alt-Esc:leave` 在新寫法下仍成立，但要確認它們真的量到新字串（例：同時斷言沒有兩個空格）。

新增、守住：`m5_test.go` 已經收了 key reference 與 PTY 下框的字串（只查 `+`）。擴充成收齊所有 popup 下框 hint（各狀態：清單 / 打字中、
YAML 一般 / 選取、PTY 三種、confirm 各動作）、panel 邊框 hint、footer 兩種、key reference 的鍵欄、上表的 toast 與空狀態，斷言：
hint / footer 沒有 `: `、兩個空格、` · `；沒有小寫的 `esc` / `enter` 鍵名；key reference 鍵欄沒有空白分隔的多鍵（`j k`）；句子裡
提到的鍵有方括號。再開 truecolor 量一個 hint 的鍵格是 Blue、說明格是 Overlay0（lipgloss 轉 24-bit 會差 1，容許 ±2）。mutation：
任一處改回舊字串、共用函式的兩個顏色對調或合成一個，都要紅。

### 3i. dev-remarks 同步（同一個 commit）

引用畫面原文的地方：「功能筆記」的 Pin（拖曳的 footer `? keys  j/k move  Enter drop  Esc cancel`）、Relatives（`enter: drill`、
`enter: drill  esc: back`）、Helm releases（`.: helm`）、YAML popup（`selecting  ?:keys  y:copy  v/Esc:leave`）；「按鍵筆記」
的 Compare 模式（`esc: exit compare`）；「設計決定」的「panel 邊框 hint 只放依 tab 而定的鍵」（`enter: drill`、`esc: back`、
`esc: exit compare`）與「PTY 裡的鍵」兩條（`Alt-t:hide  Alt-Esc:end`、`Alt-Esc:leave`、`PgUp/Home:scroll`）；「對照 tdp 時確認過的」
的 M1（`? help`、`Space menu`、`? keys`）。M5 的範圍說明見第 6 條。


## 4. namespace picker 的 loading icon 還是 braille —— D3、F7（已定案：換成 D3 的圓形切片）

**現況**（`namespace.go`）：`OpenLoading()` 打開時進入 loading，標題 ` Namespaces ` 後面那一格放 `namespaceSpinnerFrames`（十格
braille `⠋ ⠙ ⠹ …`）其中一格；`namespaceSpinnerTickMsg` 每 80ms（`namespaceSpinnerInterval`）一次，`HandleSpinnerTick()` 把
`spinnerFrame` 加一 —— 哪一格由計數決定。不 loading 時那一格是一個空白。`SetNamespaces()` 把 `loading` 設成 false 之後
`HandleSpinnerTick()` 回 nil，tick 停下。app 端在 `app.go` `Update()` 的 `case namespaceSpinnerTickMsg` 轉給它。kbu 只有這一個
loading popup（其他 popup 打開時內容都已經到了）；測試沒有碰到 `namespaceSpinner*` 或 `spinnerFrame`。

**規則**：D3 的 loading icon —— 字形 `nf-md-circle_slice_1` 到 `_8`（U+F0A9E–U+F0AA5）八格；一格 90ms、一圈 720ms；哪一格由時鐘決定
`frames[(now / 90ms) % 8]`，不是計數，tick 只在有東西 loading 時續排；一格寬，跟它取代的靜止 glyph 同寬（braille 的形狀與寬度不合，
不用）；放在 popup 標題後面時用該層的層色（bold）。F7：loading 的 popup 一律在標題後面顯示這個 icon。D6：PUA glyph 在程式碼裡寫成
code point。

**怎麼改**（user 已定案，照 filu `internal/ui/loading.go`）：

- 新增 `loading.go`：`loadingFrames`（八個 code point，`string(rune(0xf0a9e))` …）、`loadingStep = 90 * time.Millisecond`、
  `var loadingNow = time.Now`（讓測試釘時鐘）、`loadingIcon()` 回
  `loadingFrames[(loadingNow().UnixNano()/int64(loadingStep)) % int64(len(loadingFrames))]`。
- `namespace.go`：拿掉 `namespaceSpinnerFrames`、`spinnerFrame`，標題那一格改成 `loadingIcon()`；tick 間隔改 `loadingStep`；
  `HandleSpinnerTick()` 不再數格，只在 `loading` 時續排。訊息型別要不要改名（例：`loadingTickMsg`）由 kbu session 決定；拿掉的名字
  `grep` 到零筆，braille 的註解跟著拿掉。
- 標題已經用該層的層色 bold 畫（`renderFullPopup()` 的 `tStyle`），不用動；不 loading 時那一格維持一個空白，換上換下標題寬度不變。
- 順帶：fetch 回來之前關掉又重開同一個 picker，`OpenLoading()` 會再排一條 tick。計數驅動時 braille 會轉快一倍；改讀時鐘之後只是多
  重畫一次，不會轉快。要不要像 filu 用一個旗標擋掉重複的 tick，由 kbu session 決定。
- 測試：照 filu `loading_test.go` 的 `TestD3LoadingIconFollowsTheClock`（時鐘釘在 0、89ms、90ms、630ms、720ms，預期的 code point
  寫死）；namespace picker loading 時標題後面是 icon、`SetNamespaces()` 之後變回空白、兩種狀態的上框一樣寬；畫面上不再有
  U+2800–U+28FF；`SetNamespaces()` 之後 tick 不再續排。mutation：`loadingIcon()` 改成讀計數或拿掉 `% 8`、間隔改回 80ms、
  `HandleSpinnerTick()` 在 loaded 之後照樣續排，各自要紅。
- 同步：dev-remarks「對照 tdp 時確認過的」的「F7 的 loading」那條（現在寫「braille，D3 的 circle slice 是家族預設，沒換」）改寫；
  README 沒提 loading icon，不用改。


## 5. dev-remarks「偏離 tdp」中間夾著一段過時的說明 —— 文件（已定案）

**現況**（`docs/dev-remarks.md`「偏離 tdp」）：偏離有三條 —— Events tab 失焦照樣變暗（T2）、statusbar 的 context / namespace 依名稱
長度（L2）、Alterm 的 `Ctrl-t` 不揭露（K10）。Events 與 statusbar 兩條之間夾著一段舊說明：「原本這裡的兩條 K10（PTY 裡攔下捲動鍵、
Alterm 多一個出口 `Ctrl-t`）在 tdp v0.1.4 起不再是偏離……剩下的只有 `Ctrl-t` 不揭露這一點。」它寫在 statusbar 那條加進來之前，現在
讀起來像「偏離只剩一條」，也把清單從中間切開。

**規則**：D7 —— dev-remarks 的「偏離 tdp」寫「哪一條、在哪裡、為什麼」。

**怎麼改**（交給 kbu session；這份清單不動 dev-remarks 的內容）：把那段移走或改寫，讓三條偏離連成一個清單。例：整段刪掉（v0.1.4
那次改動在 CHANGELOG，兩條 K10 的現況在「設計決定」的「PTY 裡的鍵」），或縮成一句放進 `Ctrl-t` 那條的結尾（「這個鍵本身 v0.1.4 起
不算偏離，見『PTY 裡的鍵』；偏離的只是不揭露」）。`Ctrl-t` 那條的「使用者該記的出口是 `Alt-t`」可以順手寫清楚：家族的 PTY 出口鍵是
`Alt-Esc`（結束，v0.1.16 起一律先 confirm），`Alt-t` 是 Alterm 的隱藏（D5：其他 Alt 組合的出口鍵要不要 confirm 由 app 決定，kbu
不問），另一個隱藏鍵是它的別名。那條偏離裡這個鍵的寫法照 M5 是 `Ctrl-T`（`Ctrl` 後面的字母大寫；見第 6 條）。L2 與這條偏離本身照留
（已定案）。


## 6. dev-remarks 其他跟 v0.1.14–v0.1.16 對不上的文字 —— 文件

**現況與怎麼改**（交給 kbu session；跟行為修正無關的可以單獨一個 docs commit）：

- 「Popup 的分類與結構」的六類寫 **toast**（不握鍵盤）→ 照 v0.1.14 F1 的說法：除了 `Esc` 不收鍵，其他按鍵都穿過它。
- 「對照 tdp 時確認過的」的「對照時的判斷」兩條 —— zoom 不是模式、toast 算最上層（模式裡第一個 `Esc` 先收 toast）—— v0.1.14 已經
  寫進 tdp（術語「模式」、K11 的 `Tab` 列），改寫成「已經符合」並引 v0.1.14，保留 user 2026-09-29 實機確認的紀錄。「user 裁定」裡
  「關閉 PTY 一律 `Alt-Esc`、先 confirm」可以註明 v0.1.16 的 D5 把「`Alt-Esc` 一律先 confirm」寫成了家族做法。
- 「已經符合」的 M5 那條（「README 也照畫面的寫法」）與「按鍵筆記」Space menu 那段的 M5 說明（「M5：menu、footer、panel hint、popup
  hint 一套」）：改成 v0.1.15 的 M5 —— 鍵名、`/` 與 `–`、依位置的四種寫法、hint 與 footer 的兩個顏色 —— 第 3 條修完一起改。
- dev-remarks 自己的鍵寫法還沒照 M5：`Alt+Shift+S`（「功能筆記」的「列表排序」、「按鍵筆記」的「依 panel 多載的鍵」；畫面是
  `Alt-S`）；`Alt+t`（「Alterm 內部終端機」兩處、「設計決定」的「組合，不取代」兩處、「context-shift target 清掉 source」一處）；
  `Shift+Y`（「按鍵筆記」Space menu 那段）；`Alt+T` / `Alt+t` 與 `Ctrl-t`（「偏離 tdp」那條，`Ctrl` 後面的字母要大寫：`Ctrl-T`）；
  另有 `` `j`/`k` ``、`` `w`/`b`/`e` `` 這類把 `/` 放在 code 外面的寫法。M5 管的是畫面與 README，不管 dev-remarks；但同一份文件一邊說
  「modifier 全 app 一種寫法」一邊用 `+`，建議一起改（「`m5_test.go` 守著畫面上不出現 `Alt+`」那句講的是測試，不改）。
- 程式碼註解裡的 `Alt+t`、`Alt+T`、`Ctrl+C`、`Ctrl+T`、`Shift+Y` 不在 M5 範圍（M5 管畫面與 README），這份清單不列為要修；要不要順手
  改由 kbu session 決定。


## 已經符合、不用修的（對照 v0.1.14–v0.1.16 的改動）

- **F1、F8 的 toast（除了 `Esc` 不收鍵）**：toast 不在 `stackOrder()` 裡；`app.go` `Update()` 的按鍵路由在 PTY、`q` / `Ctrl-C`、`?`
  之後，toast `Owns()` 時只攔 `Esc`（`Dismiss()`），其他鍵照常交給最上層 popup 或 panel；滑鼠也打不到 toast。toast 畫在 dim 之後，
  不觸發 dim。`k4_test.go` 的三個測試守著 `Esc` 先收 toast（popup、模式、打字中），`stack_test.go` 的
  `TestStack_FadingToastHandsEscToThePanel` 守著淡出中的 toast 不再收 `Esc`。「其他鍵穿過 toast」沒有專門的測試（行為成立，v0.1.14
  只改說法），要不要補由 kbu session 決定。PTY 在最上層時 `Esc` 屬於子程序（K10），toast 等時間到。
- **K11 的 `Tab` 與 toast**：拖曳模式（`panelKey()`：`Tab`、`Shift-Tab`、`1`–`3`）與 YAML 選取模式（`Update()`）的 `Tab` 回一個
  「先 `Esc`」的 toast，模式留著；第一個 `Esc` 收 toast、第二個才離開模式（`TestK4_EscTakesTheToastBeforeAMode`）。
- **K10、D5 的出口鍵（含 v0.1.16 的一律 confirm）**：Alterm、`kubectl edit`、`kubectl exec` 三個 PTY 的出口都是 `Alt-Esc`
  （`PtyView.Update()` 認 `KeyEsc` + Alt，alt-screen 裡也攔），一律送 `ptyLeaveRequestMsg`，app 在 PTY 上疊一個 confirm（「End the
  Alterm shell?」/「Leave kubectl edit?」/「End the shell session?」）：`Enter`（或 `y`）才 `Kill()`，`Esc`（或 `n`）關掉 confirm、回到
  底下的 PTY。kbu 的 `Alt-Esc` 只有「結束」一種，沒有「只讓 focus 離開」的版本。Alterm 的 `Alt-t`（與不揭露的隱藏別名）只隱藏、shell
  留著，不問 —— D5：其他 Alt 組合的出口鍵要不要 confirm 由 app 決定。出口鍵在下框 hint 常駐；README 兩份的 PTY 表就是這樣寫。
- **術語「模式」（zoom 不是模式）**：zoom（`toggleZoom()`、`detailExpanded` / `tableExpanded`）沒有鍵換意思；`Esc` 的上一層是搜尋
  篩選 → compare 鎖 → drill，不碰 zoom，由 `z`（內容 tab 上的 `Enter`，或 `1`–`3` 換 panel）還原。README 兩份寫的是「`z` 再按一次
  還原」。
- **M5 已經照做的部分**：
  - modifier 全部是 `-`：menu 與 statusbar 的 `[Alt-t]erm`、`[Alt-S]ort panel 2 list`，key reference 的 `Alt-t`、`Alt-S`
    （`keyName()`）、`Ctrl-C`、`Shift-Tab`，footer 的 `Alt-t`，PTY 下框的 `Alt-t`、`Alt-Esc`；`m5_test.go` 的
    `TestM5_OneModifierNotation` 守著畫面上沒有 `Alt+` / `Ctrl+` / `Shift+`。畫面上出現的 `Ctrl` 組合只有 `Ctrl-C`，字母已是大寫。
  - label 的括號標記：menu 的列（`[Enter] …`、`[Esc] …` 也是）、statusbar chip（`[C]ontext`、`[N]amespace`、`[Alt-t]erm`、
    `[C]ompare`）、panel 標題的 `[1]`–`[3]`、拖曳中 Pinned 標題的 `[D]rop`。
  - key reference 是兩欄（`helpRow` 的 `key` / `desc`），鍵欄不加括號、不加冒號；鍵 Blue `#89b4fa`（`DetailLabelStyle()`，另加
    bold）、說明 Text `#cdd6f4`（`DetailValueStyle()`），正是 D2 的預設。
  - 畫面上的鍵名除了第 3b 條的 `esc` / `enter` 都是鍵帽名：`Esc`、`Enter`、`Tab`、`Space`、`PgUp`、`Home`；沒有 `Bksp`、`PgDown`。
  - statusbar 錯誤 badge 的 ` ! 3 errors ` 是驚嘆號 glyph，不是在指 `!` 這個鍵。
  - README 兩份的 modifier 都是 `-`。
- **M6 的 hint 與 footer**：YAML 下框 hint 在不能 edit 時不列 `E`、footer 列固定的幾個核心鍵 —— M6 允許 hint 與 footer 只列現在按得了
  的，由 app 決定（第 2 條只改 `?` 那半）。
- **M6 的其他 key reference**：global operation popup（`globalActions` 沒有變暗的列）、sort 兩步、Settings、breadcrumb、context
  picker、App log（`y`、`D` 沒有條件）、Compare（`L`）、confirm、拖曳模式、YAML 選取模式 —— 列出的鍵都沒有「現在不能按」的條件。
  sort 的 Reset 列在 picker 裡已經變暗，它沒有自己的熱鍵，`?` 不會列它。對象不存在就不列的已經照做（跟 menu 同一份來源）：panel 2
  空清單沒有 item operation 的鍵、沒 drill 就沒有 `[Esc] Back`、Relatives 深度 1 沒有 `B`、不能排序的種類沒有 `S` / `Alt-S`。
- **M6 v0.1.16 的「別的 surface 的一段照亮」**：kbu 唯一這樣的段落是 namespace / context picker `?` 裡的「while typing」（打字階段，
  那裡 `?` 是字元、看不到 key reference），現在是亮的；第 1 條加變暗時維持它亮。panel 的「app-wide (also in Global operation)」
  一段是這個 panel 上按得到的鍵，不算別的 surface，照一般規則。
- **loading 以外的等待文字**：panel 裡的「Waiting for logs...」、YAML 的「(no YAML — resource may still be loading)」是內容裡的文字，
  不是 F7 的 loading popup，不必用 D3 的 icon。


## 待確認

沒有。user 已定案：loading icon 換成 D3 的圓形切片（第 4 條）、YAML 的 `E` 在 `?` 列出變暗而 hint 不列（第 2 條）、dev-remarks
「偏離 tdp」那段舊說明交給 kbu session 改（第 5 條）、M5 的寫法照 v0.1.15（第 3 條）；L2 的 statusbar 偏離、`Ctrl-T` 不揭露、zoom
不是模式、`Alt-t` 隱藏不問都照留。第 3 條裡標「kbu session 決定」的（說明用字、非 focus panel 的 hint 色、`selecting` / `drag mode`
移到哪、YAML 窄版 hint、bold）是實作細節，不必問 user。
