"""Import SQL dump using Python"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop container
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Use Python to import
print('\n=== Import with Python ===')
cmd = """
docker run --rm \
  -v /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql:/dump.sql:ro \
  -v /vol1/docker/moduforge-data:/data \
  python:3.11-alpine python3 -c "
import sqlite3

# Create new DB
conn = sqlite3.connect('/data/moduforge.db')
conn.execute('PRAGMA journal_mode=WAL')
conn.execute('PRAGMA synchronous=NORMAL')

# Read and execute dump
with open('/dump.sql', 'r') as f:
    sql = f.read()

# Execute in batches
conn.executescript(sql)
conn.commit()

# Verify
c = conn.cursor()
print('Integrity:', c.execute('PRAGMA integrity_check').fetchone()[0])
print('Users:', c.execute('SELECT COUNT(*) FROM users').fetchone()[0])
print('Projects:', c.execute('SELECT COUNT(*) FROM projects').fetchone()[0])
print('Conversations:', c.execute('SELECT COUNT(*) FROM ai_conversations').fetchone()[0])
print('Messages:', c.execute('SELECT COUNT(*) FROM conversation_messages').fetchone()[0])
print('Build tasks:', c.execute('SELECT COUNT(*) FROM build_tasks').fetchone()[0])

# List tables
tables = [r[0] for r in c.execute(\"SELECT name FROM sqlite_master WHERE type='table'\").fetchall()]
print(f'Tables ({len(tables)}): {tables[:10]}...')

conn.close()
print('Done!')
"
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=120)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print('STDERR:', err[:500])

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
