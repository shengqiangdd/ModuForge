#!/bin/bash
# ModuForge SQLite 数据库定时备份（WAL 安全快照 .backup API），保留最近 14 份。
# 部署位置: /vol1/1000/docker/qwenpaw/data/backups/moduforge-backup.sh
# crontab: 30 3 * * * /vol1/1000/docker/qwenpaw/data/backups/moduforge-backup.sh >> /vol1/1000/docker/qwenpaw/data/backups/moduforge-backup.log 2>&1
set -e

SRC=/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db
DEST=/vol1/1000/docker/qwenpaw/data/backups
STAMP=$(date +%Y%m%d-%H%M%S)

if [ ! -f "$SRC" ]; then
  echo "[moduforge-backup] ERROR: source DB not found: $SRC" >&2
  exit 1
fi

mkdir -p "$DEST"

# sqlite3 .backup 走 SQLite backup API，WAL 模式下也能拿到一致快照
sqlite3 "$SRC" ".backup '$DEST/moduforge-$STAMP.db'"

# 滚动清理：只保留最近 14 份
ls -t "$DEST"/moduforge-*.db 2>/dev/null | tail -n +15 | xargs -r rm -f

COUNT=$(ls "$DEST"/moduforge-*.db 2>/dev/null | wc -l)
echo "[moduforge-backup] $STAMP done ($COUNT backups kept)"
