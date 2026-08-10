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

# Install bcrypt
print('=== INSTALL BCRYPT ===')
run(SUDO + ' pip3 install --break-system-packages bcrypt 2>&1')

# Write reset script to file
reset_script = r"""import sqlite3, bcrypt
db = sqlite3.connect('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
pw_hash = bcrypt.hashpw(b'admin123', bcrypt.gensalt()).decode()
db.execute("UPDATE users SET password_hash = ?, password_changed_at = datetime('now') WHERE username = 'admin'", (pw_hash,))
db.commit()
cur = db.cursor()
cur.execute("SELECT username, password_hash FROM users WHERE username = 'admin'")
row = cur.fetchone()
print("Updated:", row[0], row[1][:30] + "...")
db.close()
"""

run(f'cat > /tmp/reset_pw.py << \'EOF\'\n{reset_script}\nEOF')
print('\n=== RESET PASSWORD ===')
run(SUDO + ' python3 /tmp/reset_pw.py')

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
    print("LOGIN SUCCESS!")
    if "token" in result:
        print("Token:", result["token"][:40] + "...")
except Exception as e:
    print("Login error:", e)

ssh.close()
