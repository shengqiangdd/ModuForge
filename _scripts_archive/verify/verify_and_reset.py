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

# Verify password and reset if needed
script = r'''
import sqlite3, bcrypt
db = sqlite3.connect("/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db")
cur = db.cursor()
cur.execute("SELECT username, password_hash FROM users WHERE username = ?", ("admin",))
row = cur.fetchone()
if row:
    print("Username:", row[0])
    print("Hash:", row[1][:50] + "...")
    ok = bcrypt.checkpw(b"admin123", row[1].encode())
    print("admin123 matches:", ok)
    if not ok:
        # Reset password
        new_hash = bcrypt.hashpw(b"admin123", bcrypt.gensalt()).decode()
        db.execute("UPDATE users SET password_hash = ?, password_changed_at = datetime('now') WHERE username = 'admin'", (new_hash,))
        db.commit()
        # Verify
        cur.execute("SELECT password_hash FROM users WHERE username = 'admin'")
        row2 = cur.fetchone()
        ok2 = bcrypt.checkpw(b"admin123", row2[0].encode())
        print("After reset, admin123 matches:", ok2)
else:
    print("Admin not found")
db.close()
'''

run(f'cat > /tmp/verify.py << \'EOF\'\n{script}\nEOF')
run(SUDO + ' python3 /tmp/verify.py')

# Restart to pick up changes
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
    print("LOGIN SUCCESS!")
    if "token" in result:
        print("Token:", result["token"][:40] + "...")
except Exception as e:
    print("Login error:", e)

ssh.close()
