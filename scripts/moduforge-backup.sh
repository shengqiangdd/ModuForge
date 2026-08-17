#!/bin/bash
# ModuForge SQLite 数据库定时备份（WAL 安全快照 .backup API），保留最近 14 份。
# 备份后立即做 PRAGMA integrity_check，坏备份直接删除（备份不可恢复 = 没有备份）。
# 部署位置: /vol1/1000/docker/qwenpaw/data/backups/moduforge-backup.sh
# crontab: 30 3 * * * /vol1/1000/docker/qwenpaw/data/backups/moduforge-backup.sh >> /vol1/1000/docker/qwenpaw/data/backups/moduforge-backup.log 2>&1
set -e

# 备份含完整数据库（用户消息、会话、token 密文）——仅所有者可读。
umask 077

SRC=/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db
DEST=/vol1/1000/docker/qwenpaw/data/backups
STAMP=$(date +%Y%m%d-%H%M%S)
OUT="$DEST/moduforge-$STAMP.db"

if [ ! -f "$SRC" ]; then
  echo "[moduforge-backup] ERROR: source DB not found: $SRC" >&2
  exit 1
fi

mkdir -p "$DEST"
chmod 700 "$DEST" 2>/dev/null || true

# sqlite3 .backup 走 SQLite backup API，WAL 模式下也能拿到一致快照
sqlite3 "$SRC" ".backup '$OUT'"

# 完整性验证：坏备份立即删除并退出非 0
CHECK=$(sqlite3 "$OUT" "PRAGMA integrity_check;" 2>&1 | head -1)
if [ "$CHECK" != "ok" ]; then
  rm -f "$OUT"
  echo "[moduforge-backup] ERROR: integrity_check failed ($CHECK), bad backup deleted: $OUT" >&2
  exit 1
fi

# 滚动清理：只保留最近 14 份（已验证的）
ls -t "$DEST"/moduforge-*.db 2>/dev/null | tail -n +15 | xargs -r rm -f

COUNT=$(ls "$DEST"/moduforge-*.db 2>/dev/null | wc -l)
echo "[moduforge-backup] $STAMP done, integrity ok ($COUNT backups kept)"
