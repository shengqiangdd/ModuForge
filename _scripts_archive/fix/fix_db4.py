import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(err)
    return out

SUDO = 'echo "csq0216" | sudo -S'
DB = '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db'

# Write fix script to temp file on server, then execute
fix_script = r'''
import sqlite3, sys
db_path = sys.argv[1]
db = sqlite3.connect(db_path)
cur = db.cursor()

cur.execute("PRAGMA table_info(users)")
cols = [r[1] for r in cur.fetchall()]
print("Current columns:", cols)

if "password_changed_at" not in cols:
    db.execute("ALTER TABLE users ADD COLUMN password_changed_at TEXT")
    print("Added password_changed_at")
else:
    print("password_changed_at already exists")

cur.execute("SELECT id, username, email, role FROM users")
users = cur.fetchall()
print("Users (%d):" % len(users))
for u in users:
    print("  %s" % str(u))

try:
    cur.execute("SELECT id, name FROM projects")
    projects = cur.fetchall()
    print("Projects (%d):" % len(projects))
    for p in projects:
        print("  %s" % str(p))
except Exception as e:
    print("Projects: %s" % e)

cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
tables = [r[0] for r in cur.fetchall()]
print("Tables:", tables)

db.commit()
db.close()
print("Done!")
'''

# Write script to server
run(f'cat > /tmp/fix_db.py << \'ENDSCRIPT\'\n{fix_script}\nENDSCRIPT')

# Execute
run(f'{SUDO} python3 /tmp/fix_db.py {DB}')

# Restart
print('\n=== RESTART ===')
run(SUDO + ' docker restart moduforge')

import time
time.sleep(5)

# Verify login
print('\n=== TEST LOGIN ===')
import urllib.request, json
data = json.dumps({"username": "admin", "password": "admin123"}).encode()
req = urllib.request.Request('http://192.168.2.9:8086/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
try:
    resp = urllib.request.urlopen(req)
    print(resp.read().decode())
except Exception as e:
    print("Error:", e)

ssh.close()
print('\n=== DONE ===')
