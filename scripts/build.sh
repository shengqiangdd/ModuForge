#!/bin/sh
# ModuForge Lite 生产构建脚本

set -e

echo "=== Building ModuForge Lite ==="

# 1. 构建前端（如果有）
if [ -d frontend/src ] && [ -f frontend/package.json ]; then
    echo "Building frontend..."
    cd frontend
    pnpm install
    pnpm build
    cd ..
    # 将前端产物复制到后端 embed 目录
    mkdir -p backend/dist
    cp -r frontend/dist/* backend/dist/
fi

# 2. 构建后端
echo "Building backend..."
cd backend
CGO_ENABLED=0 go build -ldflags="-s -w" -o moduforge ./cmd/moduforge
cd ..

echo ""
echo "=== Build complete ==="
echo "Binary: backend/moduforge"
echo "Size: $(ls -lh backend/moduforge | awk '{print $5}')"
echo ""
echo "Run: cd backend && ./moduforge"
