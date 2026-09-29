# kbu — terminu fix

kbu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.23/principle)（tdp v0.1.23）的地方，逐條待修。
修好一條就刪掉一條。

盤點日期：2026-09-29。依據 `main` 的 `26d8e48`（已對齊 v0.1.22，工作區乾淨）。**這一輪只對 v0.1.22 → v0.1.23 的改動**：

- D7：README 講 icon 寬度的地方（通常在前置需求的 Nerd Font 那一段），寫出 `<APP>__ICON_WIDTH` 與 `TERMINU__ICON_WIDTH` 兩個變數，
  並說明後者是全家族共用的：設一次，家族每個 app 都讀到；在家族 app 的 PTY 裡跑時，外層 app 會替它設好（D6）。寫法不限。不舉「一定
  佔兩格」的字型當例子（filu 實測：Maple Mono NF CN 的 icon 看起來兩格，游標只前進一格）。kbu 的 README 環境變數表是例子。

tdp 連結（README 兩份、docs、`.claude/rules`）已由 terminu session 從 v0.1.22 改成 v0.1.23，只改網址，跟這份清單一起留在工作樹，
還沒 commit。


## 先看

- 這一輪只改文件：清單、改釘的連結與 README 兩份可以同一個 commit；CHANGELOG `[Unreleased]` 記一條文件改動（要不要記由 kbu 定）。
- 修完拿 v0.1.23 全文再對一次（這一輪的 diff 只有 D7），刪掉這份清單。不 push、不發版；把這一輪寫進 terminu
  `.local/family-fix/kbu/README.md`。


沒有要修的。README 兩份的環境變數表已經有 `KBU__ICON_WIDTH` 與 `TERMINU__ICON_WIDTH` 兩列，寫明後者全家族共用、家族 app 會替在它終端機
裡跑的程式設好（v0.1.23 的 D7 就是照 kbu 這張表寫的），也沒有舉「一定佔兩格」的字型。這一輪只需要 commit 改釘的連結、刪掉這份清單。


## 待確認

沒有。
