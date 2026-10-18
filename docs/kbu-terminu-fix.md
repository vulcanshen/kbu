# kbu：還沒符合 tdp 的地方

照 tdp v0.7.4（checklist 對過一次，2026-10-18）。每一條都卡在 terminu-go v0.1.13 或規範本身，已回報 terminu（KB 編號）；lib 或規範
改了就照新版修、修好一條刪一條，全部修完刪掉這個檔（tdp E7）。刻意不照做、寫明理由的在 [`dev-remarks.md`](dev-remarks.md)「偏離 tdp」。

| tdp | 在哪裡 | 現況 | 該怎麼改 | 等 |
|---|---|---|---|---|
| components/input/README「貼上的控制字元」（清單 input-56） | YAML viewer 的 `/` 搜尋（`internal/tui/popups.go` 的 `openYAML`） | lib 的 text input 在 `Enter` 時擋下貼上的控制字元；搜尋的打字列應該只畫、不擋 | 換成只畫不擋的搜尋輸入 | lib（KB11） |
| T1（清單 T1-4） | YAML viewer 的 `E`（`openYAML`、`internal/tui/ops.go` 的 `confirmEdit`） | `kubectl edit` 是長時間的 session，source 該清掉；terminal 卻疊在 viewer 上，edit 結束回到的是舊的 YAML | 接受 edit 時清掉整疊 | lib（KB10） |
| F4、T1（清單 T1-1） | 兩步排序（`popups.go` 的 `openSortDirection`） | 選完方向，欄位那一步也一起關掉，要排第二欄得重開 | 只關方向那一步，回到欄位那一步 | lib（KB9） |
| F7 的 loading | namespace picker（`popups.go` 的 `openNamespaces`） | checkbox popup 不能 loading，清單抓到才打開；按 `N` 到出現之間沒有回饋 | 打開時就有 loading icon，清單到了再填 | lib（KB8） |
| L1（清單 L1-1） | panel 2 在 80 欄 | 次要欄拿掉之後，Name 仍只剩約 8 格；`z` 放大時整列放得下 | 清單的欄寬依比例讓，或欄位可以設「窄時藏起來」 | lib、規範（KB17） |
| components/layout/panel「截斷」 | panel 2 的 Name、Namespace | 從尾端截：同一個 Deployment 的 pod 截到只剩 `demo-app-67f76fb666…`，彼此分不出來（舊版從中間截，`demo-app…66-gm72n`） | 兩端都有意義的名稱可以從中間截 | 規範、lib（KB19） |
| 專案 Bubble Tea 守則「WindowSizeMsg」 | panel 2 的欄（`list.go` 的 `fitColumns`、`nameGives`） | 改視窗大小後，要等下一筆 watch 資料才重排欄：拿掉的次要欄不回來、該拿掉的還在 | 視窗大小改了就重排 | lib（KB20） |
| components/README「大寫」 | 排序 menu 的標題、drill 下去的 panel 2 膠囊、viewer 的 key reference 標題 | 名稱被轉成大寫（`SORT BY: PODS`、`[2] NGINX-… > PODS`） | 名稱照原樣 | lib、規範（KB13） |
| dialog/terminal、F7 | terminal popup（Alterm、edit、shell） | 最下面一列左右兩端露出 footer 的字 | terminal 開著時那兩格不露出 footer | lib（KB16） |
