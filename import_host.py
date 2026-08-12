"""Import SQL dump using host Python via SSH"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Copy SQL dump to host
print('=== Copy dump to host ===')
stdin, stdout, stderr = ssh.exec_command('cp /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql /tmp/moduforge_dump.sql && wc -l /tmp/moduforge_dump.sql')
print(stdout.read().decode())

# Write import script
import_script = """
import sqlite3
import sys

# Remove old DB
import os
db_path = '/vol1/docker/moduforge-data/moduforge.db'
if os.path.exists(db_path):
    os.remove(db_path)

# Create new DB
conn = sqlite3.connect(db_path)
conn.execute('PRAGMA journal_mode=WAL')
conn.execute('PRAGMA synchronous=NORMAL')

# Read and execute dump
with open('/tmp/moduforge_dump.sql', 'r', encoding='utf-8') as f:
    sql = f.read()

conn.executescript(sql)
conn.commit()

# Verify
c = conn.cursor()
print('Integrity:', c.execute('PRAGMA integrity_check').fetchone()[0])
print('Users:', c.execute('SELECT COUNT(*) FROM users').fetchone()[0])
print('Projects:', c.execute('SELECT COUNT(*) FROM projects').fetchone()[0])
print('Conversations:', c.execute('SELECT COUNT(*) FROM ai_conversations').fetchone()[0])
print('Messages:', c.execute('SELECT COUNT(*) FROM conversation_messages').fetchone()[0])

tables = [r[0] for r in c.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()]
print(f'Tables: {len(tables)}')

conn.close()
print('Import complete!')
"""

sftp = ssh.open_sftp()
with sftp.open('/tmp/import_db.py', 'w') as f:
    f.write(import_script)
sftp.close()

# Run import
print('\n=== Import ===')
stdin, stdout, stderr = ssh.exec_command('python3 /tmp/import_db.py 2>&1')
print(stdout.read().decode(errors='replace'))

# Start container
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
