# kbu：還沒符合 tdp 的地方

照 tdp v0.9.11（2026-10-24）。下面的條目等 terminu-go，括號裡是 kbu 的回報編號；lib 改了就照新版修、修好一條刪一條，全部修完刪掉
這個檔（tdp E7）。刻意不照做、寫明理由的在 [`dev-remarks.md`](dev-remarks.md)「偏離 tdp」（目前沒有）。

| tdp | 在哪裡 | 現況 | 該怎麼改 | 等 |
|---|---|---|---|---|
| components/input/checkbox「Select all」（清單 checkbox-26～30） | `[N]` 的 namespace popup（`internal/tui/popups.go` 的 `namespacePicker`、`pickedSelection`） | kbu 自己放了一個 `All Namespaces` 選項（tdp 說這是用錯）；lib 的 `Checks` 按 `Space` 只翻游標那一列：勾了 All Namespaces 再勾 `default`，`Enter` 還是 All，`default` 被忽略 | 拿掉 `All Namespaces` 選項與 `pickedSelection` 裡 `now[0]` 優先的判斷，全選交給 lib 的 `Select all` 列。整個 cluster 時開啟每一列都勾；每一列都勾著（或一列都沒勾）寫回時解讀成整個 cluster，之後新建的 namespace 也算 | 升 terminu-go v0.4.13（KB31） |
