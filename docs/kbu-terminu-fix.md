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
