# kbu：還沒符合 tdp 的地方

照 tdp v0.8.0（2026-10-19 照重構回報的定案更新）。下面每一條都已經定案，等 terminu-go 的下一版（第三批）給接口；括號裡是 kbu 的回報
編號（KB）或定案編號（S）。lib 發版、go.mod 升級之後照新版修，修好一條刪一條，全部修完刪掉這個檔（tdp E7）。刻意不照做、寫明
理由的在 [`dev-remarks.md`](dev-remarks.md)「偏離 tdp」（目前沒有）。

| tdp | 在哪裡 | 現況 | 該怎麼改 | 等 |
|---|---|---|---|---|
| tdp T1「用編輯器改那一項：保留」 | 從 Space menu 執行 `E`（`list.go` 的 `setListActions`） | 帶 `Ask` 的動作接受後，lib 一律清掉整疊，編輯完回不到 menu | 編輯完回到 menu | lib（S4） |
| tdp F4、tdp T1（清單 T1-1） | 兩步排序（`popups.go` 的 `openSortDirection`） | 選完方向，欄位那一步也一起關掉，要排第二欄得重開 | 只關方向那一步，回到欄位那一步 | lib（KB9、S4） |
| tdp F7 的 loading | namespace picker（`popups.go` 的 `openNamespaces`） | checkbox popup 不能 loading，清單抓到才打開；按 `N` 到出現之間沒有回饋 | 打開時就有 loading icon，清單到了再填 | lib（KB8） |
| components/input/README「貼上的控制字元」（清單 input-56） | YAML viewer 的 `/` 搜尋（`popups.go` 的 `openYAML`） | lib 的 text input 在 `Enter` 時擋下貼上的控制字元；搜尋的打字列應該只畫、不擋 | 換成只畫不擋的搜尋輸入 | lib（KB11） |
| components/color、dialog/note「搜尋」 | YAML viewer 的 `/` 搜尋 | 只捲到命中的那一行，沒有畫底線、目前那一筆也沒有 cursor 色塊；行的顏色由 lib 照 `Lang` 上，kbu 標不了範圍 | 命中的字加底線，目前那一筆畫成 cursor 色塊 | lib（KB21、S25） |
| components/layout/panel「欄放不下時」（清單 L1-1） | panel 2（`list.go` 的 `fitColumns`、`nameGives`） | kbu 自己拿掉次要欄、決定 Name 與 Namespace 誰先讓；80 欄時 Namespace 只剩三格 | 每一欄標收掉的順序，照比例縮；kbu 自己算的那份拿掉 | lib（KB17、S13） |
| components/layout/panel「截斷」 | panel 2 的 Name、Namespace | 從尾端截：同一個 Deployment 的 pod 截到只剩 `demo-app-67f76fb666…`，彼此分不出來 | 兩端都有意義的名稱從中間截（`demo-app…66-gm72n`） | lib（KB19、S32） |
| 專案 Bubble Tea 守則「WindowSizeMsg」 | panel 2 的欄 | 改視窗大小後，要等下一筆 watch 資料才重排欄 | 視窗大小改了就重排 | lib（KB20） |
| components/README「大寫」 | 排序 menu 的標題、drill 下去的 panel 2 膠囊、viewer 的 key reference 標題 | 名稱被轉成大寫（`SORT BY: PODS`、`[2] NGINX-… > PODS`） | 名稱照原樣（`[2] PODS: nginx-7d4f`） | lib（KB13、S15） |
| components/layout/panel「位置列」 | drill 下去的 panel 2 | drill 的路徑寫在 label 裡 | 路徑搬到位置列（Lavender，太長從前面截） | lib（S16） |
| tdp K3「focus 只在使用者預期會換的時候換」 | 啟動回到上次的 panel（`tui.go` 的 `start`） | lib 沒有設定 focus 的接口，`start` 送一個數字鍵代替使用者按 | 用 lib 的接口移 focus | lib（KB5、S19） |
| dialog/terminal「歷史模式」 | Alterm、edit、shell（`pty.go`） | 只有 `PgUp\|PgDn` 捲，到最舊、回到即時沒有鍵 | 歷史模式裡 `gg\|G` 到頭尾 | lib（KB7） |
| dialog/terminal「footer 不畫」 | terminal popup（Alterm、edit、shell） | 最下面一列左右兩端露出 footer 的字 | terminal 開著時 footer 不畫 | lib（KB16） |
| dialog/toast 的 Success | 複製好了、刪好了、rollback 好了（`tui.go` 的 `toastDone`）；viewer 選取模式的 `y` | 用 Info | 用 Success（Green） | lib（S8、S51） |
| dialog/note「viewer 的折行」 | split 版的 compare（`compare.go` 的 `splitDiff`） | 打開時照寬度把每一側截好；畫面之後變窄，viewer 會把它折行 | 用 viewer 的不折行選項 | lib（KB18、S18） |
| tdp L1「比最低尺寸小時」、tdp E5 的測試邊界 | 整個畫面；`tui_test.go` 的 L4 測試 | terminu-go v0.1.13 在 5 列以下開著 menu 會當掉；測試沒有最低尺寸以下的尺寸 | 只畫名稱與提示；測試加 80×5、1×1 | lib（S53） |
