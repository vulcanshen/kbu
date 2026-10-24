# kbu：還沒符合 tdp 的地方

照 tdp v0.9.10（2026-10-24）。下面的條目等 terminu-go，括號裡是 kbu 的回報編號；lib 改了就照新版修、修好一條刪一條，全部修完刪掉
這個檔（tdp E7）。刻意不照做、寫明理由的在 [`dev-remarks.md`](dev-remarks.md)「偏離 tdp」（目前沒有）。

| tdp | 在哪裡 | 現況 | 該怎麼改 | 等 |
|---|---|---|---|---|
| components/input/checkbox（還沒有「全選」那一列的規定） | `[N]` 的 namespace popup（`internal/tui/popups.go` 的 `namespacePicker`） | lib 的 `Checks` 按 `Space` 只翻游標那一列：勾了 All Namespaces 再勾 `default`，`Enter` 還是 All，`default` 被忽略 | All Namespaces 當全選：勾它每一列跟著勾，取消任一列它跟著取消；開啟時 All 勾著，每一列都勾（user 2026-10-24 定） | tdp 規定、lib 支援（KB31） |
