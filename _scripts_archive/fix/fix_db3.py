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

# Use Python sqlite3 on the host to fix the database
print('=== FIX SCHEMA VIA PYTHON ===')
fix_sql = '''
import sqlite3
db = sqlite3.connect("{db}")
cur = db.cursor()

# Check current columns
cur.execute("PRAGMA table_info(users)")
cols = [r[1] for r in cur.fetchall()]
print("Current columns:", cols)

# Add password_changed_at if missing
if "password_changed_at" not in cols:
    db.execute("ALTER TABLE users ADD COLUMN password_changed_at TEXT")
    print("Added password_changed_at")
else:
    print("password_changed_at already exists")

# Check users
cur.execute("SELECT id, username, email, role FROM users")
users = cur.fetchall()
print(f"Users ({len(users)}):")
for u in users:
    print(f"  {u}")

# Check projects
try:
    cur.execute("SELECT id, name FROM projects")
    projects = cur.fetchall()
    print(f"Projects ({len(projects)}):")
    for p in projects:
        print(f"  {p}")
except Exception as e:
    print(f"Projects table error: {e}")

# Check all tables
cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
tables = [r[0] for r in cur.fetchall()]
print(f"Tables: {tables}")

db.commit()
db.close()
print("Schema fix complete!")
'''.format(db=DB)

run(SUDO + f" python3 -c '{fix_sql}'")

# Restart container
print('\n=== RESTART ===')
run(SUDO + ' docker restart moduforge')

import time
time.sleep(5)

# Verify
print('\n=== VERIFY ===')
verify_sql = '''
import sqlite3
db = sqlite3.connect("{db}")
cur = db.cursor()
cur.execute("PRAGMA table_info(users)")
cols = [r[1] for r in cur.fetchall()]
print("Users columns:", cols)
cur.execute("SELECT COUNT(*) FROM users")
print("User count:", cur.fetchone()[0])
db.close()
'''.format(db=DB)
run(SUDO + f" python3 -c '{verify_sql}'")

ssh.close()
print('\n=== DONE ===')
