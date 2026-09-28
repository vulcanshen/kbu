# kbu 開發者備忘

開發 kbu 時要提醒自己、以及與 AI 協作時記下的決策：各功能背後的設計筆記、理由與實作細節。kbu 遵循
[terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.13/principle)（tdp）；
使用者要知道的在 README，這裡只放開發者需要的。

---

## 運作方式

### 功能筆記

每個功能完整的說明，包括它為什麼這樣運作、在哪個版本落地。

- **Pin 資源種類（`P` + `D` 拖放）**：panel 1 的 sidebar 最上面長出一個 Pinned 區。任何資源列上按 `P` 切換 pin / unpin，順序寫進設定檔。Pin 是**移動**不是複製：被 pin 的種類從原本的分類消失、出現在 Pinned 底下，每個種類只有一個家。有兩個以上 pin 時，在 pinned 列按 `D` 進入模態拖放：`j`/`k` 把鎖定的種類跟鄰居交換，`Enter` 或 `D` 確定新順序，`Esc` 與其他任何鍵還原成進入時的快照。拖曳中標題顯示 `Pinned 󰩐 [D]rop`，被拖的列塗 lavender，一個 sticky toast 帶著鍵盤契約；拖曳中按 `Space` 會開出只有 drop 的精簡 menu，以防忘了契約。pin / sort / 日後的 per-kind 設定共用同一個 per-kind 設定區塊，所以暫時消失的 CRD（operator 重裝之類）會默默保留 pin 與 sort，回來的那一刻兩者都恢復
- **YAML Compare popup（`C`）**：panel 2 的列級 diff。在某列按 `C` 把它標成 **compare anchor**（anchor 存在時 statusbar 出現 `[C]ompare` chip）；在同一種類的另一列按 `C` 打開左右並排或 unified 的 YAML diff。在 anchor 本身按 `C` 取消 —— 同一個鍵切換三種狀態（標記 / diff / 取消）。`[C]ompare` chip 跟 `[C]ontext` 用反相的明暗：在 panel 2（表格把 `C` 拿去做 compare）`[C]ompare` 亮、`[C]ontext` 暗；在 panel 1 / 3 反過來，明暗的交接告訴使用者「在當前 panel 按 `C` 會觸發哪一個」。compare 模式下在 panel 2 開 menu，「Compare to anchor」排在**第一項**（使用者在候選列上開 menu 時，這就是主要意圖）；Mark / Unmark 留在列動作的位置。diff popup 有自己的動作 menu（`Space`）可即時切換版面，預設版面（Unified）存進設定。Compare 的 YAML 會先清理（拿掉 status / managedFields / resourceVersion / uid），讓 diff 只看使用者真正寫的內容
- **列表排序（sidebar 上 `S`、panel 2 上 `Alt+Shift+S`）**：per-kind 多欄排序，重啟後保留。選欄位 → 方向 → picker 回到選欄位那一步，不必重新叫出流程就能疊加更多 tier。每個 tier 在 panel 2 表頭顯示優先序與方向（`Name (1) ↑ · Restarts (2) ↓ …`）；只有一個 tier 時收成只剩箭頭，讓簡單情況保持安靜。最下面的 Reset 列一次丟掉整條 chain；方向那一步的 `Unset` 只移除單一 tier。`Esc` 是唯一出口 —— picker 在操作之間不會自己關掉。比較器看型別：`Age` / `Updated` 用底層時間戳（不是渲染出來的 "5d3h" 字串）；`Ready` 把 "N/M" 解析成一對整數；`Restarts`、`Desired`、`Current`、`Up-to-date`、`Available`、`Active`、`Rev` 用整數比較，所以 "10" 排在 "2" 之上。不認得的欄位默默跳過，stale 的設定不會弄壞排序。沒有存排序時 = `(namespace, name)` 升冪，跟 kubectl 跨 namespace 的預設一致
- **滑鼠支援**：點 panel 取得 focus 並移動 cursor，雙擊鑽入（合成 `Enter`），右鍵開該列的 context menu（合成 `Space`），滾輪半頁捲動（合成 `u` / `d`）。13 個 popup 也有回應：list popup 左鍵即 commit，viewer popup（YAML / Compare / App Log / Help）保留滾輪捲動，confirm 對話框刻意讓左鍵不作用，免得誤點觸發破壞性的 delete / edit / rollback。滑鼠可在 Settings popup（`>`）關掉，`scroll_direction: natural | reverse` 給偏好反向的使用者翻轉滾輪
- **Settings popup（`>`）**：app 層級的 surface，標題有齒輪 glyph。目前有 Mouse on/off + Scroll Direction；日後的全域設定都放這裡。popup 自己就是逃生口：即使 Mouse 關了，popup 內仍可點擊，讓使用者能把滑鼠開回來
- **依層數決定的 popup 邊框（v1.7.4）**：每個 popup 依巢狀深度從 `lavender → sapphire` 漸層取邊框色：layer 1（任何第一層 popup）用 lavenphire25，layer 2（疊在另一個 popup 上，例如 Compare 裡的 Diff menu、Breadcrumb 裡的 Confirm）用 lavenphire50，layer 3 lavenphire75，layer 4+ sapphire。視覺上的堆疊永遠讀成「這個疊在底下那個之上」，不需要想。Toast 警告保留 Catppuccin Peach 邊框當專用的警告訊號；PTY popup（Alterm、`kubectl edit`、`kubectl exec`）永遠是 layer 1 —— 它們是 context-shift target，取代 popup tree 而不是疊在上面（見「設計決定」的 context-shift），所以不論從哪條 menu 鏈叫出來都同一個邊框色。原地換內容的 picker（例如 sort 的欄位 → 方向）保留原本的 layer —— picker 是同一個 instance，只是換了內容
- **28 種內建資源 + CRD 支援**：啟動時動態探索 Custom Resource，分在 KubeConfig / Cluster / Workloads / Network / Config / Storage / RBAC / Autoscaling / Helm 幾個分類。Helm 分類只在 `helm` CLI 位於 `PATH` 時註冊
- **Watch 即時更新**：資源透過 Kubernetes Watch API 自動刷新
- **Vim 風格導覽**：`j`/`k`、`u`/`d` 翻頁、`gg`/`G`、`/` 搜尋
- **lazygit 風格的三 panel 版面**：編號的 sidebar、清單、詳細資料 panel，附捲動指示
- **Drill-down 導覽**：Deployment / DaemonSet / StatefulSet / Job → Pods → Containers；CronJob → Jobs；HPA → 目標 workload；PVC → 掛載它的 Pods；PDB → 受保護的 Pods；Helm Release → chart 部署的每個原生 K8s 物件
- **Relatives tab —— Lens 風格導覽**：每個 detail panel（Namespaces 除外）列出該資源可導覽的參照（owner、選到的 pod、scaleTargetRef、掛載它的 pod……）。`Enter` 鑽進 cursor 所在的參照 —— panel 重畫成*那個*資源的 Relatives，形成一條鏈（Deployment → Pod → ConfigMap → 使用它的 Pods……）。`Esc` 退一層。panel 3 下框左側顯示依 tab 而定的 hint（depth 1 是 `enter: drill`，鑽進去之後是 `enter: drill  esc: back`）。`Space` 開 breadcrumb popup，可以把 panel 1+2 跳回鏈上任何祖先（先確認）。depth>1 時 tab 標籤顯示 `Relatives N`。`Y` 打開 cursor 所在那一筆的 YAML。循環偵測擋住回到祖先；fetch 失敗 toast 後留在原地。27 種資源涵蓋 26 種 —— ConfigMaps / Secrets / ServiceAccounts 顯示*反向*參照（哪些 Pod 用我、哪些 RoleBinding 以這個 SA 為 subject……）；Helm release 顯示 `Deployed Resources`，chart 部署的每個 K8s 物件都一步可達
- **Helm releases（`helm` 在 `PATH` 上時）**：專屬的 `Helm > Releases` sidebar 分類列出叢集裡每個 release（每 3 秒 poll 一次 `helm list -A`；Helm 沒有 watch API）。panel 2 欄位：`NAME / NAMESPACE / CHART / APP VER / REV / STATUS / UPDATED`。在 release 列按 `Space` 開文件 menu（Manifest / Creator Notes / User Values / Merged Values / Hooks）；`Enter` 選一個，以 `helm get ...` 取回，在 YAML popup 顯示。menu 留在 YAML 後面，連續看幾份文件不必重開。panel 3 以 `History` tab 取代 Events —— 每個 revision 的表格（REV / STATUS / DATE / CHART / DESCRIPTION），目前部署的 rev 標 `●`。在非目前的列按 `Space` 詢問要不要 rollback；confirm 顯示確切的 `helm rollback` 指令、非同步執行，結果以 toast 顯示。helm 管理的 K8s 物件（label `app.kubernetes.io/managed-by: Helm` 或 annotation `meta.helm.sh/release-name`）在 panel 2 標 `` glyph，擋掉 `E`（kubectl edit）並 toast「Helm-managed (read-only)」—— 請改用 `helm upgrade` / `rollback`。在任何非 Releases 清單按 `.` 隱藏所有 helm 管理的物件（panel 2 下框左側永遠顯示 `.: helm` hint）
- **YAML popup（`Y`）**：選取資源的原始 `kubectl get -o yaml`，全螢幕 overlay，以 **vim 風格 buffer** 運作：`hjkl` 移動 cursor（自動捲動保持可見）、`w`/`b`/`e` 單字移動、`0`/`$` 行首 / 行尾、`gg`/`G` buffer 頭尾、`u`/`d` 半頁。按 `v` 進入字元 visual 模式（anchor 在 cursor，`hjkl` 延伸選取）；visual 模式下 `y` 複製選取的子字串，visual 模式外 `y` 複製整份 YAML。`/` 搜尋，`n`/`N` 逐一跳（整列高亮，cursor 跳到每個符合處），`E` 直接叫出 `kubectl edit`，`Esc` 先退出 visual、再關閉。YAML 放在 popup 不放在 detail panel，直式版面就不會把長 YAML 行折得很難看
- **Pod log 串流與自動追尾**：多 container 支援，格式 `<container>|<log>`；Logs tab 預設黏在尾端。Logs tab 標籤帶一個 Nerd Font glyph 顯示追尾狀態 —— 自動追尾時 `▶`（live，U+F0753），使用者往上捲之後 `⏸`（paused，U+F0754）。不論 tab 是否 active，glyph 都留在 tab 上，切 tab 時 panel 3 的 tab 列寬度不變。往上捲（`k`/`↑`/`u`/`gg`）暫停、讀歷史；按 `G` 追上並恢復追尾。panel 3 下框左側顯示 `u/d: page  gg: top  G: live` 當作隨手的小抄
- **所有 workload 種類的彙總 log**：選到 workload 列時，它管理的**每個 Pod** 的 log 串進同一個 Logs tab。每行加上前綴 `<pod-hash>│<container>│<text>`，每段有自己穩定的顏色，rollout 時不必 drill-down 就一眼看出哪個 pod 在噴錯。涵蓋 Deployment（目前的 ReplicaSet，RBAC 不足時退回用 selector）、StatefulSet、DaemonSet、Job、ReplicaSet、CronJob（跨所有保留的 Job）。Pod 汰換：串流在選列當下取快照；重新選列才會刷新
- **workload 種類的子 events 彙總**：workload 列的 Events tab 合併 workload 本身與它的子 Pods 的 events，最新的在前。Object 欄（「`Pod/web-abc-xyz`」對「`Deployment/web`」）寫出每筆 event 的來源，整條鏈就地可見。CronJob 是三層：CronJob 自己的 events + 它擁有的每個 Job 的 events + 每個 Pod 的 events，所以「昨晚的 cron 為什麼失敗」在一個 tab 讀完，不必 `kubectl describe` × N
- **Events tab 追最新（v1.7.10）**：tab 標題有跟 Logs tab 一樣的 live（▶）/ paused（⏸）glyph；`G` 重新接上尾端，`k` / 往上捲暫停並凍結目前畫面、不受進來的 watcher tick 影響。下框左側寫 `u/d: page  gg: top  G: live`。理由：忙碌 workload 的彙總 events 是一陣一陣來的 —— 暫停讓使用者讀一份快照，不會被新 events 推出畫面
- **Conditions tab**：detail panel 的新 tab，以 `TYPE / STATUS / REASON / MESSAGE / AGE` 表格呈現 `.status.conditions`（同 `kubectl describe` 的 Conditions 段）。Status `False` 的列標紅。只在會填 conditions 的種類出現（Pod / Node / PVC / Deployment / StatefulSet / DaemonSet / Job / HPA / Ingress）；沒有的種類隱藏（ConfigMap、Secret、Service 等）。events 過了 TTL 之後特別關鍵 —— conditions 反映*目前*狀態，events 反映*最近*狀態
- **Status 欄上色 —— 只標異常**：panel 2 的 Status 欄（每個有 Status 的種類 —— Pod / Node / Namespace / PVC / PV / Helm Release）加上 Events 的 Type 欄，**只**替異常值上色。黃色是過渡 / 降級（Pending / Terminating / SchedulingDisabled / Released / pending-* / Init:*），紅色是失敗（Failed / Error / CrashLoopBackOff / ImagePullBackOff / NotReady / Lost / Warning）。健康值（Running / Bound / Active / Deployed / Normal）維持列的基本前景色。顏色是訊號，不是裝飾 —— 眼睛只會被需要注意的列吸過去。cursor / lock 列改用較深的 Catppuccin Latte 版本，粉彩色才不會在反白底上被洗掉
- **切換列的 debounce（300ms）**：panel 2 狂按 j/k 以前會對每一列都發一次 detail fetch + 一次 log 串流 Start，連 cursor 一掠而過的列也是。現在 dispatch 有 debounce：每次切列把序號加一，排 300ms 後才做 fetch / 串流 Start，快速捲過 49 列只發一次 fetch（停下的那一列），而不是 49 次。lie-as-lock 不變式維持：便宜的狀態變動（停掉前一個串流、清掉 retry throttle）當場做，所以 panel 2 仍然感覺即時；只有昂貴的工作延後。跟既有 sidebar `switchSeq` 的 debounce 視窗一致，肌肉記憶相同
- **用內嵌 PTY 做編輯與 shell exec**：`E` 跑 `kubectl edit`、`S` 跑 `kubectl exec -it -- /bin/sh`，都在 app 內的虛擬終端機裡，editor 與 shell session 永遠不碰宿主終端機的 scrollback。editor 依 `$KUBE_EDITOR` / `$EDITOR`（或 `config.yaml` 的 `editor`）
- **Alterm 內部終端機**：`Alt+t` 切換 kbu 內嵌的 shell（完整 env / cwd 的 login shell）—— 像在 popup 裡 `ssh localhost`。跑 `kubectl apply -f`、`helm`，任何平常要跳出 kbu 才能做的事。shell 是**常駐的**：popup 可見時按 `Alt+t` 隱藏它但不殺 shell；再按一次重新接上（cwd、history、env、背景 job 都保留）。shell 在背景活著時，statusbar 右側顯示 `[Alt-t]erm` chip —— 跟 `[C]ontext` / `[N]amespace` 同樣的方括號熱鍵格式（statusbar 一條規則：括起來的就是熱鍵）。跟 `kubectl edit` / `kubectl exec` 互相獨立 —— 可以讓 Alterm 跑著，同時在另一個 popup 裡編輯資源或 exec 進 container
- **PTY popup 永遠是 layer 1**：Alterm、`kubectl edit`、`kubectl exec` 都用 layer 1 的邊框色（lavenphire25）。它們是 context-shift target，**取代** popup tree 而不是疊上去（entry handler 會關掉底下每個 blocking popup），所以 layer 的意思是「畫面上唯一的 popup」。所有 PTY surface 同一個邊框色，不論從哪條 menu 鏈叫出來。標題（`Alterm: hostname` 對 `Edit: pod/foo` 對 `Shell: pod/foo → ctnr`）負責區分種類
- **PTY scrollback**：所有 PTY popup（Alterm、shell exec、edit）有 10k 行歷史。`PgUp` / `PgDn` 翻頁，`Home` / `End` 跳到頂 / 回到 live。alt-screen 的程式（vim、less、htop）停用，讓它們保有自己的翻頁
- **per-container 上色的 log 標籤**：多 container 的 pod 可以逐行分辨；每個 container 名稱有穩定的顏色
- **刪除資源**：`D`（大寫，熱鍵與 `Space` menu 都是）附確認對話框。刪 namespace 帶更強的警告（`!!! Delete namespace "X"? This will remove ALL resources inside it.`），因為它會連鎖刪掉 namespace 裡每個 workload，是 kbu 開放的最危險的刪除。Events / Nodes 直接擋掉 —— Events 是系統產生的不可變紀錄，Nodes 是管理員的基礎設施動作，不在 kbu 這種偵察工具的受眾範圍
- **搜尋 / 過濾**：`/` 在 sidebar 與表格 panel、以及 namespace / context picker popup 裡搜尋。sidebar 搜尋也比對分類名（例如 "cluster" 會展開 Cluster 分類）。focus 移到別的 panel 時搜尋自動清掉 —— 選取保留，過濾不保留
- **複製到剪貼簿（`y`）**：透過 OSC 52 複製 focus 元素的內容（tmux / SSH 都通，不需要 `xclip` / `pbcopy`）。語意跟著 focus：focus 目標有 cursor 時（sidebar 種類、panel 2 的列、panel 3 Relatives / History 的列、visual 模式下的 YAML popup），`y` 只複製那一列 / 選取 —— cursor 列是 tab 分隔的原始值，可直接餵 `awk` / `cut`，YAML visual 選取是原樣子字串。沒有 cursor 時（panel 3 Logs / Events / Conditions、App Log popup、非 visual 模式的 YAML popup），`y` 複製整個 focus 內容
- **分級的 toast 通知**：info 級（1 秒、popup layer 邊框 + `󰵅 kbu` 標題、hint 寫 `auto-dismiss`）用於「Copied!」之類的確認；warning 級（2 秒、Catppuccin Peach + `󰀦 kbu` 標題）用於被擋下的動作，例如 Relatives 循環偵測或 drill 失敗；sticky 版用於模態狀態的契約（拖曳模式提示）—— hint 換成 `Esc: close`，沒有自動消失的 timer
- **多 namespace 選取（v2.1）**：`N` 打開勾選框 picker。`Enter` 切換 cursor 所在的 namespace 並立即套用（panel 2 馬上重抓）；popup 保持開著，可以連續勾好幾個，勾選的 namespace 顯示綠色。`j`/`k` 移動（`u`/`d` 翻頁、`gg`/`G` 跳），`/` 搜尋，`Esc`/`Space` 關閉。「All Namespaces」與個別 namespace 互斥 —— 勾任一個個別的會清掉 All，取消最後一個會回到 All。個別選取時每個 namespace 分開 list（client-go 的 namespaced List，依名稱順序，每個回來就漸進渲染）；「All Namespaces」維持單一的全叢集 list —— 不做先全抓再過濾。namespaced 資源的清單多一個開頭的 Namespace 欄（第一欄，在 Name 之前；跟 Name 一樣中間截斷），statusbar 顯示單一 namespace、「All Namespaces」或「N selected」。選取重啟後保留，啟動時跟仍存在的 namespace 對帳 —— 刪掉的丟棄，全部都不在就退回 All。`C` 切換 context（大寫 —— 觸發鍵用大寫，避免打搜尋字時誤觸）
- **KubeConfig ▸ Contexts（v2.2）**：唯讀檢視 kubeconfig 的 contexts，放在 sidebar 最前面的新分類「KubeConfig」。panel 2 列出 Name / Cluster / User / Namespace / Server，目前連線的 context 尾端標 `*`；panel 3 的 Info tab 加上 TLS 與 Auth 摘要，`Y` 打開 kubeconfig 形狀的 YAML 檢視。直接讀 kubeconfig（叢集連不上也能用）。嚴格唯讀 —— 不能編輯 / 刪除，Enter 不作用，切換 context 仍走 `C` picker。憑證永遠不顯示：只露出非機密的 metadata，YAML 由 allowlist 組出，token / password / key 一律顯示 `<redacted>`
- **Session-local context**：在 kbu 裡切 context 不碰 `~/.kube/config`。另一個終端機同時跑 `kubectl` 互不干擾
- **Session 狀態保存（v1.7.10）**：關掉再開會回到原處。每次離開時 kbu 把目前的 `(context, namespace, kind, panel 2 列 cursor, focus panel, panel 3 active tab)` 記進設定目錄裡、跟 `config.yaml` 放在一起的 `state.yaml`。下次啟動在 k8s client 連線前先套用記下的 context + namespace，把 sidebar cursor 還原到記下的 Kind，等 watcher tick 到了再把 panel 2 cursor 對到上次選的物件。focus panel（sidebar / table / detail）也還原，所以從 panel 2 按 `q` 離開，回來還在 panel 2。panel 3 的 active tab（v1.7.11）以 tab 名稱經 `SwitchToTabByName` 還原，新選的 Kind 沒有那個 tab 時默默退回該種類的預設。記下的值不存在了（namespace 被刪、CRD 被移除、物件汰換掉）就退回預設，並在 App Log 記一行 INFO —— 不 toast、不警告；在意的話 `!` 看得到。`state.yaml` 刻意跟 `config.yaml` 分開，理由見「設計決定」
- **依 panel 區分的選取樣式**：focus panel 的 cursor 列是亮的 lavender chip；*非 focus* panel 的選取列保留較柔的底色 + 粗體，所以在別的 panel 工作時，永遠看得到每個 panel「記得」哪個資源。非 focus 的 panel 暗成柔和的 overlay 灰，眼睛落在 focus panel 上，又不會失去另外兩個的「你在這裡」記憶
- **Detail tab**：panel 3 的 tab 清單依種類而定。workload 種類（Pods / Deployments / StatefulSets / DaemonSets / Jobs / CronJobs）以 `Logs` 開頭，因為切列最常是「這東西現在在幹嘛」的動作 —— Relatives 是刻意的 drill 動作，多切一次 tab 可以接受。順序是 `Logs` / `Relatives` / `Events` / `Conditions`（Conditions 只給會填 `.status.conditions` 的種類）。非 workload 種類以 `Relatives` 開頭，讓從 Relatives 那一筆 `Space` 跳過去時落在使用者原本的 tab。Helm release 是 `Relatives` / `History`。panel 3 沒有 `/` 搜尋 —— 有 cursor 的 tab（Relatives / History）不容許過濾列，Logs 當成單純的追尾檢視比較好讀；大量內容用 `Y` + 自己的 editor 去 grep
- **長值折行，永不截斷**：適用於 YAML、Events 與 Logs；panel 改變大小時重新折行
- **Panel 放大**：`z` 讓 focus 所在的 Table 或 Detail panel 全螢幕；再按 `z` 回到三 panel 版面
- **主題系統**：在設定目錄放一份 `theme.yaml` 覆寫顏色
- **Help 與 App Log overlay**：`?` / `!` 疊在主畫面上的 popup
- **錯誤通知**：statusbar badge + status line 訊息
- **Crash log**：panic 寫進 kbu 的 log 目錄
- **Audit log**：每次 `kubectl edit` 與 `kubectl delete` 記進 `audit-*.log`

### 按鍵筆記

**滑鼠與 confirm 對話框。** menu 類 popup（panel 2 menu、sort picker、namespace / context picker、breadcrumb、helm 文件 menu、hint、settings、confirm）不理滾輪 —— 內容短，半頁的語意不適用。viewer popup（YAML / Compare / App Log / Help）**會**隨滾輪捲動。confirm 對話框刻意讓左鍵不作用，免得誤點觸發破壞性的 delete / edit / rollback —— 只能用鍵盤 `Enter` / `y` 確認。

**依 panel 多載的鍵。** `S` / `C` / `D` 依 panel 多載 —— 同一個字母，依哪個 panel 有 focus 做不同的事，就像 `P` 只在 panel 1 有意義。panel 2 的排序要用 `Alt+Shift+S` 組合鍵，因為單獨的 `S` 已經是 Shell —— modifier 在不破壞它的前提下切出 panel 2 的排序手勢。觸發鍵刻意用大寫，避免在 `/` 搜尋欄打字時誤觸。

**panel 2 的 context menu。** 每列一個 menu，項目依資源而定 —— `Y` YAML / `E` Edit / `S` Shell / `D` Delete，加上依情境的 **`C` Compare** 項目（依狀態是 Mark anchor / Compare to anchor / Unmark anchor）。compare 模式下 cursor 在可比較的列時，「Compare to anchor」浮到 item 區的最上面 —— 使用者在候選列上開 menu 時，這就是主要意圖。下面一條分隔線切出 **panel operation** 區：`[Alt-S]ort panel 2 list` 打開跟 panel 1 Space menu 同一個欄位 picker，範圍是目前檢視的種類。用 `j`/`k` + `Enter` 或直接按字母。Mark / Unmark Compare anchor 在 commit 時關掉 menu（純狀態變動 —— menu 的事做完了，開著反而擋住 panel 2 上 anchor 列的 lavender 高亮）；Compare-to-anchor 讓 menu 保持開著（target 的 diff popup 疊上來，`Esc` 回到 menu，tdp F4）。helm 管理的列隱藏 `E`/`D`（Rule A：唯讀 —— 編輯會被 `helm upgrade`/`rollback` 蓋掉）；沒有 container 的資源隱藏 `S`。

為了導覽的可發現性，另外附加兩個只能用 cursor 選的項目（沒有單字母熱鍵，用 `j`/`k` + `Enter`）：

- `Enter ↘` —— 鑽進該列的子項（`pods` / `jobs` 等，依種類）。跟直接在列上按 `Enter` 一樣。
- `Esc ↖` —— 回到上一層清單。只在已經在種類層級的 drill 鏈裡時出現（例如正在看某個 Deployment 的 Pods）。跟直接按 `Esc` 一樣。

**Container drill 的例外：** cursor 在 container 列時（比 Pod 的 pods 清單再深一層），Space menu 收成只有 `Shell` —— 拿掉 Esc 項目，因為 Esc 是通用的「退一層」手勢，一列的 menu 不需要多餘的「返回」。

**Compare 模式。** anchor 設著時，panel 2 下框左側顯示 `esc: exit compare` hint。鎖定的列以 lavender（Mocha）粗體反白底色繪製，跟 Pinned 項目與 Settings 的 ON 開關同一個強調色 ——「這一列上有使用者設定的狀態」。statusbar 上固定寬度的 `<icon> Compare` chip 確認模式開著，不為資源名稱佔可變寬度的格子（真的比對時 popup 本身顯示 `left vs right`）。focus 離開 panel 2、或 anchor 列從 watcher 串流消失（被刪 / 被 namespace 過濾掉）時，compare 鎖自動解除。

### Nerd Font 的渲染

終端機用 **Nerd Font 的 Mono 變體**（例如 JetBrains Mono Nerd Font Mono、FiraCode Nerd Font Mono）。kbu 的 popup 標題與列標記用 Material Design icon 區段的 Nerd Font glyph；Mono 變體設計成每個 glyph 剛好畫 1 格，欄位與框線對齊才穩定。比例寬度的（非 Mono）變體、以及設成 East-Asian-Ambiguous=double 的終端機（部分 tmux + iTerm2 的 CJK 設定）可能把這些 glyph 畫成 2 格 —— kbu 仍然能用，但 helm-managed 的列與 popup 上框可能偏 1 格。看到偏移就換 Mono 變體，或把 ambiguous-width 設成 single。

### Popup 的分類與結構

- **一個 popup 一個檔、一個 `PopupAnimator`**，animator 的 `Target` 對應檔名（階層用 `_` 分隔，例如 `comparepopup_menu`、`ptyview_shell`）。top-level popup 在 `app.go View()` 裡 composite；popup 裡的子 popup（`comparepopup` → `comparemenu`）也有自己的檔與 animator，由 parent 在 `Open()` 時 reset、轉送 tick 與按鍵。`app.go View()` 以外出現裸的 `overlay.Composite` 是警訊。緣由：v1.7.4 發現 compare 裡的 menu 同時漏掉動畫與上下留白 —— 它是唯一住在別人檔案裡的 popup，以檔名做的盤點看不到它。
- **四類，各有固定版面**（tdp F1）：**menu**（`j/k` + `Enter` 從清單挑；上下留白一列）、**message**（短文 + 動作，例：applog、help、confirm、toast；上下留白一列）、**viewport**（長內容、垂直最大化、無留白，例：yamlpopup、comparepopup）、**pty**（框由 kbu 畫、內容由 subprocess 畫，例：Alterm、kubectl edit / exec）。標題一律 glyph + 文字；toast 的文字固定是 `kbu`，等級靠 glyph（`󰵅` info、`󰀦` warn）與邊框色區分。
- **動畫**（tdp F2）：開 / 關約 160ms，原地換內容約 120ms。
- **疊層只有一份順序**（tdp D3、F4、X2）：`stack.go` 的 `stackOrder()` 由下往上列出每個 popup。`View()` 照它畫；`Update()` 把按鍵與滑鼠交給最後一個 `owns()` 的那一層（`topLayer()`）；`popupDepth()` 照它數層、`closeAllBlockingPopups()` 照它關。順序依「誰開誰」：Space menu 在最底，從它開出的 picker、viewer、confirm 在上，key reference 在它們之上，PTY 最上（context-shift 會先清掉底下，見「設計決定」）。滾輪看的也是最上層：menu 類吞掉、viewer 轉成 `u` / `d`、PTY 不理。新 popup 只在 `stackOrder()` 插一次，路由、繪製、層數、滑鼠自動一致。
- **`Esc` 與關閉中的 popup**（tdp F3）：判斷一層「還在不在」用 `owns()`（開啟中或已開），不用含關閉中的 `IsActive()`；`IsActive()` 只決定還要不要畫。正在跑關閉動畫的 popup 不再接鍵，下一鍵交給底下那一層。blocking popup 在自己的 `Update` 攔 `Esc`；toast 不是 blocking（按鍵穿透到底下的 panel），由 app 層的 `Esc` handler 在它 `Owns()` 時 dismiss，淡出中的 toast 不再吃 `Esc`。
- **邊框色依層數**：`theme.PopupLayerColor(layer)` 是唯一來源，不寫死 hex。每個 popup 有 `layer` 與 `borderColor`，`SetLayer` 同時更新 animator 的顏色；開啟前以 `popupDepth() + 1` 蓋章，子 popup 用 `parent.layer + 1`；原地換內容的 picker 保留原 layer（已開著時不再蓋章，否則會重複計算自己）。
- 完整的 popup 規格（盤點清單、`padRow` 寫法、新 popup 的分類決策樹）在本機的 `.claude/rules/popup-convention.md`（不進版控）。

### Panel 外框

- **標題膠囊**：panel 1 / 2 的上框左端是一顆 powerline 膠囊 `<E0B6>[N] body<E0B4>`，`[N]` 是 panel 編號、也是跳到該 panel 的數字鍵；膠囊是 base 字 + 邊框色底 + 粗體。panel 3 只畫 `<E0B6>[3]`，收尾交給緊接的 tab 列。
- **panel 3 的 tab 列**：starship 風格的膠囊鏈，只有 active tab 是亮膠囊，其他坐在 crust 底；第一個 tab active 時跟 `[3]` 膠囊合併；tab 之間 active↔inactive 用 `E0B0`，inactive↔inactive 用 `E0B1` 細 chevron；尾端用 `E0B4` 收圓。tab 標籤一律同寬，`Logs` / `Events` 的 live / paused glyph 不論 active 與否都畫，切 tab 時 tab 列不伸縮（tdp L2）。
- **邊框 hint**：上框右端 ` <hint>─`、下框左端 `─<hint>─`、下框右端 ` X of Y `。hint 與框同色系。放不下就整段靜默丟掉、不截斷；下框空間不夠時先丟左側 hint、保留捲動指示。
- **focus 二態**（tdp L5）：focus 是雙線 `╔═╗` + Blue `#89b4fa`，非 focus 是圓角細線 `╭─╮` + Surface2 `#585b70`；兩套 box 字元同寬，切換零位移。框線粗細是不靠顏色的第二訊號。Blue 是結構色，跟 popup 層級色、使用者足跡的 Lavender 互不干涉。
- **statusbar 一列、footer（status line）一列**，列數鎖死，內容放不下從尾端捨棄（tdp L3）。

## 設計決定

- **零學習曲線。** 每個動作都透過 `Space` menu 露出。進階熱鍵（`P` pin / `S` sort / `C` compare / `Y` YAML / `E` edit / `N` ns / `>` settings / ...）是為了速度，但整張小抄都可以不理 —— `Space` 每一次都在當下的情境裡帶你走過同樣的 menu。上手文件就一句：*「When in doubt, hit Space.」*
- **組合，不取代。** Alterm（內嵌的常駐 shell，`Alt+t`）讓平常要跳出 kbu 才用的終端機工具都能在裡面跑。kbu 用來導覽與檢視；寫入側的操作用你信任的工具 —— scrollback 不分裂、不切 context，Alterm 在 `Alt+t` 切換之間保留 env / cwd / shell history。
- **編輯為什麼用內嵌 PTY。** 早期版本透過 `tea.ExecProcess` 跑 editor，再用 `kubectl apply -f` 套用結果。這個做法在離開 kbu 之後，把 kubectl 的確認訊息漏進宿主終端機的 scrollback，而 apply 與 edit 的語意差異也讓習慣 `kubectl edit` 的使用者意外。PTY popup 把一切留在 kbu 裡、直接用 `kubectl edit`，行為就是 `kubectl edit` 使用者預期的樣子。
- **Session 狀態與設定分檔。** v1.7.10 的 session 狀態檔（`state.yaml`）跟 `config.yaml` 放在同一個目錄。config 是使用者手寫的偏好（保留註解），state 每次離開都自動重寫 —— 混在一起會破壞「config 是我的文件」這條信任邊界。state 自動管理，一般不該手改（手改的是 config.yaml）。
- **Region 內主要意圖提前（v1.7.6）。** 開 menu 當下的 app 狀態明確指向**單一**意圖時，該項目提升到所屬 region 的第一位；否則維持原序，免得每次洗牌破壞肌肉記憶。目前唯一的案例：compare 模式 + cursor 在候選列時，「Compare to anchor」→ item 區第一項；cursor 在 anchor 上（Unmark 是收拾動作）或還沒鎖定（Mark）都維持原序。反例：cursor 在某個 pod 上不提升 `[Y]AML`（意圖不單一）；剛用過 Edit 也不提升 Edit。
- **panel 邊框 hint 只放依 tab 而定的鍵。** core key（Tab / Space / Esc / Enter / ?）的全 app 預設語意不在 panel 邊角重複；`Enter` / `Esc` 只有在該 tab 的行為跟預設不同時才寫進 hint（Relatives 的 `enter: drill`、depth>1 的 `esc: back`、compare 鎖定中的 `esc: exit compare`）。理由：窄版面時 panel 內寬只有 28–38 格，塞進 core key 會擠掉依 tab 而定的揭露空間。
- **glyph 子集。** 只用 `U+f...` 區段（Font Awesome / Octicons / Material Design Icons），避開 `U+e...`（Devicons / Codicons / Pomicons —— 跨字型支援度低、容易變方框）。**唯一例外**是 panel 外框用的 Powerline 分隔符 `U+E0B0`–`U+E0B7`：它是 powerline 的事實標準（starship / tmux / vim-airline / lualine 都用），每套 Nerd Font 必附、寬度穩定 1 格。收尾用圓（`E0B4` / `E0B6`）、內部銜接用三角（`E0B0` / `E0B1`）。Nerd Font 是設計的一部分，不做沒裝字型的降級分支。
- **Lavender 專給使用者足跡。** Pinned、settings ON、compare anchor、非 focus 的 cursor chip 用 Lavender；popup 邊框從 lavenphire25 起算、永不直接用 Lavender 那條明度帶。Peach 是警告、Red 是錯誤，兩者不參與 popup 層級色。
- **source 預設留在底下，只由使用者對 source 的動作關掉**（tdp F4）。popup A 開出 B 時，A 不因「開 B」而關閉；`Esc` 關 B 回到 A。反模式是在 caller 裡先 `Close()` source 再開 target。
- **context-shift target 清掉 source**（tdp T1）。`txPty`（kubectl edit / exec，分鐘級的 subprocess）、`shellPty`（Alterm）、`enterDrillDown`（panel 2 換欄位與列）不是 inline 動作：從它們回來時，浮在已換掉的畫面上的舊 source 是 stale 的。規則：context-shift target 的 **entry handler** 在最前面呼叫 `AppModel.closeAllBlockingPopups()`（關掉所有 blocking popup，排除 PTY 自己與 toast；沒有東西開著時回 `nil`），由 target 關、不由 caller 關，每個叫出它的地方自動拿到正確行為。目前的 entry point：`app.go` 的 `startEditMsg`、`startShellExecMsg`、`Alt+t` handler、`enterDrillDown()`。新增 context-shift target（例如日後的 port-forward viewer）時一併補上。
- **Logs 失焦不變暗**（tdp T2）。panel 3 失焦時，Events / Conditions / Relatives / History 暗到 overlay1，Logs 不暗、保留 pod / container 的顏色。業界前例：Lens、k9s 的串流 log 都不變暗。

### PTY 裡的鍵

tdp v0.1.4 起，K10 只要求 PTY 至少有一個出口鍵，其他組合鍵由 app 決定、跟出口鍵一樣常駐揭露（v0.1.2 的 K10 只准出口鍵，
下面兩條那時列在「偏離 tdp」）。

- **PTY 裡攔下捲動鍵（tdp K10）。** Alterm、`kubectl edit`、`kubectl exec` 的 PTY 不在 alt-screen 時，`PgUp` / `PgDn` / `Home` / `End` 由 kbu 攔下做 10k 行 scrollback（`ptyview.go`），不送給子程序：純 shell 輸出沒有自己的翻頁，少了 scrollback 就看不到捲出畫面的輸出。子程序一進 alt-screen（vim、less、htop、kubectl edit 的 editor）這四個鍵就照常轉送，讓它們保有自己的翻頁。揭露：不在 alt-screen 時下框 hint 寫 `PgUp/Home:scroll`。
- **Alterm 的出口多一個 `Ctrl-t`（tdp K10）。** Alterm 除了 `Alt-t` 也攔 `Ctrl-t`（`app.go`、`ptyview.go`；panel 上的 `Ctrl-t` 同樣叫出 Alterm），因為錄 demo 用的 VHS 0.11 在 Chrome 與 PTY 之間會丟掉 Alt modifier，demo tape 只能送 `Ctrl-t`。代價：Alterm 裡 zsh 的 transpose-chars（`Ctrl-t`）用不到。這個別名不出現在任何 help 或 hint —— 這一點仍是偏離，見「偏離 tdp」。

## 已否決，不要重提

- 用 `tea.ExecProcess` 跑 editor 再 `kubectl apply -f` 套用（改成內嵌 PTY 直接跑 `kubectl edit`，見「設計決定」）。
- 把 session 狀態寫進 `config.yaml`（改成獨立的 `state.yaml`）。
- panel 3 的 `/` 搜尋（有 cursor 的 tab 不容許過濾列；大量內容用 `Y` + editor）。

## 已知的牆與未做

- **彙總 log 的 Pod 汰換**：串流在選列當下取快照，rollout 後要重新選列才會接上新的 Pod。
- **Helm 沒有 watch API**：release 清單每 3 秒 poll 一次 `helm list -A`。
- **非 Mono 的 Nerd Font**：helm-managed 列與 popup 上框可能偏 1 格（見「運作方式」的 Nerd Font 渲染）。
- **未做**：`:` command palette（舊的熱鍵表列為 future）。
- **尚未符合 tdp 的地方**：逐條列在 [`kbu-terminu-fix.md`](kbu-terminu-fix.md)。

## 偏離 tdp

原本這裡的兩條 K10（PTY 裡攔下捲動鍵、Alterm 多一個出口 `Ctrl-t`）在 tdp v0.1.4 起不再是偏離：K10 改成「至少一個出口鍵，
其餘組合鍵由 app 決定」，兩條移到「設計決定」的「PTY 裡的鍵」。剩下的只有 `Ctrl-t` 不揭露這一點。

- **Alterm 的第二個出口 `Ctrl-t` 不揭露（K10）。** K10 要求 PTY 裡保留的鍵跟出口鍵一樣常駐揭露；`Ctrl-t` 刻意不出現在任何
  help 或 hint（`app.go` 的 `Alt+T` 處理、`ptyview.go` 的註解寫明）：它只為錄 demo 存在（見「設計決定」），使用者該記的出口是
  `Alt-t`，揭露兩個只會多一件要記的事。`Ctrl-t` 不是給使用者的鍵，是開發時 VHS 送不出 `Alt+t` 才開的，維持不揭露
  （使用者 2026-09-27 裁定）。

## 設計文件導讀

kbu 沒有另外的設計文件；每個功能的理由在本文件的「運作方式」與「設計決定」，每個版本改了什麼在 [`CHANGELOG.md`](../CHANGELOG.md)。popup 的完整規格在本機的 `.claude/rules/popup-convention.md`（不進版控）。

| 檔案 | 內容 |
|---|---|
| [`icon.svg`](icon.svg) | kbu 的圖示（README 與 splash 用） |
| [`km8-icon.svg`](km8-icon.svg) | 改名前（km8）的圖示 |
| [`social-preview.png`](social-preview.png) | GitHub 的 social preview |
| [`demo-basics.gif`](demo-basics.gif) | README 唯一的 demo |

靈感來自 [Lens IDE](https://k8slens.dev/)、[lazygit](https://github.com/jesseduffield/lazygit)、[lazydocker](https://github.com/jesseduffield/lazydocker) 與 [k9s](https://github.com/derailed/k9s)。以 Go 與 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 建構，樣式用 [Lip Gloss](https://github.com/charmbracelet/lipgloss)，Kubernetes 走 [client-go](https://github.com/kubernetes/client-go)。

## 建置與開發

從原始碼安裝：

```bash
go install github.com/vulcanshen/kbu/cmd@latest
```

本地編譯：

```bash
git clone https://github.com/vulcanshen/kbu.git
cd kbu
go build -o kbu ./cmd/
./kbu
```

`make`（或 `make help`）列出所有 target：`make build`（`CGO_ENABLED=0` 靜態、`-trimpath`、strip、注入版本）、`make run`、`make test`、`make test-race`（每次 release 前跑一次）、`make vet`、`make fmt`。`go install` 裝出來的版本字串不經 ldflags 注入。

TUI 邏輯用 Bubble Tea 的 Model-Update-View 以程式測試（送 `tea.KeyMsg`、斷言 model 狀態，不需要 TTY）；`internal/k8s` 的測試要連真的叢集（開發時用 OrbStack）。

**demo gif**：README 只放一張 `docs/demo-basics.gif`（owner 的 GitHub profile 也用它），用 VHS 錄，tape 與展示用的 config 放在 `.local/demos/`（不進版控）。其他六張（namespace、relatives、yaml-edit、compare、helm、alterm）在 2026-09-26 從 README 拿掉、檔案刪除；tape 還在 `.local/demos/`。

## 發布

push 一個 `v*` tag，GitHub Actions（`.github/workflows/release.yml`）在 ubuntu 與 macOS 跑 `go test -race ./...`，過了由 goreleaser 建 linux / darwin / windows × amd64 / arm64，發到 Homebrew tap 與 Scoop bucket；release notes 取自 `CHANGELOG.md` 對應的那一節。

踩過的坑：goreleaser 的 `--release-notes` 不可靠，workflow 最後一步用 CHANGELOG 強制覆寫 release body；這一步 `if: always()`，因為 goreleaser 在較後面的步驟（例如 Homebrew / Scoop tap push 401）失敗時 release 通常已經建好、只差 body。

發版前：CHANGELOG 加上這一版的一節；README 整份重讀（功能、按鍵、安裝、設定都要符合現況）；`go build -o kbu ./cmd/`；`go test ./... && go vet ./...`。
