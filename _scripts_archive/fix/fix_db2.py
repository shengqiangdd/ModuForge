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
DB = '/data/moduforge.db'

# Install sqlite3 in container
print('=== INSTALL SQLITE ===')
run(SUDO + ' docker exec moduforge apk add --no-cache sqlite 2>&1')

# Check schema
print('\n=== SCHEMA ===')
run(SUDO + ' docker exec moduforge sqlite3 ' + DB + ' ".schema users"')

# Add missing column
print('\n=== ADD COLUMN ===')
run(SUDO + ' docker exec moduforge sqlite3 ' + DB + ' "ALTER TABLE users ADD COLUMN password_changed_at TEXT;" 2>&1')

# Check users
print('\n=== USERS ===')
run(SUDO + ' docker exec moduforge sqlite3 ' + DB + ' "SELECT id, username, email, role FROM users;" 2>&1')

# Check projects
print('\n=== PROJECTS ===')
run(SUDO + ' docker exec moduforge sqlite3 ' + DB + ' "SELECT id, name FROM projects;" 2>&1')

# Check all tables
print('\n=== TABLES ===')
run(SUDO + ' docker exec moduforge sqlite3 ' + DB + ' ".tables"')

# Restart
print('\n=== RESTART ===')
run(SUDO + ' docker restart moduforge')

import time
time.sleep(5)

# Verify
print('\n=== VERIFY SCHEMA ===')
run(SUDO + ' docker exec moduforge sqlite3 ' + DB + ' ".schema users"')

ssh.close()
print('\n=== DONE ===')
