#!/bin/bash
# ============================================================
# ModuForge 快速备份脚本
# 用法: bash backup_db.sh [remote_host]
# ============================================================
set -e

REMOTE_HOST="${1:-192.168.2.9}"
REMOTE_USER="admin"
DB_PATH="/vol1/docker/volumes/moduforge_moduforge_data/_data"
BACKUP_DIR="./backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

mkdir -p "$BACKUP_DIR"

echo "[BACKUP] 备份 $REMOTE_HOST 的 ModuForge 数据库..."

# 复制 DB
scp "$REMOTE_USER@$REMOTE_HOST:$DB_PATH/moduforge.db" "$BACKUP_DIR/moduforge_$TIMESTAMP.db"
echo "[BACKUP] 已保存: $BACKUP_DIR/moduforge_$TIMESTAMP.db"

# 显示大小
ls -lh "$BACKUP_DIR/moduforge_$TIMESTAMP.db"

# 清理 30 天前的备份
find "$BACKUP_DIR" -name "moduforge_*.db" -mtime +30 -delete 2>/dev/null || true
echo "[BACKUP] 已清理 30 天前的备份"
echo "[BACKUP] 完成!"
