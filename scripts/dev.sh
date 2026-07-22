#!/bin/sh
# ModuForge Lite 开发环境启动脚本
# 前置: Go 1.23+, Node.js 22+, pnpm

set -e

echo "=== ModuForge Lite Dev ==="

# 创建数据目录
mkdir -p data/storage

# 启动后端（热重载用 air，否则 go run）
cd backend
if command -v air >/dev/null 2>&1; then
    echo "Starting backend with air (hot reload)..."
    air -- \
        --port :8080 \
        --jwt_secret dev-secret-do-not-use-in-prod \
        --database_path ../data/moduforge.db \
        --storage_path ../data/storage &
else
    echo "Starting backend with go run..."
    go run ./cmd/moduforge &
fi
BACKEND_PID=$!
cd ..

# 启动前端（如果存在）
if [ -d frontend/src ]; then
    cd frontend
    if [ -f package.json ]; then
        echo "Starting frontend dev server..."
        pnpm dev &
        FRONTEND_PID=$!
    else
        echo "Frontend not initialized yet. Run: cd frontend && pnpm create vite . --template svelte-ts"
    fi
    cd ..
fi

echo ""
echo "=== ModuForge Lite is running ==="
echo "  API:      http://localhost:8080/api/v1"
echo "  Frontend: http://localhost:5173 (if started)"
echo ""
echo "Press Ctrl+C to stop"

trap "kill $BACKEND_PID $FRONTEND_PID 2>/dev/null; exit 0" INT TERM
wait
