由 analysis 從 README 整理（2026-11-20），不是使用者的原話。

# kbu — KubeUI

終端機裡的一頁式 Kubernetes 工作區：左邊是資源種類，中間是游標所在那一種的資源，下面是游標那一列的一切（它的 log、
它連到哪些東西、它的 events），跟著叢集即時更新。Relatives 導覽、YAML diff 與內嵌的 shell 都是內建的；kbu 沒做的事，
用使用者慣用的工具在那個 shell 裡做。

## 給誰用

在終端機裡管 Kubernetes 叢集的人：要看清楚資源之間怎麼連、出了什麼問題，偶爾改一個、刪一個、進 container 看看，
不想離開終端機、也不想記一長串 kubectl。

## 它能做什麼

- 找到路：每個資源列出它連到的東西（owner、Service 選到的 Pod、HPA 擴縮的 workload、掛載 PVC 的 Pod、用到 ConfigMap
  或 Secret 的 Pod），跟著連結走、退回來、把畫面切到走過的任何一點；workload 往下看它的 Pod、container；28 種內建資源
  加上叢集裡的 CRD；一次看好幾個 namespace；把常用的種類釘在最上面、排成自己要的順序；任何清單依一欄或好幾欄排序；
  打字篩選；離開再開，從上次停的地方接著用。
- 看仔細：Pod 的 log 跟著最新的一行，workload 的每個 Pod 的 log 合成一份；events；conditions；有行號、語法顏色、可以
  搜尋與選取複製的 YAML；比對同一種的兩個資源哪裡不同；狀態欄只在需要注意時上色；Helm release 的 manifest、values、
  notes、hooks、revision 歷史與 rollback；kubeconfig 的 context（憑證不顯示）。
- 把事情做完：edit（真正的 `kubectl edit`，用使用者的 editor）、shell 進 container、delete（一定先問）、住在 app 裡、
  藏起來還會繼續跑的 shell、複製到剪貼簿（ssh 與 tmux 裡也能用）、一個 panel 全螢幕。從 kbu 做的每一次 edit 與 delete
  都記下來。

## 平台

macOS、Linux（含 WSL）、Windows（Windows 上開不了內嵌的 terminal：Alterm、edit、shell 不能用）。要有 kubectl、一份
kubeconfig 與連得到的叢集、Nerd Font、truecolor 終端機；Helm 的部分要有 `helm`。

## 哪些事不能發生

- 在 kbu 裡切 context 不能動到 `~/.kube/config`：另一個終端機的 `kubectl` 不受影響。
- 刪東西一定先問；刪 namespace 要多一句警告。
- Helm chart 管理的物件不能被誤改。
- kubeconfig 的憑證不顯示。
- 使用者的設定、釘選與排序不能因為檔案壞了、或升級，就默默不見。
