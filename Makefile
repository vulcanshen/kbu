# kbu — build / run / test
#
# 跑 `make`(或 `make help`)列出所有指令。
# unix-first(macOS + Linux),CGO_ENABLED=0 靜態編譯;跨平台 release 走 goreleaser。

BINARY  := kbu
PKG     := ./cmd

GOOS    := $(shell go env GOOS)
GOARCH  := $(shell go env GOARCH)
# 無 tag 時 fallback 到 short commit;-dirty 標未提交改動。strip tag 的 "v" 前綴對齊
# goreleaser(internal/version.Version 存 "2.0.2"、顯示時才加 "v";見 version.go)。
VERSION ?= $(shell v=$$(git describe --tags --always --dirty 2>/dev/null || echo dev); echo "$$v" | sed 's/^v//')
LDFLAGS := -s -w -X github.com/vulcanshen/kbu/internal/version.Version=$(VERSION)

.DEFAULT_GOAL := help

##@ 編譯 / 執行

.PHONY: build
build: ## 編譯本地執行檔 → ./kbu(CGO_ENABLED=0 靜態、-trimpath、strip、注入版本)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)
	@echo "built ./$(BINARY)  ($(VERSION) $(GOOS)/$(GOARCH))"

.PHONY: run
run: ## 本地跑 kbu TUI(go run;需真實終端機)
	go run $(PKG)

.PHONY: clean
clean: ## 移除 ./kbu
	rm -f $(BINARY)

##@ 測試 / 檢查

.PHONY: test
test: ## 跑所有測試(go test ./...)
	go test ./...

.PHONY: test-race
test-race: ## 帶 race detector 跑測試(每次 release 前跑一次)
	go test -race ./...

.PHONY: vet
vet: ## go vet ./...
	go vet ./...

.PHONY: fmt
fmt: ## gofmt -w(就地格式化 cmd/ internal/)
	gofmt -w cmd internal

##@ 其他

.PHONY: help
help: ## 顯示這份說明
	@awk 'BEGIN {FS = ":.*?## "} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5); next} \
		/^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
