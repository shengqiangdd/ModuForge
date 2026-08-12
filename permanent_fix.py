"""Permanent fix: disable btrfs CoW + verify SQLite compatibility"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Step 1: Stop container
print('=== Step 1: Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Step 2: Check btrfs CoW status
print('\n=== Step 2: Check CoW status ===')
cmds = [
    'lsattr /vol1/docker/moduforge-data/ 2>&1',
    'lsattr /vol1/docker/moduforge-data/moduforge.db 2>&1',
    'sudo chattr +C /vol1/docker/moduforge-data 2>&1',
    'sudo chattr -C /vol1/docker/moduforge-data/moduforge.db 2>&1 || true',
    'lsattr /vol1/docker/moduforge-data/ 2>&1',
]
for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode(errors='replace').strip()
    if out: print(f'  {cmd[:60]}: {out}')

# Step 3: Verify current DB integrity
print('\n=== Step 3: Verify DB ===')
stdin, stdout, stderr = ssh.exec_command("""
python3 -c "
import sqlite3
conn = sqlite3.connect('/vol1/docker/moduforge-data/moduforge.db')
result = conn.execute('PRAGMA integrity_check').fetchone()
print(f'Integrity: {result[0]}')
for t in ['users', 'projects', 'ai_conversations', 'conversation_messages']:
    try:
        cnt = conn.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'  {t}: {cnt}')
    except Exception as e:
        print(f'  {t}: ERROR')
conn.close()
"
""")
print(stdout.read().decode(errors='replace'))

# Step 4: Check if there are any other DB files that might be newer
print('\n=== Step 4: All DB files ===')
stdin, stdout, stderr = ssh.exec_command("""
find /vol1/docker/ -name "*.db" -size +1k -ls 2>/dev/null | sort -k9 -rn | head -10
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
