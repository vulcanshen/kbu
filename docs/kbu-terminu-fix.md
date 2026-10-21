# kbu：還沒符合 tdp 的地方

照 tdp v0.9.2（2026-10-21）。下面的條目等 terminu-go，括號裡是 kbu 的回報編號；lib 改了就照新版修、修好一條刪一條，全部修完刪掉
這個檔（tdp E7）。刻意不照做、寫明理由的在 [`dev-remarks.md`](dev-remarks.md)「偏離 tdp」（目前沒有）。

| tdp | 在哪裡 | 現況 | 該怎麼改 | 等 |
|---|---|---|---|---|
| components/dialog/note「App Log」（清單 note-78） | MOVE ASIDE 按 `Enter` 後 lib 記的那一筆（`internal/tui/tui.go` 的 `askAside`） | terminu-go v0.4.0 記成小寫開頭的 `moved ~/… to ~/….bak` | 升到 terminu-go v0.4.3，那一筆寫成 `Moved …` | 升 lib（KB25） |
