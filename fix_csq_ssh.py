"""Fix csq data: reassign user_id in database via SSH (password auth)"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)
print('SSH connected!')

# Step 1: Stop container
print('\n=== Stop container ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())
print(stderr.read().decode())

# Step 2: Fix database
print('\n=== Fix database ===')
sql = """
import sqlite3
db = '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db'
conn = sqlite3.connect(db)
c = conn.cursor()

csq = 'a4c50d84-a58d-4fbc-a64d-adf93ca14446'
admin = 'fec17bd3-7610-4f2a-b157-24ee1e362d23'

# Check current state
print('BEFORE:')
for t in ['projects', 'ai_conversations', 'conversation_messages', 'build_tasks']:
    try:
        cnt = c.execute(f"SELECT COUNT(*) FROM {t} WHERE user_id=?", (csq,)).fetchone()[0]
        print(f'  csq {t}: {cnt}')
        cnt = c.execute(f"SELECT COUNT(*) FROM {t} WHERE user_id=?", (admin,)).fetchone()[0]
        print(f'  admin {t}: {cnt}')
    except: pass

# Reassign from admin to csq
for t in ['projects', 'ai_conversations', 'conversation_messages', 'build_tasks']:
    try:
        c.execute(f"UPDATE {t} SET user_id=? WHERE user_id=?", (csq, admin))
        print(f'  Updated {t}: {c.rowcount} rows')
    except Exception as e:
        print(f'  Skip {t}: {e}')

conn.commit()

print()
print('AFTER:')
for t in ['projects', 'ai_conversations', 'conversation_messages', 'build_tasks']:
    try:
        cnt = c.execute(f"SELECT COUNT(*) FROM {t} WHERE user_id=?", (csq,)).fetchone()[0]
        print(f'  csq {t}: {cnt}')
    except: pass

conn.close()
print('Done!')
"""
stdin, stdout, stderr = ssh.exec_command(f"python3 -c '{sql}'")
out = stdout.read().decode(errors='replace')
err = stderr.read().decode(errors='replace')
print(out)
if err: print('STDERR:', err)

# Step 3: Start container
print('\n=== Start container ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())
print(stderr.read().decode())

# Step 4: Wait for healthy
import time
print('\n=== Waiting for healthy ===')
for i in range(20):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    print(f'  {i*2}s: {health}')
    if '"ok"' in health:
        print('Container healthy!')
        break

# Step 5: Verify csq data
print('\n=== Verify csq data ===')
stdin, stdout, stderr = ssh.exec_command("""
python3 -c "
import urllib.request, json
# Login as csq
data = json.dumps({'username': 'csq', 'password': 'csq0216'}).encode()
req = urllib.request.Request('http://localhost:8086/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
token = json.loads(resp.read())['token']
headers = {'Authorization': f'Bearer {token}'}

# Check projects
req = urllib.request.Request('http://localhost:8086/api/v1/projects', headers=headers)
resp = urllib.request.urlopen(req)
projects = json.loads(resp.read())
print(f'csq projects: {len(projects)}')
for p in projects:
    print(f'  - {p[\"name\"]}')

# Check conversations
req = urllib.request.Request('http://localhost:8086/api/v1/ai/conversations', headers=headers)
resp = urllib.request.urlopen(req)
convs = json.loads(resp.read())
conv_list = convs.get('conversations', convs) if isinstance(convs, dict) else convs
print(f'csq conversations: {len(conv_list) if conv_list else 0}')
"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
print('\n=== ALL DONE ===')
