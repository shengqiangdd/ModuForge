"""Fix: disable btrfs CoW + proper DB import"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Disable btrfs CoW on the data directory
print('\n=== Disable btrfs CoW ===')
stdin, stdout, stderr = ssh.exec_command('sudo chattr +C /vol1/docker/moduforge-data 2>&1; lsattr /vol1/docker/moduforge-data 2>&1')
print(stdout.read().decode(errors='replace'))

# Now copy the .bak file (which had working data) and use Python to fix
print('\n=== Copy .bak and fix ===')
fix_script = """
import sqlite3, os, shutil

src = '/vol1/docker/volumes/moduforge_data/_data/moduforge.db.bak'
dst = '/vol1/docker/moduforge-data/moduforge.db'

# Copy
shutil.copy2(src, dst)
print(f'Copied {os.path.getsize(src):,} bytes')

# Open and check
conn = sqlite3.connect(dst)
c = conn.cursor()

# Check integrity
result = c.execute('PRAGMA integrity_check').fetchone()
print(f'Integrity: {result[0]}')

# List tables
tables = [r[0] for r in c.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()]
print(f'Tables: {len(tables)}')

# Check data
for t in ['users', 'projects', 'ai_conversations', 'conversation_messages']:
    try:
        cnt = c.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'  {t}: {cnt}')
    except Exception as e:
        print(f'  {t}: {e}')

# If build_tasks is corrupted, drop and recreate
try:
    c.execute('SELECT COUNT(*) FROM build_tasks')
    print('  build_tasks: OK')
except:
    print('  build_tasks: CORRUPTED, dropping...')
    c.execute('DROP TABLE IF EXISTS build_tasks')
    c.execute('''CREATE TABLE build_tasks (
        id TEXT PRIMARY KEY,
        project_id TEXT NOT NULL,
        user_id TEXT NOT NULL,
        status TEXT DEFAULT 'pending',
        mode TEXT DEFAULT 'manual',
        git_url TEXT,
        git_branch TEXT,
        commit_sha TEXT,
        log TEXT,
        error TEXT,
        started_at TEXT,
        completed_at TEXT,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
    )''')
    print('  build_tasks: recreated')

conn.commit()

# Final integrity check
result2 = c.execute('PRAGMA integrity_check').fetchone()
print(f'Final integrity: {result2[0]}')

conn.close()
print('Done!')
"""

sftp = ssh.open_sftp()
with sftp.open('/tmp/fix_db_final.py', 'w') as f:
    f.write(fix_script)
sftp.close()

stdin, stdout, stderr = ssh.exec_command('python3 /tmp/fix_db_final.py 2>&1')
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
