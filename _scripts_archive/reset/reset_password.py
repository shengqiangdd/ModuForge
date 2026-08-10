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

# Check how auth.go hashes passwords
print('=== CHECK HASH METHOD ===')
run('grep -n "bcrypt\|argon\|scrypt\|hash\|Hash" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/service/auth.go 2>/dev/null | head -10')

# Reset admin password to 'admin123' using bcrypt (Go standard)
reset_script = r'''
import sqlite3, sys, subprocess, hashlib, base64, os

db_path = sys.argv[1]
db = sqlite3.connect(db_path)

# Use Go's bcrypt via a small Go script
# Or use Python bcrypt if available
try:
    import bcrypt
    new_hash = bcrypt.hashpw(b"admin123", bcrypt.gensalt()).decode()
    print("Using bcrypt:", new_hash[:20] + "...")
except ImportError:
    # Fallback: use hashlib with sha256 + salt (check what Go uses)
    print("bcrypt not available, checking Go auth code...")
    # Read the auth.go to see hash method
    import re
    with open("/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/service/auth.go") as f:
        content = f.read()
    # Find hash function
    matches = re.findall(r'(bcrypt|argon2|scrypt|sha256|md5).*?(?:Hash|hash|Password|password)', content, re.IGNORECASE)
    print("Hash methods found:", matches)
    sys.exit(1)

# Update admin password
db.execute("UPDATE users SET password_hash = ?, password_changed_at = datetime('now') WHERE username = 'admin'", (new_hash,))
print("Rows updated:", db.total_changes)
db.commit()

# Verify
cur = db.cursor()
cur.execute("SELECT username, password_hash FROM users WHERE username = 'admin'")
row = cur.fetchone()
print("Admin hash:", row[1][:30] + "..." if row else "NOT FOUND")

db.close()
print("Password reset complete!")
'''

run(f'cat > /tmp/reset_pw.py << \'ENDSCRIPT\'\n{reset_script}\nENDSCRIPT')
run(f'{SUDO} python3 /tmp/reset_pw.py {DB}')

# Restart
print('\n=== RESTART ===')
run(SUDO + ' docker restart moduforge')

import time
time.sleep(5)

# Test login
print('\n=== TEST LOGIN ===')
import urllib.request, json
data = json.dumps({"username": "admin", "password": "admin123"}).encode()
req = urllib.request.Request('http://192.168.2.9:8086/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
try:
    resp = urllib.request.urlopen(req)
    result = json.loads(resp.read().decode())
    print("LOGIN SUCCESS:", "token" in result)
    if "token" in result:
        print("Token:", result["token"][:30] + "...")
except Exception as e:
    print("Login error:", e)

ssh.close()
