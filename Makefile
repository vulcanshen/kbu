# kbu — build / run / test / package（家族共用的 target 在 family.mk）
#
# 跑 `make`（或 `make help`）列出所有指令。

BINARY := kbu
TAPES  := .local/demos/demo-basics.tape
include family.mk

##@ kbu 自己的
