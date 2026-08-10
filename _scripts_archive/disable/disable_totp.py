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

# Check admin TOTP status
print('=== ADMIN TOTP STATUS ===')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT username, totp_enabled, totp_secret FROM users;"')

# Disable TOTP for all users
print('\n=== DISABLE TOTP ===')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "UPDATE users SET totp_enabled = 0;"')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT username, totp_enabled FROM users;"')

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
