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

# Git pull
print('=== GIT PULL ===')
run(f'cd {PROJ} && {SUDO} git -c safe.directory="*" pull origin main')

# Fix JWT_SECRET in compose on server
print('\n=== FIX JWT_SECRET ===')
run(f'{SUDO} grep JWT_SECRET {PROJ}/docker-compose.yml')

# Stop, recreate
print('\n=== STOP ===')
run(f'{SUDO} docker stop moduforge; {SUDO} docker rm moduforge')

print('\n=== UP ===')
run(f'cd {PROJ} && {SUDO} docker compose up -d --remove-orphans')

import time
time.sleep(5)

# Check logs
print('\n=== LOGS ===')
run(f'{SUDO} docker logs moduforge --tail 10 2>&1')

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
