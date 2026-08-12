"""Check csq user and all data in current DB - via SSH"""
import subprocess, sys

def ssh(cmd):
    """Execute command on remote server via SSH"""
    r = subprocess.run(
        ['python', '-c', f"""
import paramiko
k = paramiko.RSAKey.from_private_key_file(r'C:\\Users\\22875\\.ssh\\id_rsa')
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'root', pkey=k)
stdin, stdout, stderr = c.exec_command('{cmd}')
out = stdout.read().decode(errors='replace')
err = stderr.read().decode(errors='replace')
print(out)
if err: print('STDERR:', err)
c.close()
"""],
        capture_output=True, text=True, timeout=30
    )
    print(r.stdout)
    if r.stderr:
        print('ERROR:', r.stderr)

# Step 1: Find DB and query it
ssh("""python3 -c "
import sqlite3, os

# Find DB
data_dir = '/vol1/docker/volumes/moduforge_moduforge_data/_data'
db = os.path.join(data_dir, 'moduforge.db')
print(f'DB: {db} ({os.path.getsize(db):,} bytes)')

conn = sqlite3.connect(db)
c = conn.cursor()

# Users
print('\\n=== USERS ===')
for r in c.execute('SELECT id, username, email FROM users'):
    print(f'  {r}')

# csq projects
print('\\n=== csq PROJECTS ===')
for r in c.execute(\\\"SELECT id, user_id, name FROM projects WHERE user_id='a4c50d84-a58d-4fbc-a64d-adf93ca14446'\\\"):
    print(f'  {r}')

# All projects by user
print('\\n=== ALL PROJECTS by user ===')
for r in c.execute('SELECT user_id, COUNT(*) FROM projects GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} projects')

# csq conversations
print('\\n=== csq CONVERSATIONS ===')
for r in c.execute(\\\"SELECT id, user_id, title FROM ai_conversations WHERE user_id='a4c50d84-a58d-4fbc-a64d-adf93ca14446'\\\"):
    print(f'  {r}')

# All conversations by user
print('\\n=== ALL CONVERSATIONS by user ===')
for r in c.execute('SELECT user_id, COUNT(*) FROM ai_conversations GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} conversations')

# Build tasks
print('\\n=== BUILD TASKS by user ===')
for r in c.execute('SELECT user_id, COUNT(*) FROM build_tasks GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} builds')

conn.close()
" """)
