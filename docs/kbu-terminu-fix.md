# kbu — terminu fix

kbu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.19/principle)（tdp v0.1.19）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

> **v0.1.19（2026-09-29，這份清單寫完後才出）**：L5 —— focus 不能只靠顏色分辨（模式會換框色），家族預設雙線；K10 ——
> 子程序還沒準備好時可以不轉送一般的鍵，但 `Ctrl-C` 照樣轉送。本清單的每一條已照 v0.1.19 重新核對過。

盤點日期：2026-09-29，依據 `main` 的 `8a625ea`（工作區乾淨；上一份清單對齊到 v0.1.17，已刪）。這一輪**只對 v0.1.17 → v0.1.18
的改動**（`git -C terminu diff v0.1.17 v0.1.18 -- principle/` 與 terminu CHANGELOG 的 v0.1.18 段）：

- F1 + D3：finder 的 focus 在哪一邊要看得出來；家族預設是 `Tab` 到清單後篩選列整列灰色（Overlay0），不用 F8 的淡化。
- K11 + D2：模式在所在的框上框右側寫出模式名，外框換成模式色 Yellow。
- D2：失焦 panel 邊框上的 hint 用 Overlay0 / Surface2。
- D3：下框 hint 放不下時從尾端整組捨棄。
- D6：探測 icon 的實際寬度，所有量寬度的地方走同一個顯示寬度函式。
- K10、K9：子程序還沒準備好時 app 可以不轉送；focus 在 PTY 裡時 `q`、`Ctrl-C` 屬於子程序。

每一處都照內容拿程式碼核對過；位置寫檔案與函式，不寫行號。D2 的失焦 hint、K10 / K9 kbu 已經照做（見「已經符合」）；要修的是
第 1–4 條。


## 先看

- **清單先 commit，再動程式**；程式的 commit 跟清單分開，commit 只加自己改的路徑。
- **每修一處補 model test，逐處 mutation**：把修正單獨改回舊行為，確認對應的測試會紅；編譯不過不算抓到；預期值寫死成條文算出來的值。
  斷言舊畫面的測試要改寫（改斷言、名稱寫出新規則），不是刪掉。
- **同一個 commit 同步 README 兩份與 dev-remarks** 描述該行為的段落；CHANGELOG 記在 `[Unreleased]`（`[Unreleased]` 開頭那句
  「… (tdp v0.1.17)」改成 v0.1.19；這一輪改掉的 `40a0573`、`90f800d`、`f558835` 在 `[Unreleased]` 裡的那幾條跟著改寫，不是另加一條
  說「改回來」）。這份清單沒有動 CHANGELOG。
- **順序**：
  1. **第 1 條（finder 篩選列灰色）kbu 先做、先 commit** —— D3 這一條引的就是 kbu `40a0573`，filu、webu 等 kbu 改好才照搬。
  2. 第 2、3 條接著做（彼此獨立）。
  3. **第 4 條（icon 寬度）等 filu 做完再照搬**：filu 還要先把自己的內容列補完（它現在也還在用 `bubbletea-overlay` 疊 popup），
     kbu 照 filu 定案後的 `width.go` 與疊 popup 的做法，不要自己先發明一套。
- **把畫面印出來看**：第 1、2 條都是「看得出來」的規則，修完開 truecolor 印畫面、實機看一次（上一輪兩項都是 user 實機看了才補上的）。
- **修完拿 v0.1.19 全文再逐條對一次**（`rules-zh_TW.md`、`defaults-zh_TW.md`、`README-zh_TW.md` 的術語），至少把這一輪碰到的
  F1、K9、K10、K11、D2、D3、D6 整節讀完。
- 修完刪掉這份清單，把「已經符合」與裁定搬進 dev-remarks「對照 tdp 時確認過的」。
- **不 push、不發版**（家族全部 app 與 tdp 穩定之前不發 release）。
- 把這一輪寫進 terminu repo 的 `.local/family-fix/kbu/README.md`（加一節「第三輪」：修了什麼、commit、踩過的坑；上一輪建議寫回 tdp
  的「focus 在哪一邊」v0.1.18 已經寫進 F1 / D3）。
- patch 腳本與 commit message 寫成檔案再執行（`bash -c '…'` 裡的英文撇號會截斷外層引號）；patch 裡的 Nerd Font 字元寫成 escape
  （上一輪 Write 工具吃掉過 glyph）。


## 1. finder 的篩選列在清單有 focus 時用 F8 的淡化，不是灰色 —— F1、D3（kbu 先做）

**現況**（`search.go`，`40a0573`）：`finderSearchBox()` 在清單有 focus（`typing` 為 false）時先照 `renderSearchBox()` 畫 —— 有篩選字時
框線是 `Status.Pending`（`#f9e2af`），放大鏡 glyph（U+F0233）與篩選字沒有指定前景色 —— 再整個過一次 `dimANSI()`。所以篩選列是「各自的顏色
淡化」，不是一個灰色；user 實機看到淡化後的反白有時反而更亮。namespace picker 與 context picker（`NamespacePickerModel.renderFullPopup()`、
`ContextPickerModel.renderFullPopup()`）都走它。其餘已經照 D3：打字時篩選列亮、畫游標 `█`，清單 cursor 列是 `SidebarSelectedStyle()`
（淡的反白）；清單有 focus 時不畫游標，cursor 列是 `finderCursorStyle()` 的 popup 層色底加 `#1e1e2e` 粗體字，跟 menu 的 cursor 列一樣。
`finder_test.go` 的 `TestF1_FinderLightsThePartWithFocus` 斷言清單階段篩選列的角是 `dimRGB(Status.Pending)`。

**規則**：F1（v0.1.18）—— finder 的「focus 在哪一邊要看得出來：只有拿鍵的那一邊是亮的」。D3 家族預設 ——「`Tab` 到清單後，篩選列整列
用灰色（Overlay0，D2 的暗字）畫，不用 F8 的淡化、也不畫反白與游標，清單的 cursor 列換成 popup 層色底加深色粗體字」。

**怎麼改**：

- `finderSearchBox()` 的非打字分支不再 `dimANSI()`：框線、放大鏡 glyph、篩選字整列都用 `theme.Overlay0`（例：
  `renderSearchBoxWithColor(query, false, width, t, lipgloss.Color(theme.Overlay0))`，中間那列的字也給同一個前景色）；不畫游標、
  不畫任何背景。打字分支與 `finderCursorStyle()` 不動。
- 測試：改寫 `TestF1_FinderLightsThePartWithFocus` 的清單階段 —— 篩選列的角、glyph、篩選字每一格的前景都**正好**是 Overlay0（不是某個
  顏色的淡化）、沒有背景、沒有 `█`；打字階段維持原斷言。mutation：改回 `dimANSI()`、只把框線改灰而字不改、清單階段照樣畫游標，
  各自要紅。
- 同步：dev-remarks「功能筆記」的「多 namespace 選取（v2.1）」那條（「`Tab` 到清單後篩選列用 F8 的 `dimANSI()` 變暗」）改成灰色；
  「對照 tdp 時確認過的」裡 user 要求 focus 看得出來那一條補上 v0.1.18 已寫進 F1 / D3、改成灰色。CHANGELOG `[Unreleased]` 裡
  「the typing line dims」那句改寫。README 沒寫這個細節，不用改。


## 2. 模式沒有在上框右側寫出模式名，拖曳時框也沒換色 —— K11、D2

kbu 的模式有兩個：panel 1 的拖曳（排 pin 的順序）與 YAML popup 的選取（`v`）。

**現況**：

- **拖曳**（`sidebar.go` `SidebarModel.View()`，`90f800d`）：拖曳中 Pinned 分類標題寫成 `Pinned [D]rag mode`（左邊、panel 內容裡），被拖
  那一列的列首放拖曳 icon（U+F0A50）並塗 lavender。panel 1 的框由 `app.go` `View()` 呼叫 `renderPanelWithScroll()` 畫，`topRight` 傳
  空字串，框維持 focus 的 Blue（`Sidebar.CategoryFg`）雙線；`focusedPanelTitle("[1]", "Kinds", …)` 的膠囊也是 Blue。框沒有換色、上框
  右側沒有模式名。`k11_test.go` 的 `TestK11_DragModeShowsInPanel1` 斷言 Pinned 標題是 `Pinned [D]rag mode`。
- **選取**（`yamlpopup.go` `YamlPopupModel.renderFullPopup()`，`f558835`）：`visualMode` 時框與標題換成 `theme.Yellow`，離開換回層色
  —— 這半已經照做；但上框是 `╭─` + 標題 + 一串 `─` + `╮`，右側沒有模式名。`TestK11_YamlSelectionFrameIsYellow` 守著框與標題的
  Yellow。

**規則**：K11（v0.1.18）——「模式要標示自己：模式名一律顯示在模式所在的框（panel 或 popup）的上框右側，外框換成模式色（家族預設見
D2）；離開模式就恢復。focus 的 panel 在模式裡照樣是 focus 的線型（L5），只有顏色換掉。」D2：模式（外框與右上角的模式名）Yellow
`#f9e2af`。

**怎麼改**：

- **拖曳**：拖曳中 panel 1 的框換成 `theme.Yellow`、維持雙線；上框右側寫模式名（例：`Drag`，用字由 kbu 定），跟框同色。
  `renderPanelWithScroll()` 已經有 `topRight`（畫在右上角、用框的顏色加粗），缺的是「換框色」—— 加一個參數或一個模式名參數，讓呼叫端
  在 `m.sidebar.IsDragging()` 時傳進去。Pinned 標題的 ` [D]rag mode` 拿掉（模式名改在右上角，「一律」在那裡）；被拖那一列的 icon 與
  lavender 是「哪一列在動」的標記，不是模式名，留著。panel 膠囊（`focusedPanelTitle()` 的 Blue 底）要不要跟著換 Yellow，tdp 只說
  外框，由 kbu session 決定。拖曳結束（commit / cancel）框與膠囊回到 Blue、右上角的字消失。
- **選取**：上框右側加模式名（例：`Select`），Yellow。標題太長時的截斷（`lipgloss.Width(title) > innerW-1` 那段）要先扣掉右上角
  模式名的寬度，模式名不能被擠掉。標題的顏色 tdp 沒規定，維持現在跟框一起 Yellow 即可。
- `renderPanelWithScroll()` 的 `topRight` 放不下時會整個不畫（`titleVis+hintVis+1 > innerW`）：拿來放模式名時要確認 80 × 40 下
  panel 1 放得下（`[1] Kinds` 膠囊加 ` Drag ─`），測試釘住。
- 測試：改寫 `TestK11_DragModeShowsInPanel1` —— 拖曳中 panel 1 上框的角與線是 Yellow、仍是雙線 `╔`、右上角有模式名、Pinned 標題
  不再有模式名、被拖那一列仍有 icon；commit 與 cancel 之後框回到 Blue、模式名消失。擴充 `TestK11_YamlSelectionFrameIsYellow` ——
  上框右側有模式名、是 Yellow，離開選取後消失；再測一個超長的資源名稱，模式名仍在。mutation：拿掉 `topRight`、拖曳時框不換色、拖曳時
  換成單線、離開模式後模式名留著、YAML 標題截斷不扣模式名的寬度，各自要紅。
- 同步：dev-remarks「功能筆記」的「Pin 資源種類」（「拖曳中標題顯示 `Pinned [D]rag mode`」）與「YAML popup（`Y`）」（框與標題換成
  Yellow）、「按鍵筆記」hint 那段（「拖曳看 panel 1 標題的 `[D]rag mode`」）、「對照 tdp 時確認過的」裡 user 2026-09-29 的那兩條
  （改成 v0.1.18 的 K11：右上角寫模式名、框 Yellow）。CHANGELOG `[Unreleased]` 裡「the Pinned title reads `[D]rag mode`」那句改寫。
  README 沒寫模式怎麼顯示，不用改。


## 3. 下框 hint 放不下時沒有從尾端整組捨棄 —— D3（PTY 還牽涉 K10）

**現況**：hint 都走 `hint.go`（`popupHint()` 畫、`fitHints()` 從尾端整組捨），但只有兩處先 `fitHints()`：footer（`StatusLineModel.layoutLine()`）
與 YAML（`YamlPopupModel.bottomBarStrings()`，先縮指示器、再從尾端整組捨）。其餘：

- `popupHint()` 直接畫、不量可用寬度：Space menu 與 global operation popup（`MenuPopupModel.renderFullPopup()`）、sort 兩步
  （`ListPickerModel.renderFullPopup()`）、Settings（`SettingsPopupModel.renderFullPopup()`）、breadcrumb
  （`BreadcrumbPopupModel.renderFullPopup()`）、namespace / context picker（兩個 `renderFullPopup()`，清單與打字兩種）、key reference
  （`HelpModel.renderFullPopup()`）、App log（`AppLogModel.renderFullPopup()`，還要跟右側的指示器分寬度）、Compare
  （`CompareYamlPopupModel.renderFrame()`）、confirm（`ConfirmModel.renderFullPopup()`）。放不下時各自把 dash 數夾到 0，下框比框寬、整列
  超出（L4）。最長的是 picker 清單的 ` Enter:toggle /:new filter Tab:filter Esc:close `（48 格），popup 內寬（F7：`min(W − 2, 120) − 2`）
  在終端機約 52 欄以下就放不下；80 欄以上都放得下。
- PTY（`PtyView.renderBottomBorder()`）：放不下時**整條 hint 都不畫**（`lipgloss.Width(hint)+4 > cols` 就只畫 dash），出口鍵也一起不見；
  Alterm 的順序是 `Alt-t:hide Alt-Esc:end PgUp/Home:scroll`，就算改成從尾端捨，也會先捨掉 `Alt-Esc`、留下 `Alt-t`。
- panel 邊框 hint（`app.go` `renderPanelWithScroll()` 的 `bottomLeft`）：跟捲動指示器擠不下時整條不畫。D3 講的是 popup 的下框，panel
  邊框不在條文裡，列在這裡是因為同一個函式就能一起處理。

**規則**：D3（v0.1.18）——「下框 hint 放不下時，從尾端整組捨棄（跟 D1 的 footer 一樣），不截在項目中間。」K10：出口鍵在 focus 位於
PTY 時常駐揭露。

**怎麼改**：

- 讓 popup 的下框 hint 一律先量可用寬度再 `fitHints()`：例如 `popupHint()` 多吃一個可用寬度（框內寬扣掉兩側 dash 與指示器），在裡面
  `fitHints()`；各 popup 的呼叫端傳自己的寬度。App log 與 YAML 一樣先讓指示器、再捨 hint（或照 YAML 的順序：先縮指示器）。
- PTY：出口鍵排第一（Alterm：`Alt-Esc:end Alt-t:hide PgUp/Home:scroll`；edit / exec：`Alt-Esc:leave PgUp/Home:scroll`），再走
  `fitHints()`，從尾端捨的時候出口鍵留到最後；不再有「整條不畫」的分支。
- panel 邊框 hint 建議也走 `fitHints()`（跟捲動指示器分寬度時從尾端捨），kbu session 決定。
- 測試：在窄寬度（例：終端機 40 欄）畫每一個 popup，斷言下框整列剛好等於框寬、最後一項是完整的 `鍵:說明`、沒有半截；PTY 在窄寬度下仍
  看得到 `Alt-Esc`。mutation：拿掉任一處的 `fitHints()`、PTY 的出口鍵排回後面、PTY 恢復整條不畫，各自要紅。
- 同步：dev-remarks「按鍵筆記」hint 那段（`hint.go` 的說明）補上「下框放不下從尾端整組捨」；「設計決定」的「PTY 裡的鍵」寫出出口鍵排
  第一的理由。README 不用改。


## 4. icon 的寬度用 lipgloss 量，CJK icon 字型上框線會歪 —— D6（等 filu 做完再照搬）

**現況**：kbu 沒有探測 icon 寬度，所有寬度都用 lipgloss / x-ansi 量（Nerd Font 的 PUA 字元一律當一格）。README 兩份的需求（「preferably a
Mono variant」）與限制（「Use the Mono variant of your Nerd Font…」）、dev-remarks「Nerd Font 的渲染」是現在的應對：請使用者換字型。
`table.go` 的 helm 欄用 `MinWidth: 2` 預留一格給可能畫成兩格的 helm 標記（註解寫明），是唯一一處針對這件事的處理。

畫面上用到的 icon（PUA，非測試程式碼）：helm 標記 U+F0833（`k8s/helm.go`）、panel 膠囊的 powerline 端點 U+E0B0 / E0B1 / E0B4 / E0B6
（`app.go`、`detail.go`；filu 的 `isWideIcon()` 把這一段當一格、排除）、popup 標題與列標記（`app.go` U+F0DC / F0E2 / F160 / F161、
`applog.go` U+F03A、`breadcrumb.go` U+F43E / F0352、`comparepopup.go` U+F08AA、`confirm.go` U+F0995、`context.go` U+F0237、`detail.go`
U+F0753 / F0754、`help.go` U+F0633、`menus.go` U+F01E7、`namespace.go` U+F51E / F0131 / F0856、`ptyview.go` U+EBCA / F018D、`search.go`
U+F0233、`settingspopup.go` U+F013、`sidebar.go` U+F0A50、`spacemenu.go` U+F4DD、`statusbar.go` U+F071、`toast.go` U+F0026 / F0D45、
`yamlpopup.go` U+F48A）、loading icon U+F0A9E–U+F0AA5（`loading.go`）、splash 的 U+F0C8（`splash.go`）。

量寬度的地方（`internal/ui`，非測試；括號是那個函式裡的呼叫數，含 `lipgloss.Width`、`ansi.StringWidth`、`ansi.Truncate`、`ansiTruncate`、
`ansi.Cut`、style 的 `.Width(…)`、`padRight`、`truncateMiddle`、`truncateSidebarLabel`、`cappedField`、`fitRow`、`fitHints`、
`len([]rune(…))`、`overlay.Composite`）：

| 檔案 | 函式 |
|---|---|
| `app.go` | `View()`（8，含 `overlay.Composite` 疊 popup 與 toast）、`joinTableAndDetail()`（2）、`ansiTruncate()`（2）、`renderPanelWithScroll()`（6） |
| `animation.go` | `RenderFrame()`（1，開關動畫逐列裁切） |
| `applog.go` | `renderAllLines()`（3）、`View()`（1）、`renderFullPopup()`（5） |
| `breadcrumb.go` | `HandleMouse()`（1）、`entryDisplayLines()`（1）、`renderFullPopup()`（3）、`renderEntry()`（2） |
| `comparepopup.go` | `truncationBanner()`（1）、`renderUnifiedDiff()`（1）、`renderSplitDiff()`（3）、`centerNoDiff()`（1）、`renderFrame()`（5） |
| `confirm.go` | `renderFullPopup()`（3） |
| `context.go` | `renderFullPopup()`（6） |
| `detail.go` | `View()`（1）、`buildInfoLines()`（1）、`buildLogLines()`（3） |
| `help.go` | `renderFullPopup()`（7）、`padRight()`（2） |
| `hint.go` | `fitHints()`（2） |
| `listpicker.go` | `renderFullPopup()`（4） |
| `namespace.go` | `renderFullPopup()`（6） |
| `popup_hittest.go` | `popupContains()`（1）、`popupRowAt()`（1）—— 滑鼠命中用 popup 的顯示寬度找框 |
| `ptyview.go` | `RenderPopup()`（4）、`renderBottomBorder()`（2） |
| `relatives.go` | `renderRelativeEntries()`（3）、`renderNestedDrillEntry()`（3） |
| `search.go` | `renderSearchBoxWithColor()`（2） |
| `settingspopup.go` | `renderFullPopup()`（5） |
| `sidebar.go` | `View()`（7）、`truncateSidebarLabel()`（1） |
| `spacemenu.go` | `bracketHotkey()`（1）、`renderFullPopup()`（8） |
| `splash.go` | `Render()`（7） |
| `statusbar.go` | `ViewFull()`（3）、`cappedField()`（3）、`fitRow()`（5） |
| `statusline.go` | `layoutLine()`（1）、`ViewWithNotice()`（6） |
| `table.go` | `View()`（1）、`renderRow()`（5）、`truncateMiddle()`（4） |
| `toast.go` | `RenderPopup()`（5） |
| `yamlpopup.go` | `renderFullPopup()`（18）、`overlaySelectionOnStyledLine()`（2）、`overlayCursorOnStyledLine()`（2）、`bottomBarStrings()`（2） |

另外兩處不在 kbu 的程式碼裡、但也量寬度：`github.com/rmhubbert/bubbletea-overlay` 的 `Composite()`（用 `lipgloss.Size`、`ansi.Truncate`、
`ansi.StringWidth` 決定 popup 疊上去的位置與左右剩下的底圖）與 lipgloss style 的 `.Width()` 補空白。`dim.go` 的 `dimANSI()` 只改 SGR
顏色碼、不量寬度，不受影響。

**規則**：D6（v0.1.18）——「有些 CJK 用的 Nerd Font（例：Maple Mono NF CN）把 icon 畫成兩格，lipgloss 卻量成一格，框線就歪。app 啟動時
探測 icon 佔幾格，所有量寬度的地方（補空白、截斷、框線、疊 popup）都走同一個顯示寬度函式；L4 的畫面測試也跑一次『icon 佔兩格』。參考
實作：filu `internal/ui/width.go`（`DetectIconWidth()`、`isWideIcon()`、`dispWidth()`、`dispClip()`）。」

**怎麼改**（**等 filu 把自己的內容列補完、定案之後再照搬**；動手前重讀 filu 當時的 `width.go`、`iconwidth_*.go` 與它疊 popup 的做法）：

- 照搬 filu 的探測與寬度函式：啟動時（`cmd/main.go`，進 Bubble Tea 之前）`DetectIconWidth()`；`isWideIcon()` 的範圍照 filu（PUA 算、
  powerline 端點不算）；`dispWidth()`、`dispClip()`、補空白與截斷的函式取代上表每一處的 `lipgloss.Width` / `ansi.StringWidth` /
  `ansi.Truncate` / `ansiTruncate` / style `.Width()`。kbu 自己的 `padRight()`、`truncateMiddle()`、`truncateSidebarLabel()`、
  `cappedField()`、`fitRow()`、`fitHints()` 內部改用 `dispWidth()`。
- 疊 popup：`overlay.Composite()` 量不到兩格的 icon，照 filu 定案的做法換掉（filu 現在也還在用它）。滑鼠命中（`popup_hittest.go`）跟著用
  同一個寬度。
- kbu 有 Windows 版（`.goreleaser.yaml` 有 `windows`，filu 沒有）：探測只在 unix 做，Windows 維持一格或給手動覆寫；filu 的
  `FILU_ICON_WIDTH` 覆寫若照搬，名字照 kbu 環境變數的慣例（`KBU__…`），寫進 README 兩份的環境變數表。
- `table.go` helm 欄的 `MinWidth: 2` 在有了探測之後改成照 `dispWidth()` 算（或確認仍需要）。
- 測試：照 filu `width_test.go` 與 `iconwidth_unix_test.go`；L4 的 `TestL4_EveryRowIsTheTerminalWidth` 再跑一次 `iconCells = 2`（用
  `dispWidth()` 量每一列），涵蓋 panel、Space menu、key reference，再加 namespace picker loading（loading icon）、YAML（標題 glyph）、
  toast、PTY、helm 標記所在的 panel 2。mutation：任一處換回 `lipgloss.Width`（兩格時那一列就差一格）要紅。
- 同步：README 兩份的需求（Mono 變體）與限制（「Use the Mono variant…」）改寫成 kbu 會探測 icon 寬度；dev-remarks「Nerd Font 的渲染」
  改寫、「運作方式」加一段寬度的做法。


## 已經符合、不用修的（對照 v0.1.18 的改動）

- **L5、K10（v0.1.19）**：focus 的 panel 畫雙線 `╔═╗`、失焦圓角（`app.go` 的框線選擇），不只靠顏色；PTY 從第一格就轉送按鍵，`Ctrl-C` 一直是子程序的。
- **D2 的失焦 panel 邊框 hint**：`renderPanelWithScroll()` 的 `bottomLeft` 在 panel 沒有 focus 時用 `recededHint()`（鍵 Overlay0
  `#6c7086` 加粗、冒號與說明 Surface2 `#585b70`），focus 時用 `brightHint()`（Blue / Overlay0）—— 正是 v0.1.18 D2 新增的那一列。
  `hint_test.go` 的 `TestD2_PanelHintColoursFollowFocus` 守著兩種顏色。
- **K9：focus 在 PTY 裡時 `q`、`Ctrl-C` 屬於子程序**：`app.go` `Update()` 的按鍵路由在 `q` / `Ctrl-C` 的離開流程之前，先看最上層是不是
  PTY（`topLayer()` 是 `*PtyView` 就整個交給它）；Alterm 隱藏時 focus 不在 PTY，`q` 照常離開。`k9_test.go` 的
  `TestK9_PtyOnTopKeepsQAndCtrlC` 守著。PTY 上疊著 `Alt-Esc` 的 confirm 時，最上層是 confirm、不是 PTY，`q` / `Ctrl-C` 走離開流程，
  也對。
- **K10：子程序還沒準備好時可以不轉送**：這是「可以」，不是「必須」。kbu 的三個 PTY 都是本機的子程序（shell、`kubectl edit`、
  `kubectl exec`），`PtyView.Start()` 同步拿到 `ptmx`，按鍵從開啟動畫的第一格就轉送（`owns()` 含開啟中）；`kubectl exec` 連線中的鍵由
  kubectl 自己收著，不會丟。出口鍵（`Alt-Esc`，Alterm 另有 `Alt-t`）在 `PtyView.Update()` 裡排在轉送之前，任何時候都有效，下框 hint 常駐
  （第 3 條只調整它放不下時的捨法）。
- **F1 / D3 的 finder 其餘部分**：打字時篩選列亮、畫游標；清單 cursor 列打字時是淡的反白、清單有 focus 時是 popup 層色底加深色粗體字
  （`finderCursorStyle()`）；清單有 focus 時不畫游標。要改的只有篩選列的灰色（第 1 條）。
- **K11 / D2 的選取模式框色**：YAML 選取時框換成 Yellow、離開換回層色，已經照做（第 2 條只補右上角的模式名）。


## 待確認

沒有。第 1 條照 D3 的家族預設（user 已說淡化時反白有時更亮）；第 2 條照 K11（模式名放右上角、框 Yellow）；第 4 條的順序（等 filu）
已由 terminu 定。清單裡標「kbu session 決定」的（模式名用字、panel 膠囊要不要跟著換色、panel 邊框 hint 要不要也從尾端捨、Windows 的
寬度覆寫）是實作細節，不必問 user。
