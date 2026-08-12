"""Full recovery: dump data from corrupted DB into clean new DB"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Write recovery script
recovery = """
import sqlite3, os

src_path = '/vol1/docker/moduforge-data/moduforge.db'
new_path = '/vol1/docker/moduforge-data/moduforge_new.db'

# Remove new DB if exists
if os.path.exists(new_path):
    os.remove(new_path)

# Open corrupted DB (read data despite corruption)
src = sqlite3.connect(src_path)
src.row_factory = sqlite3.Row

# Create clean new DB
dst = sqlite3.connect(new_path)
dst.execute('PRAGMA journal_mode=WAL')
dst.execute('PRAGMA synchronous=NORMAL')

# Get all tables and their schemas
tables = []
for row in src.execute("SELECT name, sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL AND name NOT LIKE 'sqlite_%' AND name != 'lost_and_found'"):
    tables.append((row[0], row[1]))

print(f'Found {len(tables)} tables')

# Copy data table by table
total = 0
for name, schema in tables:
    try:
        # Create table in new DB
        dst.execute(schema)
        
        # Copy data
        rows = list(src.execute(f'SELECT * FROM [{name}]'))
        if rows:
            # Get column names
            cols = [desc[0] for desc in src.execute(f'SELECT * FROM [{name}] LIMIT 0').description]
            placeholders = ','.join(['?' for _ in cols])
            col_names = ','.join([f'[{c}]' for c in cols])
            
            for row in rows:
                dst.execute(f'INSERT INTO [{name}] ({col_names}) VALUES ({placeholders})', list(row))
            
            total += len(rows)
            print(f'  {name}: {len(rows)} rows')
        else:
            print(f'  {name}: 0 rows')
    except Exception as e:
        print(f'  {name}: ERROR - {e}')

dst.commit()

# Create indexes
for row in src.execute("SELECT sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL AND name NOT LIKE 'sqlite_%'"):
    try:
        dst.execute(row[0])
    except:
        pass

dst.commit()

# Verify
c = dst.cursor()
result = c.execute('PRAGMA integrity_check').fetchone()
print(f'\\nNew DB integrity: {result[0]}')

for t in ['users', 'projects', 'ai_conversations', 'conversation_messages', 'build_tasks']:
    try:
        cnt = c.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'  {t}: {cnt}')
    except Exception as e:
        print(f'  {t}: {e}')

src.close()
dst.close()

# Replace
os.replace(new_path, src_path)
print(f'\\nReplaced! Final size: {os.path.getsize(src_path):,} bytes')
print('Recovery complete!')
"""

sftp = ssh.open_sftp()
with sftp.open('/tmp/recover_db.py', 'w') as f:
    f.write(recovery)
sftp.close()

print('\n=== Recover ===')
stdin, stdout, stderr = ssh.exec_command('python3 /tmp/recover_db.py 2>&1')
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
