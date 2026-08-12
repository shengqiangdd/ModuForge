"""Fix csq data - separate DB fix step"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)
print('SSH connected!')

# Write fix script to server
fix_script = r'''
import sqlite3
db = "/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db"
conn = sqlite3.connect(db)
c = conn.cursor()

csq = "a4c50d84-a58d-4fbc-a64d-adf93ca14446"
admin = "fec17bd3-7610-4f2a-b157-24ee1e362d23"

print("BEFORE:")
for t in ["projects", "ai_conversations", "conversation_messages", "build_tasks"]:
    try:
        cnt = c.execute("SELECT COUNT(*) FROM " + t + " WHERE user_id=?", (csq,)).fetchone()[0]
        print("  csq " + t + ": " + str(cnt))
        cnt = c.execute("SELECT COUNT(*) FROM " + t + " WHERE user_id=?", (admin,)).fetchone()[0]
        print("  admin " + t + ": " + str(cnt))
    except Exception as e:
        print("  " + t + ": " + str(e))

for t in ["projects", "ai_conversations", "conversation_messages", "build_tasks"]:
    try:
        c.execute("UPDATE " + t + " SET user_id=? WHERE user_id=?", (csq, admin))
        print("  Updated " + t + ": " + str(c.rowcount) + " rows")
    except Exception as e:
        print("  Skip " + t + ": " + str(e))

conn.commit()

print("")
print("AFTER:")
for t in ["projects", "ai_conversations", "conversation_messages", "build_tasks"]:
    try:
        cnt = c.execute("SELECT COUNT(*) FROM " + t + " WHERE user_id=?", (csq,)).fetchone()[0]
        print("  csq " + t + ": " + str(cnt))
    except Exception as e:
        print("  " + t + ": " + str(e))

conn.close()
print("DB fix done!")
'''

# Upload script
sftp = ssh.open_sftp()
with sftp.open('/tmp/fix_csq.py', 'w') as f:
    f.write(fix_script)
sftp.close()
print('Script uploaded')

# Execute
stdin, stdout, stderr = ssh.exec_command('python3 /tmp/fix_csq.py 2>&1')
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print('STDERR:', err)

# Restart container to pick up DB changes
print('\n=== Restart container ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose restart 2>&1')
print(stdout.read().decode())
print(stderr.read().decode())

# Wait for healthy
import time
print('\n=== Waiting for healthy ===')
for i in range(15):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'  {i*2}s: healthy!')
        break
    print(f'  {i*2}s: {health}')

# Verify
print('\n=== Verify csq data ===')
stdin, stdout, stderr = ssh.exec_command("""python3 -c "
import urllib.request, json
data = json.dumps({'username': 'csq', 'password': 'csq0216'}).encode()
req = urllib.request.Request('http://localhost:8086/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
token = json.loads(resp.read())['token']
headers = {'Authorization': 'Bearer ' + token}
req = urllib.request.Request('http://localhost:8086/api/v1/projects', headers=headers)
resp = urllib.request.urlopen(req)
projects = json.loads(resp.read())
print('csq projects: ' + str(len(projects)))
for p in projects:
    print('  - ' + p['name'])
req = urllib.request.Request('http://localhost:8086/api/v1/ai/conversations', headers=headers)
resp = urllib.request.urlopen(req)
convs = json.loads(resp.read())
cl = convs.get('conversations', convs) if isinstance(convs, dict) else convs
print('csq conversations: ' + str(len(cl) if cl else 0))
" """)
print(stdout.read().decode(errors='replace'))

ssh.close()
print('\n=== ALL DONE ===')
