# kbu — terminu fix

kbu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.19/principle)（tdp v0.1.19）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。有意不修的，改寫成
`dev-remarks.md`「偏離 tdp」的一條並附理由。

盤點日期：2026-09-29，依據 `main` 的 `8a625ea`（對照 v0.1.17 → v0.1.19 的改動）。第 1–3 條已修（2026-09-29，`terminu-fix` 分支：
`3b403dc` finder 篩選列灰色、`6ebb869` 模式名在右上角與 Yellow 外框、`f085fb4` 下框 hint 從尾端整組捨，併回 `main`），「已經符合」
搬進 dev-remarks「對照 tdp 時確認過的」。**剩第 4 條，等 filu 做完再照搬**（2026-09-29 user：filu 還在做）。


## 先看

- **清單先 commit，再動程式**；程式的 commit 跟清單分開，commit 只加自己改的路徑。
- **每修一處補 model test，逐處 mutation**：把修正單獨改回舊行為，確認對應的測試會紅；編譯不過不算抓到。
- **同一個 commit 同步 README 兩份與 dev-remarks** 描述該行為的段落；CHANGELOG 記在 `[Unreleased]`。
- **動手前重讀 filu 當時的 `width.go`、`iconwidth_*.go`、`d6_test.go` 與它疊 popup 的做法**（2026-09-29 看到的是 filu `e1de220`：
  `compositeDisp`、`dispCutLeft`、`centerDisp`、`padDisp`，user 說 filu 還在做，以 filu 定案後的為準）。
- **把畫面印出來看**，並在 CJK icon 字型（例：Maple Mono NF CN）下實機看一次。
- 修完拿 tdp 當時的最新版全文再對一次 D6、L4；刪掉這份清單。
- **不 push、不發版**。
- 寫進 terminu repo 的 `.local/family-fix/kbu/README.md`。
- patch 腳本與 commit message 寫成檔案再執行；patch 裡的 Nerd Font 字元寫成 escape。


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
