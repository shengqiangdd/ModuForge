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

# Use container's sqlite3 to fix schema
print('=== FIX SCHEMA IN CONTAINER ===')
run(f'{SUDO} docker exec moduforge apk add --no-cache sqlite 2>&1 || true')

print('\n=== CURRENT SCHEMA ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db ".schema users"')

print('\n=== ADD PASSWORD_CHANGED_AT ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db "ALTER TABLE users ADD COLUMN password_changed_at TEXT;" 2>&1 || echo "Column may already exist"')

print('\n=== CHECK OTHER TABLES ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db ".tables"')

# Check for any other missing columns
print('\n=== FULL SCHEMA ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db ".schema"')

# Count users
print('\n=== USER COUNT ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM users;" 2>&1 || echo "No users table or empty")

# List users
print('\n=== USERS ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db "SELECT id, username, email, role FROM users;" 2>&1 || echo "No users"')

# Count projects
print('\n=== PROJECTS ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db "SELECT COUNT(*) FROM projects;" 2>&1 || echo "No projects table or empty")

# Restart container to apply migration
print('\n=== RESTART CONTAINER ===')
run(f'{SUDO} docker restart moduforge')

import time
time.sleep(5)

# Verify
print('\n=== VERIFY SCHEMA ===')
run(f'{SUDO} docker exec moduforge sqlite3 /data/moduforge.db ".schema users"')

ssh.close()
print('\n=== DONE ===')
