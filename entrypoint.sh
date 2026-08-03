#!/bin/sh
set -e

DATA_DIR="/app/data"
DB_FILE="$DATA_DIR/moduforge.db"

# 确保数据目录存在
mkdir -p "$DATA_DIR"

# 如果数据库不存在，从默认位置复制
if [ ! -f "$DB_FILE" ]; then
  echo "[entrypoint] First run — initializing database..."
  if [ -f /app/default-data/moduforge.db ]; then
    cp /app/default-data/moduforge.db "$DB_FILE"
    cp /app/default-data/moduforge.db-shm "$DATA_DIR/" 2>/dev/null || true
    cp /app/default-data/moduforge.db-wal "$DATA_DIR/" 2>/dev/null || true
  fi
fi

# 确保权限正确
chmod -R 755 "$DATA_DIR" 2>/dev/null || true

# Generate ADB keys if missing (first run after volume mount)
if [ ! -f /root/.android/adbkey ]; then
  echo "[entrypoint] Generating ADB keys..."
  mkdir -p /root/.android
  adb start-server 2>/dev/null || true
fi
chmod 600 /root/.android/adbkey 2>/dev/null || true
chmod 644 /root/.android/adbkey.pub 2>/dev/null || true

echo "[entrypoint] Starting server..."
exec "$@"
