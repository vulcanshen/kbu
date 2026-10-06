# kbu — terminu fix

kbu 發版前要修的 bug，逐條待修。修好一條就刪掉一條。

盤點日期：2026-10-06。依據 `main` 的 `839876f`（已對齊 tdp v0.1.23，工作區乾淨）。**這一輪不是 tdp 改版**：是 input 盤點
（2026-09-29，terminu `.local/input-survey/kbu.md`）翻出來、跟之後 tdp components 怎麼定無關的 bug；五個 app 修完就發版。
tdp 版本不變，連結不用改。這份清單由 terminu session 寫好留在工作樹，還沒 commit。


## 先看

- 每條先寫一個會紅的測試，再修；CHANGELOG `[Unreleased]` 的 Fixed 各記一條（合併或拆開由 kbu 定）。
- 第 1 條是五個 app 共通的做法，五份清單的「做法」同一段文字：畫出來的樣子與行為要一樣，程式怎麼寫各 app 自己定。
- 最後「這一輪不修」列的要等 components 的 input 檔定案，不要先動。
- 修完刪掉這份清單。不 push、不打 tag、不發版：發版是下一步，版號由 user 在 terminu session 一個一個定。把這一輪寫進
  terminu `.local/family-fix/kbu/README.md`。


## 1. 單行的值收進換行、Tab 與控制字元 —— 五個 app 共通

**現況**：五個 input（panel 1 kind 搜尋、panel 2 資源清單搜尋、namespace picker、context picker、YAML 搜尋）貼上時整段
原樣接到尾端，含換行；換行把框畫壞。

**做法**（五個 app 同一段，2026-10-06 user 定案；之後寫進 tdp components 的 input 檔）：

- 範圍：單行的值 —— 一行文字、路徑、密碼／PIN、搜尋與篩選列。多行編輯框不在這條（只有 webu 的 editor，另有一條）。
  只收數字的欄位照舊只收 `0`–`9`。
- 收進值：只看以文字進來的字元（貼上的那一段）。按下去的 `Tab`、`Enter`、`Ctrl-J` 這些鍵照舊做它們原本的事（K2、K3），
  不變成值裡的字元（locku 修的時候補的，2026-10-06）。
  - 換行（`\r\n` 算一個，單獨的 `\n`、`\r` 也各算一個）與 Tab 原樣留在值裡，不換成空白、不刪。`\r\n` 原樣存或存成
    `\n` 都可以（locku 原樣存；webu 存成 `\n`，之後一個 rune 就是一個單位，`Backspace`、遮罩、量寬都不用另外處理），
    畫、數、刪都當一個。
  - 其他控制字元（其餘的 C0、DEL、C1）丟掉。
- 預填的值（原本的名字、從檔案或別的程式讀來的值、提議）打開時就走同一個過濾：換行與 Tab 照樣畫、照樣擋，其他控制字元
  丟掉，不然 ESC 會直接送進終端機（webu 修的時候補的，2026-10-06）。
- 畫：換行畫成 `\n`、Tab 畫成 `\t`，Red `#f38ba8`，佔 2 格，跟手打的 `\`、`n`（一般值的顏色）分得開。量寬、截斷、
  捲動都把它當成一個 2 格寬、不能切開的單位。整列畫成灰色的列（沒在打的篩選列、提議）`\n`、`\t` 也跟著灰，整列一次
  上色（webu 的做法）。
- 遮罩的值：照樣遮罩，一個換行或 Tab 也是一顆遮罩符號，不露出 `\n`；使用者靠錯誤列知道。
- 刪：`Backspace` 一次刪掉整個（它本來就是一個字元）。
- 送出：值會被拿去用的 input（送出、存檔、執行、交給別的程式），值裡有換行或 Tab 時 `Enter` 不送出，錯誤列說出哪一欄
  不能有換行或 Tab（英文；句式、大小寫照該 app 現有的錯誤訊息，例：`Name can't have line breaks or tabs`）。其他照 K3：
  多欄表單 focus 跳到第一個不合格的欄位、label 變 Red；有「第一次送出後每鍵重驗」的照舊。
  - 原本送出不會失敗、所以沒預留錯誤列的 input，現在會失敗了，照 F7 打開時就預留錯誤列。
- 搜尋與篩選列（值只拿來找東西，不存、不執行；`Enter` 選的是清單裡的項目）：只照上面畫，不擋。

**為什麼**：單行的值裡換行沒有意義。原樣畫出來會把框畫壞；偷偷換成空白或刪掉，又改了使用者的值而看不出來（user：
「應該轉成 `\n` 或 `\t` 這種明確顯示」）。只在畫面上轉、值裡留原字元，是為了跟手打的 `\n` 分得開，也不會把沒有意義的
字元送出去。

**kbu 要改的地方**：
- 收字：`typedRunes()`（`search.go:95`）是五個共用的入口，在這裡丟掉其他控制字元、留下換行與 Tab。
- 畫：`renderSearchBoxWithColor()`（`search.go:58`）畫 Red `\n`／`\t`；量寬與截斷當成一個 2 格單位（跟第 3 條一起改）。
- 預填：沒有（五個框都從空的開始，或是使用者之前打的值）。
- 擋：五個都是搜尋或篩選，不擋。YAML 搜尋的 `Enter` 是標出符合的行，也算搜尋。
- 比對：值裡的換行照原字元比對（自然找不到），不另外處理。


## 2. `Backspace` 一次刪一個 byte

**現況**：五個 `handleSearchKey()`（`sidebar.go:614`、`table.go:368`、`namespace.go:226`、`context.go:140`、
`yamlpopup.go:876`）各寫一份 `s[:len(s)-1]`，`Alt-Backspace` 也一樣：CJK 一個字要按 3 次，中間兩次畫出 `�`；Nerd Font
icon（4 bytes）要按 4 次。

**怎麼改**：一次刪最後一個 rune，跟 filu、sshu、webu 一樣。五份一起改；要不要抽成一個 helper 由 kbu 定。


## 3. 值太長時用 byte 截斷

**現況**：`search.go:74-78` 用 `dispWidth()` 量寬，卻用 `text[:innerW-1] + "…"` 切 byte。CJK、icon 會被切在字元中間，畫出
`�`；切完的字比框短，`…` 後面留一段空白（盤點時打 12 個 CJK 字，送出去的是 `中文中文`、`中` 的第一個 byte、`…`，後面空
7 格）。

**怎麼改**：依顯示寬度截，不切在字元中間，補白到右框對齊；第 1 條的 `\n`、`\t` 當成一個 2 格單位。**截哪一邊照現狀
（保留開頭）**：要不要改成保留尾端、看得到正在打的字，等 components 的 input 檔定。


## 4. 註解把「CJK icon 字型」當成「icon 佔兩格」

**現況**：tdp D7（v0.1.23）不拿「一定佔兩格」的字型當例子（filu 實測：Maple Mono NF CN 的 icon 看起來兩格，游標只前進一格）。
kbu 有幾處註解把「CJK icon 字型」當成「icon 佔兩格」的代稱：`internal/ui/width.go:47`（`a CJK icon font draws wide`）、`:56`
（`single-width even on CJK icon fonts`）、`:78`（`each file-type icon eats on a CJK icon font`）、`internal/ui/iconwidth_unix_test.go:14`
（`icon consumed 2 cells (CJK font)`）。CHANGELOG `[Unreleased]` 沒有這個問題。webu、filu 都找到同一類並改了。

**怎麼改**：照 filu 整類 grep：當代稱用的改成「icon 佔兩格的字型」（佔幾格看字型與終端機，啟動時量）。不說死的寫法可以留，
例：`width.go:22`、`iconwidth_unix.go:17`、`docs/dev-remarks.md:80`（「有些字型（CJK 的『全寬 icon』字型）」）；要不要順手改由 kbu 定。


## 這一輪不修

- 值太長時截哪一邊、水平捲動。
- 游標移動與其他編輯鍵（`←`／`→`、`Home`／`End`、`Ctrl-U`、`Ctrl-W`…）。


## locku、webu、filu、sshu 先修完的經驗（2026-10-06）

- **值裡的 `\t`、`\n` 不能直接交給 lipgloss**：`Render` 會把 Tab 換成空白、在換行處斷成兩列。先換成要畫的 `\n`、`\t`，
  再上色。
- **同一個值有兩條路進來，要用同一個過濾。** locku 的設定畫面不過濾、鎖定畫面只收 `IsPrint`，兩邊各自合理，合起來就設得出
  一個解不開的 PIN。
- **測試要用跟舊行為不同的輸入。** 貼 `12\n34` 時字元數剛好等於單位數，測不出「`\r\n` 算一個」；換成 `12\r\n34` 才分得出來。
- **bracketed paste 是一整個 `KeyRunes`**（`Paste: true`，換行、`\r`、Tab、ESC 都在裡面）；打字進來的 `KeyRunes` 不會有控制
  字元，所以每個 `KeyRunes` 都過濾是安全的，不用看 `msg.Paste`。按下去的 `Tab`、`Enter`、`Ctrl-J` 是別的 `msg.Type`。
- **灰的列要整列一次上色。** 把提議、沒在打的篩選列拆成「灰字＋`\n`」好幾段，webu 原本「整列是一段 Overlay0」的測試就紅了；
  先畫成純文字，再整列上色一次。
- **預填的值不只一個入口。** webu 加書籤的標題提議每一鍵之後重算，那裡也要過濾；每一個入口都要有測試，拿掉其中一處的過濾要紅。
- **「這個框沒有錯誤列」的舊測試要換量法。** 每個框都有錯誤列之後，拿沒有錯誤列的框當基準量高度就失效了；改量「被拒時原因
  寫進框裡、高度跟打開時一樣」。
- **實機確認用 `tmux paste-buffer -p`**：送的是 bracketed paste。tmux 預設把 LF 換成 CR，正好也驗到「單獨的 `\r` 算一個換行」。
- **`[Unreleased]` 與測試註解也要 grep。** webu 的 CHANGELOG `[Unreleased]` 還寫著舊的變數名與「CJK 字型就是兩格」：已發版的段落是
  歷史不動，還沒發的段落會原樣出現在下一版的說明裡。
- **先擋再 trim（filu）。** filu 送出前 `TrimSpace`，頭尾貼進來的換行在檢查之前就被吃掉：`Icon\r` 原樣按 `Enter` 就改名成
  `Icon`。檢查要拿原值、在 trim 之前。
- **同一類的寫法整個 grep（filu）。** 「CJK 字型的 icon 就是兩格」filu 的清單只列四處，grep 出來七八處，連 dev-remarks 一整段的
  標題都是。清單列的位置只是找到的那幾個。
- **預填的入口要找全（sshu）。** 不是使用者打的字進來的地方都算：sshu 的清單列了五個，實際還有選單填回來的值與從啟動目錄來的
  預設值。kbu 這份寫「預填：沒有」是盤點時的判斷，修的時候照這個標準再確認一次。

## 待確認

沒有。
