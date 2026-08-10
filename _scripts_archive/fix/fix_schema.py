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
PROJ = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'
MOUNTPOINT = '/vol1/docker/volumes/moduforge_moduforge_data/_data'

# 1. Check current users table schema
print('=== CURRENT SCHEMA ===')
run(f'{SUDO} sqlite3 {MOUNTPOINT}/moduforge.db ".schema users"')

# 2. Check all tables
print('\n=== ALL TABLES ===')
run(f'{SUDO} sqlite3 {MOUNTPOINT}/moduforge.db ".tables"')

# 3. Check what columns are missing by looking at the new migration
print('\n=== NEW MIGRATION CHECK ===')
# Look for password_changed_at in the Go source
run(f'grep -r "password_changed_at" {PROJ}/backend/ 2>/dev/null')

# 4. Try to add missing column
print('\n=== ADD MISSING COLUMNS ===')
# Add password_changed_at if missing
run(f'{SUDO} sqlite3 {MOUNTPOINT}/moduforge.db "ALTER TABLE users ADD COLUMN password_changed_at DATETIME DEFAULT NULL;" 2>&1 || echo "Column may already exist"')

# Check for other missing columns by looking at the full schema
print('\n=== FULL SCHEMA AFTER FIX ===')
run(f'{SUDO} sqlite3 {MOUNTPOINT}/moduforge.db ".schema users"')

# 5. Check if there are more missing columns by trying to register
print('\n=== TEST REGISTER ===')
# First check what other tables might have issues
run(f'{SUDO} sqlite3 {MOUNTPOINT}/moduforge.db ".schema" 2>&1 | grep -i "CREATE TABLE" || true')

ssh.close()
