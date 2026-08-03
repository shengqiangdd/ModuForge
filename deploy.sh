#!/bin/bash
# ModuForge 部署脚本
# 用法: bash deploy.sh
set -e

echo "=== ModuForge 部署 ==="

# 检查 docker
if ! command -v docker &>/dev/null; then
  echo "错误: 未安装 Docker"
  exit 1
fi

# 检查 docker compose
if ! docker compose version &>/dev/null 2>&1; then
  echo "错误: 未安装 docker compose"
  exit 1
fi

# 停止旧容器 + 重新构建启动（一条命令搞定）
echo "1. 停止旧容器..."
docker compose down 2>/dev/null || true

echo "2. 重新构建并启动..."
docker compose up --build -d

# 等待健康检查
echo "3. 等待服务启动..."
sleep 5

# 检查状态
if curl -s http://localhost:8086/health | grep -q '"status":"ok"'; then
  echo "✅ 部署成功！服务运行在 http://localhost:8086"
else
  echo "⚠️ 服务可能还在启动中，请检查: docker compose logs -f"
fi

echo ""
echo "查看日志: docker compose logs -f"
echo "停止服务: docker compose down"
