"""Check DB files properly"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check all backup files
print('=== Check backup files ===')
cmd = """
docker run --rm \
  -v /vol1/docker/volumes/moduforge_data/_data:/src:ro \
  python:3.11-alpine sh -c '
    pip install --quiet pysqlite3 2>/dev/null || true
    python3 -c "
import sqlite3, os, sys

files = [
    \"/src/moduforge.db\",
    \"/src/moduforge.db.bak\",
    \"/src/moduforge.db.pre_recovery.bak\",
    \"/src/moduforge_recovered.db\",
]

for f in files:
    if not os.path.exists(f):
        print(f\"SKIP: {f} (not found)\")
        continue
    size = os.path.getsize(f)
    print(f\"\\n=== {f} ({size:,} bytes) ===\")
    try:
        conn = sqlite3.connect(f)
        c = conn.cursor()
        result = c.execute(\"PRAGMA integrity_check\").fetchone()
        print(f\"  Integrity: {result[0]}\")
        if result[0] == \"ok\":
            for t in [\"users\", \"projects\", \"ai_conversations\", \"conversation_messages\"]:
                try:
                    cnt = c.execute(f\"SELECT COUNT(*) FROM {t}\").fetchone()[0]
                    print(f\"  {t}: {cnt} rows\")
                except Exception as e:
                    print(f\"  {t}: {e}\")
        conn.close()
    except Exception as e:
        print(f\"  ERROR: {e}\")
"
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=60)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print('STDERR:', err[:500])

# Also check the current bind mount DB
print('\n=== Current bind mount DB ===')
cmd2 = """
docker run --rm \
  -v /vol1/docker/moduforge-data:/data:ro \
  python:3.11-alpine python3 -c "
import sqlite3
conn = sqlite3.connect('/data/moduforge.db')
c = conn.cursor()
result = c.execute('PRAGMA integrity_check').fetchone()
print(f'Integrity: {result[0]}')
if result[0] == 'ok':
    for t in ['users', 'projects', 'ai_conversations']:
        cnt = c.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'{t}: {cnt}')
conn.close()
"
"""
stdin, stdout, stderr = ssh.exec_command(cmd2, timeout=30)
print(stdout.read().decode(errors='replace'))

ssh.close()
