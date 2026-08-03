#!/bin/bash
# ModuForge v4.1.0 Deploy Script
# WebSocket fix + Agent Self-Evolution + Presets

set -e

echo "=== ModuForge v4.1.0 Deploy ==="
echo "Features:"
echo "  1. WebSocket background tab fix (heartbeat + visibility API)"
echo "  2. Agent self-evolution system (3 new skills)"
echo "  3. Agent presets (5 built-in styles)"
echo "  4. Pattern learning for modules"
echo ""

# Build backend
echo "[1/3] Building backend..."
cd /app/working/workspaces/default/ModuForge/backend
export PATH="/usr/local/go/bin:$PATH"
go build -o moduforge ./cmd/server
echo "  ✓ Backend built"

# Build frontend
echo "[2/3] Building frontend..."
cd /app/working/workspaces/default/ModuForge/frontend
npm run build
echo "  ✓ Frontend built"

# Deploy
echo "[3/3] Deploying..."
cd /app/working/workspaces/default

# Stop existing containers
docker compose down 2>/dev/null || true

# Start new containers
docker compose up -d --build

echo ""
echo "=== Deploy Complete ==="
echo ""
echo "New Features:"
echo ""
echo "🔧 WebSocket Fix:"
echo "  - Heartbeat ping every 25s keeps connection alive"
echo "  - Page Visibility API detects tab background/foreground"
echo "  - Exponential backoff reconnect (1s → 15s max)"
echo "  - Backend read deadline extended to 5 minutes"
echo ""
echo "🧠 Agent Self-Evolution:"
echo "  - self_evolve: Learn from execution history"
echo "  - pattern_learn: Record and apply successful patterns"
echo "  - agent_preset: Manage agent styles and presets"
echo ""
echo "🎨 Agent Presets (QwenPaw-style):"
echo "  - CodePilot: 全能编程助手"
echo "  - Module Master: Android模块专家"
echo "  - Debugger: 调试诊断专家"
echo "  - Architect: 系统架构师"
echo "  - Teacher: 教学模式"
echo ""
echo "📊 Database Tables Added:"
echo "  - module_patterns: Pattern learning storage"
echo "  - agent_presets: Agent style presets"
echo ""
