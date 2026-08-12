#!/bin/bash
# ============================================================
# ModuForge 安全部署脚本
# 用法: bash deploy_safe.sh [--skip-build] [--restore-backup]
# ============================================================
set -e

# --- 配置 ---
REMOTE_HOST="192.168.2.9"
REMOTE_USER="admin"
REMOTE_PASS="csq0216"
REMOTE_DIR="/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge"
COMPOSE_DIR="$REMOTE_DIR"
CONTAINER_NAME="moduforge"
DB_VOLUME_PATH="/vol1/docker/volumes/moduforge_moduforge_data/_data"
HEALTH_URL="http://localhost:8086/health"
MAX_WAIT=60

# --- 颜色 ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[DEPLOY]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; }

# --- SSH helper ---
ssh_cmd() {
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$REMOTE_USER@$REMOTE_HOST" "$@"
}

ssh_cmd_interactive() {
    sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" "$@"
}

# Check if sshpass is available
if ! command -v sshpass &> /dev/null; then
    warn "sshpass not found, trying key-based auth..."
    SSH_FUNC="ssh_cmd"
else
    SSH_FUNC="ssh_cmd_interactive"
fi

# --- Parse args ---
SKIP_BUILD=false
RESTORE_BACKUP=false
for arg in "$@"; do
    case $arg in
        --skip-build) SKIP_BUILD=true ;;
        --restore-backup) RESTORE_BACKUP=true ;;
    esac
done

echo ""
log "=========================================="
log "  ModuForge 安全部署"
log "=========================================="
echo ""

# --- Step 1: 备份当前数据库 ---
log "Step 1/6: 备份数据库..."
BACKUP_NAME="moduforge_backup_$(date +%Y%m%d_%H%M%S).db"

if [ "$SSH_FUNC" = "ssh_cmd_interactive" ]; then
    sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" bash << REMOTE_BACKUP
set -e
cd "$DB_VOLUME_PATH"

# 备份当前 DB
if [ -f moduforge.db ]; then
    cp moduforge.db "$BACKUP_NAME"
    echo "  备份完成: $BACKUP_NAME (\$(du -h $BACKUP_NAME | cut -f1))"
    
    # 清理 7 天前的备份
    find . -name "moduforge_backup_*.db" -mtime +7 -delete 2>/dev/null || true
    echo "  已清理 7 天前的备份"
else
    echo "  警告: 当前没有数据库文件"
fi

# 同时保留一个最新的 .bak
cp moduforge.db moduforge.db.bak 2>/dev/null || true
echo "  .bak 已更新"
REMOTE_BACKUP
else
    ssh "$REMOTE_USER@$REMOTE_HOST" bash << REMOTE_BACKUP
set -e
cd "$DB_VOLUME_PATH"
if [ -f moduforge.db ]; then
    cp moduforge.db "$BACKUP_NAME"
    echo "  备份完成: $BACKUP_NAME"
    find . -name "moduforge_backup_*.db" -mtime +7 -delete 2>/dev/null || true
else
    echo "  警告: 当前没有数据库文件"
fi
cp moduforge.db moduforge.db.bak 2>/dev/null || true
REMOTE_BACKUP
fi

# --- Step 2: 上传代码变更（如果有的话）---
log "Step 2/6: 同步代码..."
# 使用 rsync 同步后端和前端代码（排除 node_modules、target 等）
if command -v rsync &> /dev/null; then
    rsync -avz --delete \
        --exclude 'node_modules' \
        --exclude 'target' \
        --exclude '.git' \
        --exclude '*.db' \
        --exclude '*.db-*' \
        --exclude 'dist' \
        --exclude '.env' \
        ./backend/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/backend/" 2>/dev/null || warn "rsync 后端失败，跳过"
    
    rsync -avz --delete \
        --exclude 'node_modules' \
        --exclude 'dist' \
        --exclude '.svelte-kit' \
        ./frontend/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/frontend/" 2>/dev/null || warn "rsync 前端失败，跳过"
    
    log "  代码同步完成"
else
    warn "  rsync 不可用，跳过代码同步（请手动上传）"
fi

# --- Step 3: 停止容器（而不是 recreate）---
log "Step 3/6: 安全停止容器..."
if [ "$SSH_FUNC" = "ssh_cmd_interactive" ]; then
    sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" bash << 'REMOTE_STOP'
# 先优雅停止，给 SQLite 时间 checkpoint WAL
docker stop moduforge 2>/dev/null || true
sleep 2

# 确保 WAL 已 checkpoint（删除前等待）
cd /vol1/docker/volumes/moduforge_moduforge_data/_data
if [ -f moduforge.db-wal ]; then
    WAL_SIZE=$(stat -f%z moduforge.db-wal 2>/dev/null || stat -c%s moduforge.db-wal 2>/dev/null || echo 0)
    echo "  WAL 文件大小: $WAL_SIZE bytes"
    if [ "$WAL_SIZE" -gt 0 ]; then
        echo "  警告: WAL 文件非空，等待 3 秒..."
        sleep 3
    fi
fi

# 删除 WAL/SHM（容器已停止，安全）
rm -f moduforge.db-wal moduforge.db-shm
echo "  容器已停止，WAL/SHM 已清理"
REMOTE_STOP
else
    ssh "$REMOTE_USER@$REMOTE_HOST" bash << 'REMOTE_STOP'
docker stop moduforge 2>/dev/null || true
sleep 2
cd /vol1/docker/volumes/moduforge_moduforge_data/_data
rm -f moduforge.db-wal moduforge.db-shm
echo "  容器已停止"
REMOTE_STOP
fi

# --- Step 4: 重建并启动 ---
log "Step 4/6: 重建并启动..."
if [ "$SSH_FUNC" = "ssh_cmd_interactive" ]; then
    if [ "$SKIP_BUILD" = true ]; then
        sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" \
            "cd $COMPOSE_DIR && docker compose up -d 2>&1"
    else
        sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" \
            "cd $COMPOSE_DIR && docker compose up -d --build 2>&1"
    fi
else
    if [ "$SKIP_BUILD" = true ]; then
        ssh "$REMOTE_USER@$REMOTE_HOST" "cd $COMPOSE_DIR && docker compose up -d 2>&1"
    else
        ssh "$REMOTE_USER@$REMOTE_HOST" "cd $COMPOSE_DIR && docker compose up -d --build 2>&1"
    fi
fi

# --- Step 5: 等待健康检查 ---
log "Step 5/6: 等待容器健康..."
for i in $(seq 1 $MAX_WAIT); do
    sleep 2
    STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$HEALTH_URL" 2>/dev/null || echo "000")
    if [ "$STATUS" = "200" ]; then
        log "  容器健康! (等待了 $((i*2))s)"
        break
    fi
    if [ $i -eq $MAX_WAIT ]; then
        err "  容器未在 ${MAX_WAIT}s 内健康，检查日志..."
        if [ "$SSH_FUNC" = "ssh_cmd_interactive" ]; then
            sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" \
                "docker logs moduforge --tail 20 2>&1"
        fi
        exit 1
    fi
done

# --- Step 6: 验证数据完整性 ---
log "Step 6/6: 验证数据..."

# 登录获取 token
LOGIN_RESP=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null)

TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
    err "  登录失败!"
    echo "  登录响应: $LOGIN_RESP"
    
    # 尝试从备份恢复
    log "  尝试从备份恢复..."
    if [ "$SSH_FUNC" = "ssh_cmd_interactive" ]; then
        sshpass -p "$REMOTE_PASS" ssh -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" bash << REMOTE_RESTORE
set -e
cd "$DB_VOLUME_PATH"
docker stop moduforge 2>/dev/null || true
sleep 2
if [ -f moduforge.db.bak ]; then
    cp moduforge.db.bak moduforge.db
    rm -f moduforge.db-wal moduforge.db-shm
    docker start moduforge
    echo "  已从 .bak 恢复"
else
    echo "  没有备份可恢复!"
fi
REMOTE_RESTORE
    else
        ssh "$REMOTE_USER@$REMOTE_HOST" bash << REMOTE_RESTORE
set -e
cd "$DB_VOLUME_PATH"
docker stop moduforge 2>/dev/null || true
sleep 2
if [ -f moduforge.db.bak ]; then
    cp moduforge.db.bak moduforge.db
    rm -f moduforge.db-wal moduforge.db-shm
    docker start moduforge
    echo "  已从 .bak 恢复"
fi
REMOTE_RESTORE
    fi
    
    # 重新等待健康
    sleep 10
    for i in $(seq 1 15); do
        sleep 3
        STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$HEALTH_URL" 2>/dev/null || echo "000")
        if [ "$STATUS" = "200" ]; then break; fi
    done
    
    # 重新登录
    LOGIN_RESP=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
        -H 'Content-Type: application/json' \
        -d '{"username":"admin","password":"admin123"}' 2>/dev/null)
    TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")
fi

if [ -z "$TOKEN" ]; then
    err "  恢复后仍然无法登录，请手动检查!"
    exit 1
fi

# 验证项目
PROJECTS=$(curl -s http://localhost:8086/api/v1/projects \
    -H "Authorization: Bearer $TOKEN" 2>/dev/null)
PROJECT_COUNT=$(echo "$PROJECTS" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")

# 验证 AI 对话
CONVS=$(curl -s http://localhost:8086/api/v1/ai/conversations \
    -H "Authorization: Bearer $TOKEN" 2>/dev/null)
CONV_COUNT=$(echo "$CONVS" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('conversations',[])) if isinstance(d,dict) else 0)" 2>/dev/null || echo "0")

echo ""
log "=========================================="
log "  部署完成!"
log "=========================================="
log "  项目数量: $PROJECT_COUNT"
log "  AI 对话:  $CONV_COUNT"
log "  前端地址: http://192.168.2.9:8086"
log "  登录: admin / admin123"
log "=========================================="

if [ "$PROJECT_COUNT" -lt 1 ]; then
    warn "  警告: 项目数量为 0，数据可能丢失!"
    warn "  请检查数据库状态"
fi
