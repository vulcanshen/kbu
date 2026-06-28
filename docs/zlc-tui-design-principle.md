# TUI Design Principle

開發過程中累積、可遷移到任意 TUI app 的通則。原則本身獨立於 K8s / Bubble Tea
/ Lipgloss 等任何特定框架或功能領域。

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

**km8 v1.7.5 案例 — Logs unfocus 是否 dim**：

| 步驟 | 內容 |
|---|---|
| Origin UX of 「unfocus dim」 | 不搶 focus、視覺退階、讓 user 知道焦點在哪 |
| 當下衝突 | Streaming 內容（Logs）needs glance-readability；dim 之後使用者餘光看不到正在到達的更新 |
| 兩個 UX 同源嗎？ | 否。「不搶 focus」 vs 「streaming glance」是兩個不同 UX 目標 |
| 結論 | 擴充規則（§7.x streaming exception）、不壓掉 streaming UX |

**也適用反向警覺**：當新情境恰巧 fit 既有規則、要警覺「**這次合規 = 真的
UX 對齊、還是只是僥倖**」、別讓規則替你做思考。規則只是縮短 derive 距離的
shortcut、不該變成 derive 的替代品。

---

## 術語定義

### Surface

**任何能成為 focus 對象、跟使用者直接互動的 UI 容器**。在這份文件的範圍
內、surface 主要指 **panel 與 popup**。Statusbar / footer 等持續性顯示區
**不是 surface**（focus 不會 land 在它們身上、不接受直接互動）。

### Focus

**使用者當下能直接操作的 UI 位置**。Focus 是 ZLC、互動規則、popup 規則等
多條原則的共同基礎、必須先定義清楚。

Focus 有兩個層次：

1. **Surface 層** — 當前 active 的 surface（哪個 panel 或哪個 popup）
2. **位置層** — 該 surface 內的具體位置（cursor 指的 row / 選中的 tab /
   聚焦的子元件等）

例：

| 情境 | Focus |
|---|---|
| Sidebar panel active、cursor 指著某 item | sidebar panel + 該 item |
| 主 list panel active、cursor 指著某 row | 主 list panel + 該 row |
| Popup 開啟、cursor 指 menu 內某行 | popup + 該行 |

Surface 層的切換由 `Tab` 跟 surface-jump key 控制（§4.1）；位置層的移動
由 surface 自身的 cursor 鍵（vim 風 `j` / `k` 等）控制、不換 surface。

「**作用對象在 focus 範圍內**」（§A 判準）意指作用對象是當前 surface 本
身、或位置層上的具體 item / row / tab。

### Contextual / Non-contextual 動作

**Contextual 動作** — 有明確作用對象、且作用對象在當前 focus 範圍內的動作。
例：對 cursor 指的 item 做檢視 / 編輯 / 刪除、對當前 panel 的 list 做排序
/ 過濾。

**Non-contextual 動作** — 沒有明確作用對象、或作用對象不在當前 focus 範圍
內的動作。多為 app-level 的全域動作。例：切換全域 sub-window、開啟設定、
退出 app。

ZLC（§A）的覆蓋範圍 = **所有 contextual 動作的集合**。Non-contextual 動作
落在 ZLC 範圍外、不在 Space menu 完整性的判斷之中。

---

## 核心設計哲學

底下兩條是這套原則的最頂層 — 一條規範**目標**（ZLC）、一條規範**機制**（專職化）。
七大分類底下所有規則、都是為了實現這兩條而展開。

### A. 遇事不決用 Space — ZLC (Zero Learning Curve)

**目標**：使用者不需要看文件、不需要記憶 hotkey、隨時迷路按 `Space` 就能找回
方向。

每個 panel / cursor / popup 焦點上、按 `Space` 都能打開「**此處能做什麼**」
的完整清單。Letter hotkey 是加速捷徑、不是必經之路 — 想學再學、不想學完全
靠 `Space` 也走得通。

#### ZLC 的覆蓋範圍：focus 上下文相關的所有動作

ZLC 規範的是「**當前 focus 上下文中、使用者能做的所有動作**」。Space menu
必須完整列出這些動作 — 不論該動作是 app 的「主功能」還是「便利附加」、
只要它跟當前 focus 相關、就必須在 Space menu 內現身。Space menu 在這個
範圍是**單一真實來源**、地位無可取代。

判準：

> **這個動作有作用對象嗎？作用對象在當前 focus 範圍內嗎？**
> - 有、且對象在當前 focus 內 → **contextual**、必須在 Space menu
> - 無作用對象、或對象不在當前 focus 內 → **non-contextual**、不在 ZLC 範圍

舉例：

| 動作 | 性質 | 在 ZLC 範圍嗎 |
|---|---|---|
| 對 cursor 指的 item 做檢視 / 編輯 / 刪除 | contextual（作用於 cursor 那筆） | 是、Space menu 必有 |
| 對當前 panel 的 list 做排序 / 過濾 | contextual（作用於當前 panel） | 是、Space menu 必有 |
| 對 cursor 指的 item 做標記 / 收藏 / 標 anchor | contextual（作用於 cursor 指的對象） | 是、Space menu 必有 |
| 切換 app-level 的全域 sub-window | non-contextual（不歸任何 focus 管） | 否、走別的介面揭露 |
| 進入 app-level 子介面（設定 / 說明 / 退出） | non-contextual（app-level） | 否、走別的介面揭露 |

「便利 vs 必要」這個維度跟 ZLC **無關** — 一個動作即使是「便利附加」（缺
它 app 還能完成本質目的）、只要它作用在當前 focus 上、就是 contextual、
必須在 Space menu。反之、即使是必要的全域 toggle、也不會放 Space menu、
因為它不歸任何 focus 管。

#### Non-contextual 動作不稀釋 ZLC

Non-contextual 動作（global toggle、settings、help 等）天生不適合 Space
menu — 強塞進去反而違反 §6.6 的 cursor-first 排序、讓 menu 雜亂無焦點。

這類動作**不在 ZLC 規範範圍**。開發者可以用 footer / cheatsheet / help
popup 等方式揭露、也可以選擇不揭露、都不算 ZLC 破洞。

關鍵：**Space menu 的單一真實來源地位、只在「focus 上下文」這個範圍內成
立**。Non-contextual 動作落在這個範圍外、不該硬塞進來。

#### 其他衍生影響

- **Onboarding 介面 = Space menu（針對 contextual 動作）**。沒看過 README
  的使用者、光按 `Space` 就能在每個 focus 上完成該 focus 支援的所有動作。
- **`Space` 在任何 focus 都不能「沒回應」** — 即使該 focus 沒有具體動作、
  也要顯示 cheatsheet 或「無動作」訊息、不能讓使用者按下去什麼都沒發生。

ZLC 不是 nice-to-have、是這套設計的**核心目標**。§4.1 / §4.2 / §6.6 都是
為了實現它而存在的規則。

### B. 元素專職化 — 一個元素、一個意思、不兼職、不共用

**機制**：視覺/互動元素一旦被指派一個語意、就「專職化」、其他語意不能借用
同一個元素表達。

這條跨色彩、符號、hotkey、popup taxonomy 等多個分類、是底下所有原則的編碼
基礎、也是檢驗新規則是否 well-formed 的試紙。

舉例：

- 顏色帶被 user footprint 訂走、popup 不能用同明度（即使視覺上好看）
- `Esc` 被「關閉/退出」訂走、不能拿去當「確認」
- `Space` 被「打開 menu」訂走、不能拿去當別的動作
- glyph 位置被「類型訊號」訂走、不能拿來當純裝飾

---

## 1. 空間結構 (Spatial)

### 1.1 Panel 排列必須過「窄寬度可用」測試

不論 app 功能、panel 切分必須先滿足窄寬度終端的可用性。**怎麼拆是開發者的
決定**、原則只規範「在窄寬下要可用」這個底線。

### 1.2 Width stability

Panel header / statusbar 的動態元素必須維持寬度恆定、否則主視野寬度浮動會
造成 popup 開啟時視覺晃動。

### 1.3 Statusbar / footer 永遠單行

內容由 app 自己決定、但長度 + 數量必須能單行顯示完。折行到兩行會吃 main
panel 高度、引發整個 layout 連鎖偏移。

---

## 2. 色彩 (Colour)

### 2.1 三個錨點決定全 app 配色

只需指派三個顏色、整個 app 的色系即成形：

| 錨點 | 角色 |
|---|---|
| `base` | 最深、canvas、什麼都不是 |
| `user footprint` | 比 base 淺、使用者足跡（選中標記 / Pinned / 設定 ON 等） |
| `popup ceiling` | 比 user footprint 淺、popup 最頂層的色 |

中間 popup 各層的顏色用插值算法從錨點計算（見 §2.5）、不另外指派。

### 2.2 明度作 z-axis

TUI 沒有 shadow / elevation / parallax、能編碼深度的只有色相 + 明度。深度從
深 (base) 到淺 (popup 最頂層) 一路遞減、絕對不能逆轉。

### 2.3 顏色帶專職化（meta 律的具體實例）

每個 semantic 層佔一個明度帶、不准跨用。Popup border 不能用 user footprint
的明度、不是「不好看」、是那條明度帶已經訂出去了。

### 2.4 Override 色不參與 z-axis

特殊強訊號（warning、user identity 等）獨立於明度系統、永遠跳出搶眼、
優先於層級規則。例如警告色不論浮在哪一層 popup 都保持警告色、不跟著層級
變色。

### 2.5 Popup 各層的顏色用插值從錨點計算

之所以「三個錨點」就足夠、是因為 popup 各層的明度是用算法**插出來的**、不
另外指派。

```
N = 預期最大 popup 巢狀深度
popup layer K 的明度 = lerp(user_footprint_lightness,
                            popup_ceiling_lightness,
                            K / N)
K > N 時 clamp 到 popup_ceiling
```

實作可選：
- 線性插值（lightness 維度均分）
- HSL 漸進
- 預製離散色階（例：N=4、取 25% / 50% / 75% / 100% 四段）

開發者只要決定三個錨點 + 一個插值方式、N 層 popup 顏色就確定了、不需要逐層
人工指派。

---

## 3. 符號語彙 (Symbol Vocabulary)

### 3.1 Nerd Font is design

Nerd Font icon 是 design vocabulary 的一環、不可當 optional 依賴拔掉。沒裝
Nerd Font 就不該用這套 app — 不要設計「降級顯示」分支、那會讓設計妥協。

### 3.2 Glyph 限定 `U+f...` 開頭區段

避開 `U+e...` 區段：

- `U+e...` 屬 Nerd Font 較私有的子集（Devicons / Codicons / Pomicons 等）
- terminal 字體未完整安裝或版本不夠新時、會 fallback 成豆腐塊 / 問號 / 空白
- `U+f...` 區段（Font Awesome / Octicons / Material Design Icons 為主）跨
  字體支援度高、是保險選擇

### 3.3 Surface 標籤格式：glyph + text

任何 surface（panel / popup）的內容標籤都採「**類型訊號 + 內容訊號**」並列：

- 類型訊號 = glyph、一眼識別此 surface 屬於哪一類
- 內容訊號 = text、表達此 surface 的個體內容

兩者**缺一不可** — 不能 glyph-only、不能 text-only。

---

## 4. 互動 (Interaction)

### 4.1 Core 4 鍵跨全 app 一致

`Tab` / `Enter` / `Esc` / `Space` 在任何 panel / popup 都保持相同語意：

| 鍵 | 語意 |
|---|---|
| `Tab` | focus 切換 |
| `Enter` | 確認 / 進入 |
| `Esc` | 取消 / 退出 |
| `Space` | **打開 menu popup**（當前 cursor / panel 的可用動作清單） |

這四鍵的語意絕對不能因 panel / popup 而變、否則使用者基本導航就壞了。
`Space` 的這個語意是 ZLC（§A）的入口、不是普通的 hotkey。

### 4.2 Letter hotkey ⊆ Space menu （完整性原則、ZLC 的具體實作）

字母 hotkey (`e`, `d`, `c` 等) 屬於補充層、是 Space menu 內動作的**加速捷
徑**、不是新增功能。完整性測試：

> 一個沒看過 letter hotkey 的新使用者、光靠 `Space` 應該能在每個 focus 上
> 做完該 focus 的所有 contextual 動作。如果某個 contextual 動作只能用
> letter hotkey 觸發、Space menu 沒有、那就是 ZLC 破洞。

也就是：

- **動作清單** = Space menu 內容
- **動作清單的快捷鍵** = letter hotkey
- 任何 letter hotkey 對應的 **contextual** 動作、必須在對應 focus 的 Space
  menu 內出現

Letter hotkey 是「給知道的人」的優化、Space menu 是「給所有人」的完整界面。

Non-contextual 動作（全域 toggle、settings、help 等）不在本條規則範圍 —
由 §A 的 non-contextual 條款處理、不算 ZLC 破洞。

### 4.3 Esc 可關「所有看得到的東西」

任何可見浮層（popup、toast、auto-dismiss toast 也算）都必須接受 Esc 立即
關閉。使用者沒有等動畫倒數的義務。

---

## 5. Mouse（負面規範）

唯一一條「規範什麼不該存在」的原則、跟其他正面規範不同性質。

### 5.1 Mouse 可有可無

Mouse 不是 first-class input、是 keyboard 的可選 alternative。沒有 mouse
也必須能完整操作 app。

### 5.2 Mouse 必為 keyboard 的 mapping、不引入新語意

若實作 mouse 支援、行為必須一對一對應到 keyboard、**不能有「只有 mouse
才能做的事」**：

| Mouse 動作 | 對應 keyboard |
|---|---|
| 左鍵單擊 | Select（focus panel + item） |
| 左鍵雙擊 | Enter |
| 右鍵 | Space（打開 menu popup） |

這條讓 mouse 自然成為「keyboard 的 alternative input」、不是「另一套要學
的東西」。

---

## 6. 浮層 (Transient Surfaces)

### 6.1 Popup 四類 taxonomy

依「誰渲染 body」+「需不需要 padRow 呼吸空間」分類：

| 類 | body 渲染者 | padRow | 用途 |
|---|---|---|---|
| **menu** | app 自己 | 有 | 互動選擇器 (j/k + Enter) |
| **message** | app 自己 | 有 | 短文字 + 動作 (confirm / toast / help) |
| **viewport** | app 自己 | 無 | 長內容、可滾動 (YAML / diff / log) |
| **pty** | 外部 subprocess | 無 | 內嵌 terminal session |

每類有固定的 layout 規格、不允許「混血」popup（例如一個 popup 同時是
menu + viewport）。

### 6.2 Popup 開關必有動畫

In 與 out 都必須有動畫、否則使用者感受不到 z-axis 變化。

參考 timing（落在人類感知合理區間）：

| 動作 | 時長 |
|---|---|
| Open | ~160ms |
| Close | ~160ms |
| In-place swap | ~120ms |

判準：使用者能感受到 z-axis 變化、但不會打斷節奏。一般 UX 共識落在
100~200ms。

### 6.3 Popup border 色 = 所屬 layer 明度

不可 hardcode、必須從 §2.5 的插值公式計算、跟 §2.2 明度 z-axis 同步。

### 6.4 Popup stack 預設保留 source

開啟 target popup 時、source popup 預設留在底層、Esc on target 回到
source。使用者沒有撤掉 source、只是在 target 上做了一次互動。

例外情況見 §7.1。

### 6.5 Esc 通殺、auto-dismiss 也算

任何可見 popup 必須支援 Esc 立即關閉、包含 auto-dismiss 的 toast。
參照 §4.3 通則。

### 6.6 Menu popup 若分 region、第一 region 固定為 cursor 操作

Menu popup **不一定要分 region** — 整體只有單一類動作時、直接列出即可、不
需要 header。

當 menu 的動作清單需要依「動作對象」分組呈現時、才適用以下規則：

- **第一 region 固定**：當前 cursor 上的操作
- **後續 region**：依操作類型分組、**每個 region 需有 header 說明此類型**

這條讓使用者從上往下讀就能優先看到「**對著我選的這個 item 我能做什麼**」、
其次才看到全局可用的操作。

### 6.7 錯誤訊息呈現

任何錯誤必須立刻可見、但**不能阻塞 app**：

- popup / toast 即時顯示
- Esc 可關閉（§4.3 通則）
- 是否有「歷史記錄查閱」介面是 app 自己決定的層、不寫進通則

---

## 7. 時間軸 UX (Time-axis UX)

UI 原則「靜止可見」、本分類的原則「只在時間軸上才存在」 — 只能從 user flow
推演、靜止截圖看不出來。

本分類目前只有一條成形原則、其他屬「待累積樣本」的觀察池。隨 app 開發完整
度提升、新原則會慢慢浮現。

### 7.1 Target popup 完成後、source popup 是否仍有意義？

**預設保留 source（見 §6.4）**。只有 target popup 在功能設計上明確判斷
「使用者完成 target 後、source 已失去意義」、target 才於進場前清除 source。

這個判斷無法寫成通則：

- 從 code 看不出來 — confirm 跟長時 session 在 code 層都是「target 進場前」
  這同一個時點、長得一樣
- 從 mockup 看不出來 — 靜止狀態下「source 是否還有意義」沒有答案
- **只能從「使用者用完 target 後、心智上還會不會想看 source」這個推演判斷**

判準需 case-by-case、由 target 的功能內容決定。

典型對照：

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

**km8 案例**（v1.7.5）：

| Panel 3 tab | streaming? | unfocus 處理 |
|---|---|---|
| Logs | ✅ k8s log stream | 不 dim、container/pod hash color 保留 |
| Events | ❌ 靜態 list | dim 至 overlay1 |
| Conditions | ❌ 靜態 table | dim 至 overlay1 |
| Relatives | ❌ 靜態 nav hub | dim 至 overlay1 |
| History | ❌ 靜態 list | dim 至 overlay1 |

**業界 precedent**：Lens、k9s 對 streaming logs 都不 dim、是 monitoring
TUI 的事實標準。

**跟 §B 元素專職化的關係**：streaming 例外**不是** dim 元素本身的語意 overload
（dim 仍只代表「退階」）、而是「unfocus 這個 panel state 觸發 dim」這個 rule
本身根據內容類型有兩個分支。例外的記載讓 design 一致性 traceable、不變
implicit knowledge。

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

---

*隨後續觀察累積會持續修訂。*
