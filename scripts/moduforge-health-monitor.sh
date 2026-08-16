#!/bin/bash
# ModuForge 健康监控：每 5 分钟检查 /health 与容器状态，连续失败 3 次生成告警标记。
# 部署位置: /vol1/1000/docker/qwenpaw/data/backups/moduforge-health-monitor.sh
# crontab: */5 * * * * /vol1/1000/docker/qwenpaw/data/backups/moduforge-health-monitor.sh >> /vol1/1000/docker/qwenpaw/data/backups/moduforge-health-monitor.log 2>&1

STATE_FILE=/vol1/1000/docker/qwenpaw/data/backups/.moduforge-health-failures
ALERT_FILE=/vol1/1000/docker/qwenpaw/data/backups/moduforge-ALERT.txt
FAILURES=0
REASON=""

# 1) HTTP health check
HTTP=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 http://localhost:8086/health 2>/dev/null)
if [ "$HTTP" != "200" ]; then
  REASON="HTTP /health -> $HTTP"
fi

# 2) Container status
if ! docker ps --filter name=moduforge --filter health=healthy --format "{{.Names}}" | grep -q moduforge; then
  if docker ps --filter name=moduforge --format "{{.Names}}" | grep -q moduforge; then
    REASON="$REASON; container not healthy"
  else
    REASON="$REASON; container not running"
  fi
fi

if [ -n "$REASON" ]; then
  FAILURES=$(cat "$STATE_FILE" 2>/dev/null || echo 0)
  FAILURES=$((FAILURES + 1))
  echo "$FAILURES" > "$STATE_FILE"
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] FAIL #$FAILURES: $REASON"
  if [ "$FAILURES" -ge 3 ]; then
    echo "⚠️  ModuForge 服务异常（连续 ${FAILURES} 次检查失败，最近一次 $(date '+%Y-%m-%d %H:%M:%S')）：${REASON}" > "$ALERT_FILE"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ALERT written: $ALERT_FILE"
  fi
else
  if [ -f "$STATE_FILE" ]; then
    rm -f "$STATE_FILE"
    [ -f "$ALERT_FILE" ] && mv "$ALERT_FILE" "$ALERT_FILE.recovered"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] recovered, alert cleared"
  fi
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] OK"
fi
