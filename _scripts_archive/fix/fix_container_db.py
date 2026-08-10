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

# Check what the container sees
print('=== CONTAINER DB CHECK ===')
# Check container mount
run(SUDO + ' docker inspect moduforge --format "{{range .Mounts}}{{.Source}} -> {{.Destination}}{{println}}{{end}}"')

# Check the actual DB inside container
print('\n=== DB IN CONTAINER ===')
# The container runs as 'app' user, but DB is at /data/moduforge.db
# Check if container has its own copy
run(SUDO + ' docker exec moduforge ls -la /data/ 2>&1')

# The issue might be that the container created a NEW empty db at startup
# Let's check the container's env
print('\n=== CONTAINER ENV ===')
run(SUDO + ' docker exec moduforge env | grep -i db')

# Check if there's a DB_PATH override
print('\n=== CONTAINER CMD ===')
run(SUDO + ' docker inspect moduforge --format "{{.Config.Cmd}}"')

# The real fix: copy the fixed DB into the container and restart
print('\n=== COPY DB TO CONTAINER ===')
run(SUDO + ' docker cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db moduforge:/data/moduforge.db')
run(SUDO + ' docker cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-shm moduforge:/data/moduforge.db-shm 2>/dev/null; true')
run(SUDO + ' docker cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-wal moduforge:/data/moduforge.db-wal 2>/dev/null; true')

# Fix permissions
run(SUDO + ' docker exec moduforge chown app:app /data/moduforge.db* 2>/dev/null; true')

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
