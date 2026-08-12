"""Permanent fix - use exec_command for sudo operations"""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Step 1: Create health check script
print('=== Step 1: Create health check ===')
health_script = r'''#!/bin/bash
DB_PATH="/vol1/docker/moduforge-data/moduforge.db"
BACKUP_DIR="/vol1/docker/moduforge-backups"
LOG_FILE="/var/log/moduforge-health.log"
mkdir -p "$BACKUP_DIR"
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"; }
if [ ! -w "$DB_PATH" ]; then log "ERROR: DB not writable"; chmod 664 "$DB_PATH"; fi
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
    log "CRITICAL: DB corrupted ($RESULT)"
    LATEST=$(ls -t "$BACKUP_DIR"/*.db 2>/dev/null | head -1)
    if [ -n "$LATEST" ]; then cp "$LATEST" "$DB_PATH"; log "Restored from $LATEST"; fi
else
    HOUR=$(date +%H); MIN=$(date +%M)
    if [ $((HOUR % 6)) -eq 0 ] && [ "$MIN" -lt 5 ]; then
        cp "$DB_PATH" "$BACKUP_DIR/moduforge_$(date +%Y%m%d_%H%M).db"
        ls -t "$BACKUP_DIR"/*.db 2>/dev/null | tail -n +11 | xargs rm -f 2>/dev/null
    fi
fi
'''

# Write via exec_command (with sudo)
cmd = f"""
cat > /tmp/health_check.sh << 'HEALTHEOF'
{health_script}
HEALTHEOF
sudo cp /tmp/health_check.sh /usr/local/bin/moduforge-health-check.sh
sudo chmod +x /usr/local/bin/moduforge-health-check.sh
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print(err)

# Set up cron
print('\n=== Step 2: Set up cron ===')
cmds = [
    'sudo crontab -l 2>/dev/null | grep -v moduforge-health | { cat; echo "*/5 * * * * /usr/local/bin/moduforge-health-check.sh"; } | sudo crontab -',
    'mkdir -p /vol1/docker/moduforge-backups',
    'cp /vol1/docker/moduforge-data/moduforge.db /vol1/docker/moduforge-backups/moduforge_initial.db',
    'sudo crontab -l 2>/dev/null | grep moduforge',
]
for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode(errors='replace').strip()
    if out: print(f'  {out}')

# Step 3: Verify
print('\n=== Step 3: Verify ===')
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
print(f'Health: {stdout.read().decode().strip()}')

# Step 4: Test clear failed
print('\n=== Step 4: Test clear failed ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

# Step 5: Restart test
print('\n=== Step 5: Restart test ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose restart 2>&1')
print(stdout.read().decode())
time.sleep(5)
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
print(f'After restart: {stdout.read().decode().strip()}')

# Final
print('\n=== Final ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "Clear:"
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
echo ""
echo "Integrity:"
python3 -c "import sqlite3; c=sqlite3.connect('/vol1/docker/moduforge-data/moduforge.db'); print(c.execute('PRAGMA integrity_check').fetchone()[0])"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
print('\n=== DONE ===')
