# kbu：還沒符合 tdp 的地方

照 tdp v0.9.0（2026-10-20）。下面的條目等 terminu-go 給接口，括號裡是 kbu 的回報編號；lib 改了就照新版修、修好一條刪一條，全部修完刪掉
這個檔（tdp E7）。刻意不照做、寫明理由的在 [`dev-remarks.md`](dev-remarks.md)「偏離 tdp」（目前沒有）。

| tdp | 在哪裡 | 現況 | 該怎麼改 | 等 |
|---|---|---|---|---|
| components/README「控制字元」（清單 components-28） | panel 3 的 Logs（`internal/tui/detail.go` 的 `logRows`） | `\n`、`\r`、Tab、其他控制字元照規範畫；顏色碼與清畫面這類序列拿掉，沒有照著上色、也沒有畫成紅色的名字 | 照「拿來讀的內容」畫 | lib（KB24） |
