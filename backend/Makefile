# ModuForge backend 验证命令（容器内执行，宿主机无需 Go）
#
# 用法：
#   make test              # 跑全部后端测试（首次较慢，之后走缓存秒级）
#   make test PKG=./internal/handler/   # 指定包
#   make test RUN=TestLogSink           # 指定测试名（可配合 PKG）
#   make build             # 编译 cmd/moduforge，验证可构建
#
# 缓存：GOCACHE/GOMODCACHE 挂载到宿主机持久目录，避免每次重复编译/下载。

GO_IMAGE  ?= golang:1.25-alpine
CACHE_DIR ?= /vol1/1000/docker/qwenpaw/data/gocache
# 容器内后端源码路径（与 go.mod module 名一致）
SRC_DIR   := /go/src/github.com/moduforge/backend

RUN_OPTS := --rm -e CGO_ENABLED=1 \
	-e GOCACHE=/cache/build \
	-e GOMODCACHE=/cache/mod \
	-v $(CURDIR):$(SRC_DIR) \
	-v $(CACHE_DIR):/cache \
	-w $(SRC_DIR)

# 默认跑全部包；可用 PKG=./internal/handler/ 指定
PKG ?= ./...

.PHONY: test build

test:
	@mkdir -p $(CACHE_DIR)
	@echo "==> go test $(PKG) $(if $(RUN),-run $(RUN),)"
	@docker run $(RUN_OPTS) $(GO_IMAGE) sh -e -c \
		'apk add --no-cache gcc musl-dev >/dev/null 2>&1; \
		 go test $(PKG) $(if $(RUN),-run "$(RUN)",) -count=1 -v 2>&1 | grep -viE "go: downloading"'

build:
	@mkdir -p $(CACHE_DIR)
	@echo "==> go build ./cmd/moduforge/"
	@docker run $(RUN_OPTS) $(GO_IMAGE) sh -e -c \
		'apk add --no-cache gcc musl-dev >/dev/null 2>&1; \
		 go build -o /tmp/server ./cmd/moduforge/ && echo BUILD_OK'
