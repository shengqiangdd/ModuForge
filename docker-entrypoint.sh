#!/bin/sh
set -e

log() {
  echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

# ── JWT_SECRET 优先级：环境变量 > .env 文件 > 自动生成并持久化 ──
if [ -n "$JWT_SECRET" ]; then
  log "Using JWT_SECRET from environment variable"
elif [ -f /data/.env ] && grep -q "^JWT_SECRET=." /data/.env 2>/dev/null; then
  # 从持久化卷读取之前生成的密钥
  export JWT_SECRET=$(grep "^JWT_SECRET=" /data/.env | head -1 | cut -d= -f2-)
  log "Loaded JWT_SECRET from /data/.env"
else
  # 首次启动：生成随机密钥并写入持久化卷，重启后不变
  JWT_SECRET=$(openssl rand -hex 32)
  export JWT_SECRET
  echo "JWT_SECRET=${JWT_SECRET}" >> /data/.env
  log "Generated random JWT_SECRET and saved to /data/.env"
fi

# ── 启动服务 ──
log "Starting ModuForge on port ${PORT:-:8080}..."

# Fix volume ownership if running as root (e.g. after a host-level backup/restore).
if [ "$(id -u)" = '0' ]; then
  chown -R moduforge:moduforge /data /app/uploads 2>/dev/null || true
  exec su -s /bin/sh -c 'exec "$0" "$@"' -- /app/server "$@"
else
  exec /app/server "$@"
fi
