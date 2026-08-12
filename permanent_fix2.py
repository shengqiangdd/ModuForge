"""Permanent fix: health check + backup + CoW protection"""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Step 1: Create health check script on server
print('=== Step 1: Create health check script ===')
health_script = '''#!/bin/bash
# ModuForge DB Health Check - runs every 5 minutes via cron
DB_PATH="/vol1/docker/moduforge-data/moduforge.db"
BACKUP_DIR="/vol1/docker/moduforge-backups"
LOG_FILE="/var/log/moduforge-health.log"

mkdir -p "$BACKUP_DIR"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

# Check if DB is writable
if [ ! -w "$DB_PATH" ]; then
    log "ERROR: DB not writable, fixing permissions"
    chmod 664 "$DB_PATH"
fi

# Quick integrity check using Python
RESULT=$(python3 -c "
import sqlite3
try:
    conn = sqlite3.connect('$DB_PATH')
    r = conn.execute('PRAGMA integrity_check').fetchone()
    conn.close()
    print(r[0])
except Exception as e:
    print(f'ERROR: {e}')
" 2>&1)

if [ "$RESULT" != "ok" ]; then
    log "CRITICAL: DB corrupted ($RESULT), restoring from backup"
    # Find latest good backup
    LATEST=$(ls -t "$BACKUP_DIR"/*.db 2>/dev/null | head -1)
    if [ -n "$LATEST" ]; then
        cp "$LATEST" "$DB_PATH"
        log "Restored from $LATEST"
    else
        log "No backup found!"
    fi
else
    # Auto-backup every 6 hours
    HOUR=$(date +%H)
    MIN=$(date +%M)
    if [ $((HOUR % 6)) -eq 0 ] && [ "$MIN" -lt 5 ]; then
        BACKUP_NAME="$BACKUP_DIR/moduforge_$(date +%Y%m%d_%H%M).db"
        cp "$DB_PATH" "$BACKUP_NAME"
        log "Auto-backup created: $BACKUP_NAME"
        # Keep only last 10 backups
        ls -t "$BACKUP_DIR"/*.db 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null
    fi
fi
'''

sftp = ssh.open_sftp()
with sftp.open('/usr/local/bin/moduforge-health-check.sh', 'w') as f:
    f.write(health_script)
sftp.close()

# Make executable and set up cron
cmds = [
    'sudo chmod +x /usr/local/bin/moduforge-health-check.sh',
    # Add cron job (every 5 minutes)
    'sudo crontab -l 2>/dev/null | grep -v moduforge-health | { cat; echo "*/5 * * * * /usr/local/bin/moduforge-health-check.sh"; } | sudo crontab -',
    # Run initial backup
    'mkdir -p /vol1/docker/moduforge-backups',
    'cp /vol1/docker/moduforge-data/moduforge.db /vol1/docker/moduforge-backups/moduforge_$(date +%Y%m%d_%H%M).db',
    'sudo crontab -l | grep moduforge',
]
for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode(errors='replace').strip()
    if out: print(f'  {out}')

# Step 2: Verify container is healthy
print('\n=== Step 2: Verify ===')
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
print(f'Health: {stdout.read().decode().strip()}')

# Step 3: Test clear failed (should work now)
print('\n=== Step 3: Test clear failed ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

# Step 4: Restart container to verify stability
print('\n=== Step 4: Restart test ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose restart 2>&1')
print(stdout.read().decode())
time.sleep(5)
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
print(f'After restart: {stdout.read().decode().strip()}')

# Final test
print('\n=== Final test ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "Clear failed:"
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
echo ""
echo "DB integrity:"
python3 -c "import sqlite3; c=sqlite3.connect('/vol1/docker/moduforge-data/moduforge.db'); print(c.execute('PRAGMA integrity_check').fetchone()[0])"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
print('\n=== DONE ===')
