# kbu — terminu fix

kbu 尚未符合 [terminu design principle](https://github.com/vulcanshen/terminu/tree/v0.1.21/principle)（tdp v0.1.21）的地方，逐條待修。
修好一條就刪掉一條，並同步 README（兩份）與 `docs/dev-remarks.md` 裡描述該行為的段落。

盤點日期：2026-09-29。依據 `main` 的 `57846a6`（已對齊 v0.1.20，工作區乾淨）。**這一輪只對 v0.1.20 → v0.1.21 的改動**
（`git -C ~/Documents/sideproj/terminu diff v0.1.20 v0.1.21 -- principle/`）：

- D5：選取文字的模式照 vim 移動 —— `h/j/k/l`、`w/b/e`、`0/$`、`gg/G`、`u/d`（user 要求寫回 tdp，filu 轉達）。
- D6：環境變數命名 `<APP>__<NAME>`（app 名後兩個底線、變數名全大寫單底線分隔），共用名 `<APP>__CONFIG` / `__STATE` / `__DATA`
  / `__CACHE`（都指向**目錄**）/ `__ICON_WIDTH`；給別的程式讀的變數例外；**改名不留舊名**（user 2026-09-29 裁定）。
- D6：疊 popup 時 popup 比畫面寬或高（調整終端機大小的那一格）：起點取 0、超出的部分切掉，不可以 panic（kbu、locku 照搬時抓到，
  filu 的參考實作有這個 bug）。

tdp 連結（README 兩份、docs、`.claude/rules`）已由 terminu session 從 v0.1.20 改成 v0.1.21，只改網址，跟這份清單一起留在工作樹，
還沒 commit。


## 先看

- 清單與改釘的連結先一起 commit，再動程式；commit 只加自己改的路徑。
- 每修一處補 model test、做 mutation；同一個 commit 同步 README 兩份與 dev-remarks；CHANGELOG 記 `[Unreleased]`。
- **環境變數改名是破壞性改動**：CHANGELOG `[Unreleased]` 要寫一條「改名，舊名不再讀」並列出新舊對照；CHANGELOG 裡已發版的舊段落不改。
  `.checkpoints/` 是本機筆記，不用改。改完 `grep -rn '<舊名>'` 除了 CHANGELOG 舊段落應該是零。
- **修完拿 v0.1.21 全文再逐條對一次**，修完刪掉這份清單。不 push、不發版；把這一輪寫進 terminu `.local/family-fix/kbu/README.md`。


## 1. 環境變數沒完全照家族命名 —— D6（v0.1.21）

**現況**：kbu 本來就用雙底線，但有三處跟 v0.1.21 對不上，另外還留著 km8 時期的舊名相容：

| 現在 | 意思 | 改成 |
|---|---|---|
| `KBU__CONFIGPATH`（`internal/ui/app.go`、`internal/config/config.go`） | 設定**檔**的路徑；`CONFIGPATH` 兩個字黏在一起 | `KBU__CONFIG`：設定**目錄**，`config.yaml` 放在裡面 |
| `KBU__STATEPATH`（`internal/config/state.go`） | 狀態**檔**的路徑 | `KBU__STATE`：狀態目錄，`state.yaml` 放在裡面 |
| `KBU_TEST_K8S`（`internal/k8s/client_test.go`） | 測試用，單底線 | `KBU__TEST_K8S` |
| `KM8__CONFIGPATH`、`KM8__STATEPATH`、`KM8__ALTERM_SHELL`、`KM8__ALTERM_LOGIN_SHELL`、`KM8__SHELL`、`KM8__LOGIN_SHELL`（`app.go`、`config.go`、`state.go`、`v2rename.go`） | km8 改名時留的舊名相容與 deprecation 提示 | 拿掉（照 v0.1.21「改名不留舊名」；user 裁定「可以改就現在改」） |

`KBU__ALTERM_SHELL`、`KBU__ALTERM_LOGIN_SHELL`、`KBU__ICON_WIDTH` 已經符合。

**規則**：D6（v0.1.21）—— `<大寫 app 名>__<變數名>`，變數名全大寫、單字之間一個底線；app 自己讀的變數（含測試用、傳給自己子程序的）
都照這個寫。共用名：`<APP>__CONFIG`（設定目錄）、`<APP>__STATE`（狀態目錄）、`<APP>__DATA`（資料目錄）、`<APP>__CACHE`（快取目錄）、
`<APP>__ICON_WIDTH`。給別的程式讀的變數例外。**改名不留舊名**（user 裁定）。

**怎麼改**：照上表改名與改語意；`internal/config/v2rename.go` 裡環境變數的 deprecation 提示與對應測試（`v2rename_test.go`、`config_test.go`、
`state_test.go`、`app_test.go` 的 `KM8__` 案例）一起拿掉（設定檔內容的 v2 改名遷移若還有別的用途，只拿掉環境變數那部分）。一起改的地方：
`.local/demos/` 的 tape（`demo-basics`、`demo-compare`、`demo-yaml-edit`、`demo-relatives`、`demo-helm`、`demo-namespace`、`demo-alterm` 設的是
檔案路徑，改成目錄；`kbu-demo-config` 那份照新語意放）、README 兩份的環境變數段、`docs/dev-remarks.md`。`demo-alterm.tape` 的註解還提到
`KBU__SHELL`（v1.7.5 的舊名），一起清掉。改完 `grep -rn 'KM8__\|KBU__CONFIGPATH\|KBU__STATEPATH\|KBU_TEST' .` 除了 CHANGELOG 舊段落是零。


## 已經符合、不用修的（對照 v0.1.21 的改動）

- **D6 `compositeDisp()` 不 panic**：kbu 照搬時就修了（起點取 0、超出切掉，`TestD6_CompositeDisp`），v0.1.21 就是照 kbu 寫的。
- **D5 選取模式的移動**：YAML viewer 的 visual 有 `h/j/k/l`、`w/b/e`、`0/$`、`gg/G`、`u/d`（`yamlpopup.go`）。


## 待確認

沒有。
