# ── ModuForge 构建与质量工具链 ──
# Go 二进制 + 前端静态文件 + 测试/lint/基准

APP_NAME := moduforge
VERSION  := $(shell git describe --tags --always --dirty 2>nul || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
BUILD    := go build -ldflags="$(LDFLAGS)" -trimpath

.PHONY: build clean dev release size test lint bench all

# ── 构建 ──

## 开发构建（快速，保留 debug info）
dev:
	cd backend && go build -o ../bin/$(APP_NAME).exe .

## 生产构建（strip + trimpath，最小二进制）
build:
	cd backend && $(BUILD) -o ../bin/$(APP_NAME).exe .

## 带 UPX 压缩的发布构建（需要安装 UPX）
release: build
	@where upx >nul 2>&1 && upx --best --lzma bin/$(APP_NAME).exe || echo "UPX not found, skipping compression"

## 查看产物大小
size:
	@echo === Backend ===
	@for %%f in (bin\*.exe) do @echo %%f: %%~zf bytes
	@echo === Frontend ===
	@for %%f in (frontend\dist\**\*) do @echo %%f: %%~zf bytes

## 清理编译产物
clean:
	cd backend && go clean -cache -testcache
	@if exist bin\$(APP_NAME).exe del /q bin\$(APP_NAME).exe
	@echo Build artifacts cleaned.

# ── 质量 ──

## 运行全部后端测试（含竞态检测）
test:
	cd backend && go test -v -race -count=1 ./internal/...

## 运行 lint（staticcheck）
lint:
	cd backend && staticcheck ./...

## 列出所有基准测试
bench-list:
	cd backend && go test -C internal/service -bench=. -list=".*" -timeout 10s

## 全部质量检查：测试 + lint + 构建
all: test lint build
	@echo "=== All checks passed ==="
