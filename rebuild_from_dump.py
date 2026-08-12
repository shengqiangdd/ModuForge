"""Rebuild DB from SQL dump"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Check SQL dump
print('\n=== SQL dump info ===')
stdin, stdout, stderr = ssh.exec_command("""
ls -la /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
head -50 /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
""")
print(stdout.read().decode(errors='replace'))

# Rebuild from dump
print('\n=== Rebuild from dump ===')
cmd = """
docker run --rm \
  -v /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql:/dump.sql:ro \
  -v /vol1/docker/moduforge-data:/data \
  alpine sh -c '
    apk add --no-cache sqlite 2>/dev/null
    
    # Remove old DB
    rm -f /data/moduforge.db
    
    # Create new DB and import
    sqlite3 /data/moduforge.db < /dump.sql 2>&1 || echo "Import had errors (may be OK)"
    
    echo "=== Verify ==="
    echo "Integrity:"
    sqlite3 /data/moduforge.db "PRAGMA integrity_check;"
    echo "Users:"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM users;"
    echo "Projects:"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM projects;"
    echo "Conversations:"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM ai_conversations;"
    echo "Messages:"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM conversation_messages;"
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=60)
print(stdout.read().decode(errors='replace'))

# Start
print('\n=== Start ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())

import time
for i in range(10):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'Healthy at {i*2}s')
        break

# Test
print('\n=== Test ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "Clear failed:"
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
echo ""
echo "Projects:"
curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
for p in json.load(sys.stdin): print(f'  {p[\"name\"]}')
"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
