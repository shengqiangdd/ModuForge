"""Fix: drop and recreate corrupted build_tasks table"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Use .bak file and fix build_tasks
print('\n=== Fix build_tasks ===')
cmd = """
# Copy the .bak file (health check passed)
cp /vol1/docker/volumes/moduforge_data/_data/moduforge.db.bak /vol1/docker/moduforge-data/moduforge.db

# Drop and recreate build_tasks using sqlite3 in a container
docker run --rm \
  -v /vol1/docker/moduforge-data:/data \
  alpine sh -c '
    apk add --no-cache sqlite 2>/dev/null
    echo "Before:"
    sqlite3 /data/moduforge.db "SELECT name FROM sqlite_master WHERE type=\"table\" AND name LIKE \"%build%\";"
    
    # Drop corrupted table
    sqlite3 /data/moduforge.db "DROP TABLE IF EXISTS build_tasks;"
    
    # Recreate with correct schema
    sqlite3 /data/moduforge.db "CREATE TABLE IF NOT EXISTS build_tasks (
      id TEXT PRIMARY KEY,
      project_id TEXT NOT NULL,
      user_id TEXT NOT NULL,
      status TEXT DEFAULT '\''pending'\'',
      mode TEXT DEFAULT '\''manual'\'',
      git_url TEXT,
      git_branch TEXT,
      commit_sha TEXT,
      log TEXT,
      error TEXT,
      started_at TEXT,
      completed_at TEXT,
      created_at TEXT DEFAULT CURRENT_TIMESTAMP,
      FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
    );"
    
    echo "After:"
    sqlite3 /data/moduforge.db "SELECT name FROM sqlite_master WHERE type=\"table\" AND name LIKE \"%build%\";"
    sqlite3 /data/moduforge.db "PRAGMA integrity_check;"
    
    echo "Data check:"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM users;"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM projects;"
    sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM ai_conversations;"
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=60)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print('STDERR:', err[:300])

# Start
print('\n=== Start ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())

import time
print('\n=== Wait ===')
for i in range(10):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'  {i*2}s: healthy!')
        break

# Test
print('\n=== Test clear failed ===')
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
