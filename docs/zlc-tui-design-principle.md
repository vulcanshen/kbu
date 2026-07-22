# TUI Design Principle

一份跨 TUI app 的通用設計原則、獨立於任何 K8s / Bubble Tea / Lipgloss 等
特定領域或框架。

本文件回答的問題是：**「在一個 terminal UI 上、什麼樣的設計能讓使用者
不靠文件就能用？」** ZLC（Zero Learning Curve）是核心目標、其他章節是
支撐 ZLC 的周邊規範。

> 本文件是 **interface**。某個 app 怎麼具體 implement、寫在它自己的
> implementation doc 裡（範例：`kbu-zlc-implementation.md`）。

---

## §0 Meta — Guide serves UX, not the other way

這份 doc 是過去 UX 決策的結晶、不是未來 UX 的約束。**當規則跟它原本要服
務的 UX 衝突、UX 贏、規則該擴充而不是壓掉 UX。倒果為因 = 把工具當目的、是
設計失敗。**

判斷一條規則該堅持還是該擴充的三步：

1. **回想規則的 origin UX**：這條規則當初是為了解決什麼 user-facing 問題？
2. **檢查當下衝突**：眼前情境是否還適用 origin UX？兩者是同一個目標、還是
   兩個不同目標恰巧發生在同一個 surface？
3. **如果不適用**：擴充規則（加 exception / sub-rule）而非壓掉 UX。

**也適用反向警覺**：當新情境恰巧 fit 既有規則、要警覺「**這次合規 = 真的
UX 對齊、還是只是僥倖**」、別讓規則替你做思考。規則只是縮短 derive 距離的
shortcut、不該變成 derive 的替代品。

---

## 術語定義

### Surface

**任何能成為 focus 對象、跟使用者直接互動的 UI 容器**。在本文件範圍內、
surface 主要指 **panel 與 popup**。Statusbar / footer 等持續性顯示區
**不是 surface**（focus 不會 land 在它們身上、不接受直接互動）。

### Focus

**使用者當下能直接操作的 UI 位置**。Focus 是 ZLC、互動規則、popup 規則等
多條原則的共同基礎、必須先定義清楚。

Focus 有兩個層次：

1. **Surface 層** — 當前 active 的 surface（哪個 panel 或哪個 popup）
2. **位置層** — 該 surface 內的具體位置（cursor 指的 row / 選中的 tab /
   聚焦的子元件等）

「**作用對象在 focus 範圍內**」意指作用對象是當前 surface 本身、或位置層
上的具體 item / row / tab。

### Contextual / Non-contextual 動作

**Contextual 動作** — 有明確作用對象、且作用對象在當前 focus 範圍內的動作。
例：對 cursor 指的 item 做檢視 / 編輯 / 刪除、對當前 panel 的 list 做排序
/ 過濾。

**Non-contextual 動作** — 沒有明確作用對象、或作用對象不在當前 focus 範圍
內的動作。多為 app-level 的全域動作。例：切換全域 sub-window、開啟設定、
退出 app。

這個區分在 ZLC 的範圍判定（§A）反覆使用。

---

## 核心設計哲學

底下兩條是本文件的最頂層 — 一條規範**目標**（ZLC）、一條規範**機制**（專
職化）。七大分類底下所有規則、都是為了支撐這兩條而展開。

### §A. ZLC（Zero Learning Curve）— 基礎操作貫穿全 app

**目標**：使用者**不需要看文件、不需要記憶 hotkey**、靠一套**跨 surface
不變的基礎操作**就能用完整個 app。學新 surface 時不用重新學「這個 surface
裡 Enter 是什麼意思 / Esc 又是什麼意思」、所有 surface 共用同一套基礎語
意、學一次走遍 app。

ZLC 是**目標**、不是某個特定 key 或某個特定 UI。User 在 app 內遇到的動作
分兩種性質、ZLC **同時涵蓋兩種**、各用一個 core-key 入口列出該類動作：

| Track | 動作性質 | 典型例子 | core-key 入口 |
|---|---|---|---|
| **Contextual** | 作用對象在當前 focus 範圍內 | 對 cursor row 做檢視 / 編輯；對當前 panel 做排序 / 過濾 | 典型 `Space`、列出當前 focus 能做的事（§A.1） |
| **Non-contextual** | 作用對象不歸任何 focus 管、屬 app 全域 | 切換 namespace / context、開設定、退出 | 典型 `?`、列出 app 所有全域動作（§A.2） |

兩條 track 都**屬於 ZLC**、各有自己的 core-key 入口、**結構對稱**。不
能用其中一條的手法去蓋另一條的需求 — contextual 動作擠進 §A.2 入口、
或 non-contextual 動作擠進 §A.1 入口、兩種都會壞 ZLC。

判準：

> **這個動作有作用對象嗎？作用對象在當前 focus 範圍內嗎？**
> - 有、且對象在當前 focus 內 → **contextual** → §A.1
> - 無作用對象、或對象不在當前 focus 內 → **non-contextual** → §A.2

#### §A.0 ZLC score — ZLC 是百分比分數、不是 binary

ZLC 不是「有 / 沒有」、是 0%-100% 的連續分數。每個 app 都落在這條軸上
某個位置：

- **100%**：完全不用事先學、第一次開 app 就能完成所有動作
- **0%**：不先學根本無法使用、任何動作都要事先讀文件

ZLC score 越高 = 越 user-friendly、越低 = user 學習成本越高。

**ZLC score 由兩個設計可控軸合成**：

| 軸 | 性質 | 定義 | 範圍 |
|---|---|---|---|
| **X. 揭露程度**（disclosure ratio）| 設計軸 | `X = user 透過 core-key 入口可以明確看到的操作數 ÷ app 包含的全部操作數` | [0, 1] |
| **Y. core-key role 數量** | 設計軸 | `Y = \|contextual core-key role ∪ non-contextual core-key role\|`（兩條 track 用到的 role 聯集大小、alias 共用 role 算 1 個）| [1, ∞) |

**ZLC score 公式**：

```
ZLC score = X × min(1, 5/Y) × 100%
```

兩個因子的意義：

- `X` — 揭露程度：入口列出多少 = user 不用查就能找到的比例。**分子**
  是「user 按 §A.1 / §A.2 core-key 入口、能直接看到並執行的操作」；
  **分母**是「app 提供的所有操作」（包括只能透過 letter hotkey 或外部
  文件才知道的）。任何「藏起來」的操作都把 X 拉低。
- `min(1, 5/Y)` — core-key 「同時握得住」係數：Y ≤ 5 時係數 = 1（沒
  penalty）、Y > 5 時係數 = 5/Y、線性 penalty。鼠標 5 button（左/右
  /中/滾輪上/滾輪下）是這個上限的物理類比。

**Y 算 role 不算 key — user 學的是概念、不是 binding**：

User 學的單位是 **role**（「取消」「focus 切換」「打開 contextual 入
口」等概念）、不是 binding 的 key 本身。Y 計算只看 distinct role 數、
不看 key 數。

| 情境 | 對 Y 的影響 |
|---|---|
| 兩條 track 共用同一 role（如「取消」、`Tab` focus 切換）| 聯集算 1 個、不重複計入 |
| 同一 role 多個 key alias（如 `Esc` 跟 `q` 都做「取消」）| 算 1 個 role、不增加 Y |
| 新增一個 key 但 role 已存在 | 不增加 Y |
| 新增一個 key 且 role 是新的 | +1 |

**Alias 完整性條件 — 完整 alias 才算同 role**：

一個 role 綁多個 key alias（如 `Esc` + `q` 都做「取消」）要算同一個
role、**必須在整個 app 所有 surface 都同樣有效**。如果只在某些 surface
有效、某些 surface 沒、就是「半套 alias」、不算同 role：

- 完整 alias：`Esc` 跟 `q` 在 main panel、所有 popup、所有 modal、所有
  surface 都做完全一樣的「取消當前最上層」— Y +0
- 半套 alias：`q` 只在 main panel 取消、popup 內 `q` 沒效（或做別的事）
  — user 要學「在哪些 surface `q` 可以、哪些不行」這個額外 conditional
  rule、等於多學一個概念、**Y 仍然 +1**

跟 §4.1「core-key 跨全 app 一致」對等：alias 也必須跨 surface 一致才
算同 role、否則破壞 user 「同一個 key 走全 app」的心智模型、反而引入
「要學的條件」、適得其反。

**Action 粒度 — X 計算的單位**：分子分母都用 **user 學習單位**、不用
implementation 數量。例：app 對 30 個 resource type 都支援「View YAML」、
user 學一次「Y view YAML」全套通用 → 算 **1 個 action**、不論 code 內
部有幾個 switch case。X 計算只看 user-side 學習單位、不看 implementation
breadth — 避免 framework 被 implementation 數量綁架（例：app 加 1 個新
resource type 不該讓 X 突然掉、因為 user 沒多學任何東西）。

X 跟 Y 任一條低、ZLC 都會掉。設計者只能調這兩個軸、score 自動算出來。

**對照例**：

| App | X 揭露 | Y core-key | min(1, 5/Y) | ZLC score |
|---|:---:|:---:|:---:|:---:|
| **km8** (entry-key + interactive menu)| 1.0 | 5 | 1.0 | **100%** |
| **nano** (ambient cheatsheet 永久揭露 + `^G` 補)| ~1.0 | ~5 | 1.0 | **~100%** |
| **vim 默認** (prompt-style、不揭露) | 0（`:` 給空 prompt、揭露 0 個 action；`:help` 要先學才能用）| 海量、設 ~30 | 0.167 | **0%** |

**km8 vs nano vs vim — ZLC vs 其他 dimension 的分離**：

km8 跟 nano 在 ZLC 上**幾乎同分**（都 ~100%）— user 一打開都能用、不需
事先讀 README。差別在 design polish、不在 ZLC：

| | km8 | nano | vim |
|---|---|---|---|
| ZLC | ~100% | ~100% | 0% |
| 揭露策略 | entry-key + interactive menu（`j/k` 選 + Enter execute）| ambient cheatsheet 永久顯示 + `^G` 補 hidden | 無揭露（`:` 是 prompt、不是 cheatsheet） |
| Hotkey 是否 mandatory | optional（`j/k` + Enter 可 bypass）| optional（即時學就能用） | mandatory（沒揭露、必須事先學）|
| Context-awareness | menu 跟 cursor 對齊、只列當下能做的 | screen-bottom 永遠列全部 | N/A |
| 學會後操作效率 | medium | low（沒 motion / macro） | 業界最高 |
| Target user | DevOps / SRE | 一次性 / 偶爾用 | 長期投資、把 editor 當第二母語 |

km8 vs nano 的真正差別在 **context-awareness / 介面整潔 / 學會後加速空
間**、這些都是 design polish、**不在 ZLC 規範內**。

---

**ZLC 是可疊加 layer — 揭露機制跟 app core 可分離**：

ZLC score 由揭露機制決定、不必 hard-coded 到 app core。同一個 app
core、加上「ZLC layer」（which-key / command palette / interactive
menu plugin 等）、ZLC score 就能大幅提升 — app 功能不變、只是揭露
管道改善。

**vim + LazyVim / which-key 案例**：

| 階段 | vim core 功能 | 揭露機制 | ZLC score |
|---|---|---|---|
| **vim 默認** | 海量 motion / edit / ex command | `:` prompt（不揭露）| **0** |
| **vim + which-key**（LazyVim 預設）| 同上、core 完全沒動 | `<leader>` prefix + popup 列出當前能按的所有 binding（典型 §A.1 entry-key + interactive menu pattern）| **接近 100%** |

vim core 一字未動、LazyVim 在 vim 上**疊加了一個 ZLC layer**、score
從 0 跳到接近 100。這驗證 ZLC 跟 app core 是兩件事、可分離。

**Implication**：

- ZLC 不必 hard-coded 在 app core、可以是 plugin layer
- 設計者可以做「ZLC plugin」、套到任何既有 app 上補強 ZLC
- 同 framework 適用兩種角度的設計者：

| 角度 | 目標 |
|---|---|
| **App / framework 設計者** | 設計時把 ZLC layer 直接 build-in（如 km8 Space menu / `?` help）|
| **Plugin / distro 設計者** | 在 ZLC = 0 的 app 上補 ZLC layer（如 LazyVim 在 vim 上補 which-key）|

- 評斷 app ZLC 時、要明確是「**default state**」還是「**+ ZLC layer**」
  — 同一個 app core 兩種 state 分數可以天差地遠（vim 0% vs vim+LazyVim
  ~100%）。

---

**重要 framework 邊界**：

> **「熱鍵的易用性」(hotkey ergonomics) 不在 ZLC 規範範圍內。**

ZLC 只規範「**user 能不能不靠事先學習就 reach 並 execute 操作**」— 不
論 user 是透過：

- **0-學習 execute**：`j/k` 選 + Enter execute（km8 §A.1 interactive menu）
- **即時學 + execute**：看 cheatsheet 看到 hotkey → 立刻按 hotkey
  execute（nano screen-bottom、km8 §A.2 `?` help popup）

只要 user 不需要事先讀 README、流程能在 app 內走完、都算 ZLC 友善。

至於這些 hotkey **好不好按**（單 key vs chord vs 三 key chord）、**記
不記得住**、**要不要分 mode**、**有沒有 motion composability** — 全
都是 hotkey ergonomics、是另一個 dimension、**由其他 framework 規範、
不在本文件範圍**。

---

**ZLC 高 ≠ 好 app**、**ZLC 低 ≠ 爛 app** — ZLC 只是「不需事先學習就
能用」這個 dimension 的度量、跟其他 dimension（hotkey ergonomics、
context-awareness、學會後效率、composability、肌肉記憶投資 ROI、…）
獨立。

本框架是給「**想要高 ZLC**」的 app 設計者用的 — 確認自己的設計選擇真
的拿到了想要的 ZLC 分數、而不是有意識的覺得「我 app ZLC 高」但實際算
出來是 0。如果設計者明白選擇「ZLC 低、其他維度高」（如 vim）、本框架
也能幫忙確認「沒選錯邊」。


**ZLC score 的反向表達**：

「user 事前認知門檻」是 ZLC score 的反面、同一件事不同方向看：

```
事前認知門檻 = 100% − ZLC score
```

事前認知門檻 = 0 ↔ ZLC = 100%；事前認知門檻 = 100% ↔ ZLC = 0。設計者
不需要單獨度量它、它跟 ZLC score 是同一個數字。寫進這份 doc 是因為某些
情境下「user 還要學多少」比「ZLC 多少」更直觀。

**ZLC 100% 不是強制目標** — 設計者要意識到自己的 app 落在哪、為什麼是
這個分數、是不是有意識的取捨（如 vim）。

#### §A.0.Y Y 軸規範：core-key 集合 ≤ 5 個

兩條 track 加起來、user 為了走完 app **需要學的 core-key 不能超過 5 個**。
鼠標的 5 個 button（左鍵 / 右鍵 / 中鍵 / 滾輪上 / 滾輪下）是這個上限的
物理類比 — **單一操作介面就應該能貫穿整個 app**。

Y ≤ 5 不是美學取捨、是 ZLC 的物理可行性：超過 5、user 同時握不住、就要
回去翻 cheatsheet、Z 上升、ZLC 線性掉。

典型 keyboard core-key 集合（example、非規範）：

| Core-key | 角色 | 對應條款 |
|---|---|---|
| `Tab` | focus 切換 | §4.1 |
| `Enter` | 確認 / 進入 | §4.1 |
| `Esc`（或 `q` 等取消 key） | 取消 / 關閉 | §4.3 |
| `Space`（或其他 contextual 入口 key） | §A.1 here-can-do-what 入口 | §A.1 |
| `?`（或其他 non-contextual 入口 key） | §A.2 全域動作入口 | §A.2 |

實際取多少個、設計者自選 — 單 panel app 可能不需要 `Tab`、只用 4 個；
某 app 把取消跟 quit 合進 `q`、騰一個 slot 出來。重點是**總數 ≤ 5**。

要再加 core-key 之前先 review：

- 它是不是其實該歸進兩個入口之一（contextual 進 §A.1、global 進 §A.2）？
- 它是不是 letter hotkey（入口內動作的加速捷徑）就夠了？letter hotkey
  **不算 core-key**、不佔 5 個 slot
- 真的非加不可、就要拿掉某個現有 core-key、不能無限擴張

#### §A.1 Contextual 動作 — core-key 入口列當前 focus 能做的事

**手法**：指派一個 core-key（典型 `Space`）作為 contextual 入口。使用
者迷路時、在當前 focus 按這個 key、跳出**當前能做的事的完整清單**。
Letter hotkey 是這份清單裡每個動作的加速捷徑、不另算 core-key。

**前提 — entry key 自身必須 user-discoverable**：

User 不可能按一個自己不知道存在的 key。Entry key 自身**必須有揭露管
道**、否則 X = 0（user 找不到入口、跟沒入口同效）。

**揭露管道必須存在**（強制）、**揭露形式自由**（設計者選）。例：

- footer / bottom statusline 顯示 `Space menu`
- sidebar 常駐 cheatsheet 列入口
- 開 app 時 onboarding screen 提示
- Panel 內遞迴揭露（某 Space menu 內列「按 ? 看全域動作」）
- 任何 user 不用先知道就能看到的地方

形式自由的核心是「user 第一次開 app 不靠任何外部知識、能看到至少一個
entry key」。這個前提**對 §A.1 跟 §A.2 entry key 都適用**。

**完整性原則**：

> 一個沒看過 letter hotkey 的新使用者、光靠 §A.1 入口就能在每個 focus
> 上做完該 focus 的所有 contextual 動作。

如果某個 contextual 動作**只能用 letter hotkey 觸發**、入口沒列、就是
ZLC 破洞 — 新使用者按入口找不到、必須去學 hotkey、違反「不需看文件就
能用」的承諾。

**常見誤判**:「便利 vs 必要」不是這條 track 的判斷軸：

設計者很容易直覺地用「這是 app 主功能還是便利附加？」當「動作要不要進入
口」的判斷標準 — 主功能進、便利附加不進。這個直覺錯的、會反過來破壞 ZLC：

- 動作即使是「便利附加」（缺它 app 還能完成本質目的）、只要它作用在當前
  focus 上、就是 contextual、**必須**進入口。否則 user 找不到、必須去學
  hotkey、ZLC 破洞。
- 反之、即使是必要的全域 toggle、也**不**進入口、因為它不歸任何 focus
  管 — 它走 §A.2、不走 §A.1。

判斷軸永遠是「**作用對象在不在 focus 範圍**」、不是「重不重要」。

#### §A.2 Non-contextual 動作 — core-key 入口列 app 全域能做的事

App 在開發過程中、會出現某些**必要但無法歸進 focus 上下文**的動作 — 全
域 toggle、模式切換、settings、help、quit 等。這些動作存在的理由是 app
的彈性需要、ZLC 不會也不該排除它們、但**它們不能擠進 §A.1 入口**（不歸
任何 focus 管、塞進去會稀釋當前 focus 的動作清單、user 找不到「我現在能
對 focus 做什麼」）。

對這類動作、§A.2 用對稱手法處理：

**指派另一個 core-key**（典型 `?` / `F1` / `Ctrl-K`）作為 non-contextual
入口。User 在任何 surface 按這個 key、跳出 **app 所有全域動作的完整清單**。

**完整性原則**：

> 沒看過 README 的使用者、不論當前在哪個 surface、按 §A.2 core-key 都
> 能找到 app 提供的全部全域動作。

如果某個全域動作只在某個特定 surface 才能觸發（或只能用隱藏 hotkey 觸
發、§A.2 入口找不到）、就是 §A.2 ZLC 破洞。

**揭露分兩層、分清強制 vs optional**：

| 層 | 內容 | 強制 / Optional |
|---|---|---|
| Layer 1 | **Entry key 自身的揭露**（user 知道 `?` 能按）| **強制**（同 §A.1 前提、不揭露 = X = 0）|
| Layer 2 | **個別全域動作的 ambient 揭露**（在 statusbar / footer / chip 等位置額外持續顯示個別動作的存在）| **Optional**（加分項、不影響 ZLC 完整性）|

- **Layer 1（強制）**：entry key (`?`) 自身要 user-discoverable、形式
  自由 — footer / sidebar / onboarding / 遞迴從別的 entry 揭露 / ...。
  如果 user 不知道 `?` 能按、就跟 vim `:` 同處境（X = 0、ZLC = 0）、
  不論 `?` 按下後揭露多完整都救不回來。
- **Layer 2（optional）**：個別全域動作的持續揭露（如「N: namespace」
  chip、「Alt-t: shell」chip）是 app 自選的加分項、增加 user 對個別動
  作的 ambient awareness、但不影響 ZLC 完整性。即使全部拿掉、user 透過
  `?` 仍能找到全部全域動作。

#### 兩條 track 加起來才是完整的 ZLC

- **§A.0.Y 保證**：core-key 總數 ≤ 5、user 同時握得住
- **§A.1 保證**：當前 focus 上下文裡所有能做的事、user 從 §A.1 core-key
  入口都找得到
- **§A.2 保證**：app 提供的所有全域動作、user 從 §A.2 core-key 入口都
  找得到

三條合起來、就達成「沒看過 README 的 user 在任何 surface、學 ≤ 5 個 key
就能完成 app 支援的所有動作」這個 ZLC 承諾。

衍生規則：

- **§A.1 / §A.2 入口在任何 surface 都不能「沒回應」** — 即使該 surface
  沒有具體 contextual 動作或全域動作、入口也要顯示 cheatsheet 或「無動
  作」訊息、不能讓使用者按下去什麼都沒發生。

### §B. 元素專職化 — 一個元素、一個語意、不兼職

**機制**：任何視覺/互動元素（顏色、符號、按鍵、容器格式等）一旦被指派一
個語意、就「專職化」、其他語意不能借用同一個元素表達。

這條跨色彩、符號、互動、浮層 taxonomy 等多個分類、是底下所有原則的編碼
基礎、也是檢驗新規則是否 well-formed 的試紙。

舉例（hypothetical）：

- 某個顏色被「使用者足跡」訂走、浮層 border 就不能用同明度（即使視覺上好
  看）— 否則使用者看到那個顏色會猜「這是我設定過的東西嗎？」反而干擾
- `Esc` 被「關閉/退出」訂走、不能拿去當「確認」— 否則一個按鍵兩個語意、
  使用者要 case-by-case 判斷
- 符號位置被「類型訊號」訂走、不能拿來當純裝飾 — 一旦兼職、類型訊號就
  失去信號強度

破壞專職化的代價是「使用者要學多套規則」、跟 ZLC 直接衝突。

---

## 1. 空間結構 (Spatial)

### 1.1 Panel 切分必須過「窄寬可用」測試

不論 app 功能、panel 切分必須先滿足窄寬度終端的可用性。**怎麼拆是設計者
的決定**、原則只規範「在窄寬下要可用」這個底線。

判準：在合理的最小寬度（多數 SSH session 預設）下、app 必須仍能完成核心
任務、不能要求使用者一定要寬螢幕才能用。

### 1.2 Width stability

Panel header / statusbar 的動態元素必須維持寬度恆定、否則主視野寬度浮動會
造成 popup 開啟時視覺晃動。具體做法：動態文字若有可能多種長度、用 padding
或固定 slot 寬度抵銷、不讓寬度跟內容綁定。

### 1.3 Statusbar / footer 行數絕對固定

行數由 app 自己決定（1 行、2 行、N 行皆可）、但一旦決定就**鎖在那個行
數**、不能因為當下內容多寡而 reflow。Main canvas 高度跟著 statusbar 行數
決定、statusbar 行數一變、整個 layout 連鎖偏移。

代價自負：

- 選 1 行：內容溢出時必須截斷 / 隱藏 / 滾動、不能折行多吃一行
- 選 2 行（或 N 行）：內容少時會浪費那幾行的空間、不能臨時縮回 1 行

判準：**app 顯示靜態穩定**永遠優先於「省畫面」 — 寧可多佔幾行恆定、不要
讓 user 看到 layout 因內容跳動。

---

## 2. 色彩 (Colour)

### 2.1 配色用「最少必要錨點」

TUI 沒有 shadow / elevation 可用、能編碼層次的只剩色相 + 明度。配色不需要
逐一指派每個元素的顏色、而是先選定「最少必要錨點」、其餘層次從錨點推導。

典型錨點集合（example、非規範）：

| 錨點 | 角色 |
|---|---|
| **base** | 最深、canvas、什麼都不是 |
| **user footprint** | 比 base 淺、使用者足跡（選中標記 / Pinned / 設定 ON 等） |
| **popup ceiling** | 比 user footprint 淺、浮層最頂層的色 |

實際要幾個錨點、是 app 設計者的選擇。重點是：**指派一次、推導全 app**、
不要每個元素都自己挑顏色。

### 2.2 明度作 z-axis

TUI 沒有 shadow / elevation / parallax、能編碼深度的只有色相 + 明度。深度
從深 (base) 到淺 (popup 最頂層) 一路遞減、**絕對不能逆轉**。

這條是通用規範 — 任何 TUI 都受此約束、否則使用者無法直覺感受「哪個浮層
在上 / 哪個在下」。

### 2.3 顏色帶專職化（§B 在色彩的具體實例）

每個 semantic 層佔一個明度帶、不准跨用。例：「使用者足跡」這個語意佔某
明度帶、浮層 border 就不能用同明度（即使視覺上好看）— 那條明度帶已經訂出
去了。

### 2.4 Override 色不參與 z-axis

特殊強訊號（warning / error / user identity 等）獨立於明度系統、永遠跳出
搶眼、優先於層級規則。

例：警告色不論浮在哪一層 popup 都保持警告色、不跟著層級變色 — 否則 user
看到「這層的警告色」會以為是該層的正常元素。

### 2.5 範例 mechanism：layer-based 插值

當「user footprint」跟「popup ceiling」兩個錨點之間有 N 層浮層需要區分時、
一個合法做法是用插值算出每層的明度：

```
N = 預期最大浮層巢狀深度
浮層 layer K 的明度 = lerp(user_footprint_lightness,
                          popup_ceiling_lightness,
                          K / N)
K > N 時 clamp 到 popup_ceiling
```

實作可選：
- 線性插值（lightness 維度均分）
- HSL 漸進
- 預製離散色階（例：N=4、取 25% / 50% / 75% / 100% 四段）

這是**範例**、不是規範。設計者也可以選擇用其他方式（例：手動指派每層、
N 太大時 fallback 同色）。重點是「層次能被視覺區分」、不是「必須用插
值」。

### 2.6 Panel-aware 視覺消歧

當同一個 letter hotkey 在不同 focus 下綁定不同 action（panel-aware
hotkey）、若兩個指示器**同時可見**、設計者可用視覺差異（明度、色相、
位置）讓使用者一眼看出「目前按這個鍵會 fire 哪一個」。

常見做法是 **anti-correlated brightness**：當前 focus 會觸發的那個用
active colour、不會觸發的那個 dim。亮度的「交班」本身就在傳達「按這個鍵
在這個 panel 會做什麼」、不需要文字標籤或 popup 解釋。

這條的成立條件：兩個指示器**同時可見**。如果只有一個會出現、不適用本條
（直接用 active colour 即可、沒人會誤會）。

跟 §2.4 邏輯類似：brightness 在這條被 panel-context 接管、不參與 popup
layer scale。

---

## 3. 符號語彙 (Symbol Vocabulary)

### 3.1 圖示字體是設計、不是 optional 依賴

若 app 用 icon font（Nerd Font 等）作為視覺語彙、它就是 design 的一部分、
不能當 optional 依賴拔掉。沒裝的使用者**不該用這套 app** — 不要設計「降級
顯示」分支、那會讓設計妥協。

這條反過來說也成立：如果 app 想兼容沒裝 icon font 的使用者、就不能用 icon
font 當設計語彙、應該整套改用 ASCII / Unicode 基本字符。**選一邊、不要兩
邊都做。**

### 3.2 Glyph 限定可靠子集

跨 terminal / 跨字體版本的 glyph 渲染穩定性差異很大、設計時要選**跨環境
fallback 最不容易掉 box** 的子集。具體哪個子集屬於 icon font 設計而定、
不寫進通則 — 通則只規定「選定後不能跨進不穩定區段」。

### 3.3 Surface 標籤格式：類型訊號 + 內容訊號

任何 surface（panel / popup）的內容標籤都採「**類型訊號 + 內容訊號**」
並列：

- 類型訊號 = glyph、一眼識別此 surface 屬於哪一類
- 內容訊號 = text、表達此 surface 的個體內容

兩者**缺一不可** — 不能 glyph-only、不能 text-only。前者使用者看不出是
什麼、後者使用者要花時間讀字決定它是哪一類。

---

## 4. 互動 (Interaction)

### 4.1 Core key set 跨全 app 一致

選定一組 core key（總數 ≤ 5、見 §A.0.Y）、語意在任何 surface 都絕對不變。
典型集合（example、非規範）：

| 鍵 | 典型語意 | 對應條款 |
|---|---|---|
| `Tab` | focus 切換 | §4.1 |
| `Enter` | 確認 / 進入 | §4.1 |
| `Esc` | 取消 / 退出 | §4.3 |
| `Space` | §A.1 contextual 入口 | §A.1 |
| `?` | §A.2 non-contextual 入口 | §A.2 |

要選哪幾個鍵、各鍵綁什麼語意、是 app 設計選擇。**絕對的部分是「選定後
跨 surface 不變、且總數 ≤ 5」**、否則使用者基本導航就壞了、ZLC 立刻破洞。

### 4.2 Letter hotkey ⊆ here-can-do-what 入口（完整性原則）

字母 hotkey 屬於補充層、是 here-can-do-what 入口內動作的**加速捷徑**、不
是新增功能。完整性測試：

> 一個沒看過 letter hotkey 的新使用者、光靠 here-can-do-what 入口應該能在
> 每個 focus 上做完該 focus 的所有 contextual 動作。如果某個 contextual
> 動作只能用 letter hotkey 觸發、入口沒列、那就是 ZLC 破洞。

也就是：

- **動作清單** = 入口內容
- **動作清單的快捷鍵** = letter hotkey
- 任何 letter hotkey 對應的 **contextual** 動作、必須在對應 focus 的入口
  內出現

Letter hotkey 是「給知道的人」的優化、入口是「給所有人」的完整界面。

Non-contextual 動作（全域 toggle / settings / help 等）不適用本條 — 它
們走 §A.2 的另一套手法（跨 surface 一致觸發 + 持續通道揭露）、不擠進
入口。

### 4.3 指派一個 core key 當「全 app 取消/關閉」

設計者必須指派一個 core key、語意是「關閉當前最上層的可見浮層 / 取消當
前操作」、跨 surface 不變。任何可見浮層（popup / toast / auto-dismiss
toast 也算）按下這個 key 都必須立即關閉、使用者沒有等動畫倒數的義務。

該 key 是哪一個是設計選擇、典型例：

- `Esc` — 最常見、跟多數 GUI / vim insert-mode-exit 慣例對齊
- `q` — 帶 vim / less / man page 風格的 TUI 常用
- 其他 — 任何 core key 都可以、只要全 app 不變

重點：**一旦選定、跨 surface 絕對不變**。如果某個浮層用一個 key 取消、
另一個浮層用別的 key 取消、就違反 §4.1 core key 跨 surface 一致、user
必須 case-by-case 記、ZLC 破洞。

### 4.4 Hotkey discoverability 標記方式

若 app 在 statusbar / menu entries / popup hints 等位置揭露 letter hotkey、
標記方式必須整個 app 一個 rule、不能混用多套。

常見作法（example、非規範）：

| 形式 | 範例 |
|---|---|
| 用 bracket 包住熱鍵字元 | `[X]label` |
| 用 angle bracket | `<X>label` |
| 用顏色強調熱鍵字元 | 熱鍵字元用 highlight colour、其他字元正常 |

哪一種都行、**重點是同 app 內只能選一種**、否則使用者要學多套規則、跟
§4.1 跨 surface 一致的精神衝突。

**Anti-pattern**：用顏色 / glyph 暗示「這個 element 是某個 hotkey 的入口」
而不顯式標記 hotkey。顏色 / glyph convention 是 designer 內部心智、使用
者不會自動連起來；要顯式標記、不要靠不可被 user 學會的 convention 傳達。

跟 §2.6（panel-aware 視覺消歧）配合：標記說「這是鍵」、明度說「這鍵在這
個 focus 會不會 fire」、兩者各司其職（§B 元素專職化）。

---

## 5. Mouse（負面規範）

唯一一條「規範什麼不該存在」的原則、跟其他正面規範不同性質。

### 5.1 Mouse 可有可無

Mouse 不是 first-class input、是 keyboard 的可選 alternative。沒有 mouse
也必須能完整操作 app。

### 5.2 Mouse 必為 keyboard 的 mapping、不引入新語意

若實作 mouse 支援、行為必須一對一對應到 keyboard、**不能有「只有 mouse
才能做的事」**。典型 mapping（example）：

| Mouse 動作 | 對應 keyboard |
|---|---|
| 左鍵單擊 | Select（focus panel + item） |
| 左鍵雙擊 | Enter |
| 右鍵 | here-can-do-what 入口 |

這條讓 mouse 自然成為「keyboard 的 alternative input」、不是「另一套要學
的東西」。

---

## 6. 浮層 (Transient Surfaces)

### 6.1 浮層分類

依「誰渲染 body」+「內容是否需呼吸空間」可分類、每類有固定的 layout 規
格、不允許「混血」浮層（例如一個浮層同時是 menu + viewport）。

常見分類軸（example、非規範）：

| 軸 | 典型分類 |
|---|---|
| 誰渲染 body | app 自己 / 外部 subprocess |
| 內容類型 | 互動選擇器 / 短文字 + 動作 / 長內容可滾動 / subprocess 接管 |

設計者選定分類後、每類有什麼 layout、由 app 自己定。重點是「**分類本身要
專職化**」（§B）— 一個浮層屬一類、不混。

### 6.2 浮層開關必有動畫

In 與 out 都必須有動畫、否則使用者感受不到 z-axis 變化。

判準：使用者能感受到 z-axis 變化、但不會打斷節奏。一般 UX 共識落在
100~200ms。

### 6.3 浮層 border 色 = 所屬 layer 明度

若 app 採用 layer-based 配色（§2.5）、border 色必須由 layer 推導、不可
hardcode、不可逆轉 §2.2 明度 z-axis 規範。

不採 layer-based 配色的 app 不適用本條。

### 6.4 浮層 stack 預設保留 source

開啟 target 浮層時、source 浮層預設留在底層、Esc on target 回到 source。
使用者沒有撤掉 source、只是在 target 上做了一次互動。

例外見 §7.1（target 完成後 source 是否仍有意義）。

### 6.5 取消 key 通殺、auto-dismiss 也算

任何可見浮層必須支援 §4.3 指派的取消 key 立即關閉、包含 auto-dismiss 的
toast。參照 §4.3 通則。

### 6.6 Menu 浮層若分 region、cursor-first

Menu 浮層**不一定要分 region** — 整體只有單一類動作時、直接列出即可、不
需要 header。

當 menu 的動作清單需要依「動作對象」分組呈現時、才適用以下規則：

- **第一 region 固定**：當前 cursor 上的操作
- **後續 region**：依操作類型分組、**每個 region 需有 header 說明此類型**

這條讓使用者從上往下讀就能優先看到「**對著我選的這個 item 我能做什麼**」、
其次才看到全局可用的操作。

### 6.7 錯誤訊息呈現

任何錯誤必須立刻可見、但**不能阻塞 app**：

- popup / toast 即時顯示
- 取消 key 可關閉（§4.3 通則）
- 是否有「歷史記錄查閱」介面是 app 自己決定的層、不寫進通則

---

## 7. 時間軸 UX (Time-axis UX)

UI 原則「靜止可見」、本分類的原則「只在時間軸上才存在」 — 只能從 user flow
推演、靜止截圖看不出來。

本分類隨 app 開發完整度提升、新原則會慢慢浮現。

### 7.1 Target 浮層完成後、source 是否仍有意義？

**預設保留 source（見 §6.4）**。只有 target 在功能設計上明確判斷「使用者
完成 target 後、source 已失去意義」、target 才於進場前清除 source。

這個判斷無法寫成通則：

- 從 code 看不出來 — confirm 跟長時 session 在 code 層都是「target 進場
  前」這同一個時點、長得一樣
- 從 mockup 看不出來 — 靜止狀態下「source 是否還有意義」沒有答案
- **只能從「使用者用完 target 後、心智上還會不會想看 source」這個推演判斷**

典型對照（hypothetical）：

| Target 類型 | 是否清除 source | 判斷 |
|---|---|---|
| 短時 confirm / message | 保留 | 使用者完成後可能想回原 menu 取消或繼續 |
| 長時 session（shell / 編輯器） | 清除 | 使用者從 session 出來、注意力已轉移、舊 menu 浮著反而恍神 |
| 大幅切換 context 的動作（drill-down 等） | 清除 | 底下視圖已換、舊 menu 對應的對象已不存在 |

### 7.2 Streaming 內容 unfocus 不退階

「Panel unfocus = dim 退階」這條 panel-level 規約適用於**靜態內容**、
不適用於 **streaming 內容**。

**Why**：dim 的隱含語意是「焦點不在這、稍後再看也行」。Streaming 內容
沒有「稍後」—— 資訊**正在到達**、dim 它等同於把使用者餘光的 glance 路徑
切斷、遺失流經畫面的更新。靜態內容你 focus 回去同樣可見、streaming 內容
focus 回去時剛才那些 line 已經滾出視窗。

**判準軸**：

> 問「user 用餘光看到這個 panel 在更新時、有沒有可吸收的資訊」。
> 有 → streaming → unfocus 不 dim、保留全部視覺資訊。
> 沒 → 靜態 → 依 panel unfocus 規約 dim 退階。

**跟 §B 元素專職化的關係**：streaming 例外**不是** dim 元素本身的語意
overload（dim 仍只代表「退階」）、而是「unfocus 這個 panel state 觸發
dim」這個 rule 本身根據內容類型有兩個分支。例外的記載讓 design 一致性
traceable、不變 implicit knowledge。

**對 §0 Meta 的呼應**：這條 sub-rule 是 §0「規則服務 UX、不是 UX 服務規則」
的典型例子 — 「unfocus dim」原規則的 origin UX（不搶 focus）跟 streaming
glance UX 是兩個不同目標、規則該擴充而非壓掉 streaming。

---

## 結語

這套原則的層次：

1. **核心設計哲學**（§A / §B）— ZLC 是目標、專職化是機制。底下所有規則
   都是這兩條的展開。
2. **七大分類**（§1~§7）— UI / UX / 互動的具體規範。
3. **跨類觀察**：
   - **UI 原則靜止可見、UX 原則時間軸才存在** — UI 層可早期規劃、UX 層
     emergent、要 app 長到一定完整度才浮上來。任何「v0.1 就寫完的 UIUX
     guide」必然缺後者。
   - **負面規範（Mouse）跟正面規範（其他）地位不同** — 負面規範規定什麼
     不該存在、用於防止語意系統被稀釋。

本文件描述「**該滿足什麼**」、不規範「**怎麼滿足**」 — 具體實現是各
app 的 implementation 層職責。

---

*隨後續觀察累積會持續修訂。*
