import paramiko
import time
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=300):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(err)
    return out

# 1. Stop and remove old container + orphan
print('=== STOP OLD ===')
run('echo csq0216 | sudo -S docker stop moduforge-app-1 2>/dev/null; echo csq0216 | sudo -S docker rm moduforge-app-1 2>/dev/null; echo csq0216 | sudo -S docker stop moduforge 2>/dev/null; echo csq0216 | sudo -S docker rm moduforge 2>/dev/null')

# 2. Prune old image
print('=== PRUNE OLD IMAGE ===')
run('echo csq0216 | sudo -S docker rmi moduforge-app 2>/dev/null; echo csq0216 | sudo -S docker rmi moduforge 2>/dev/null')

# 3. Fresh build (no cache this time since we removed the old image)
print('=== DOCKER BUILD (fresh) ===')
t0 = time.time()
run('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && echo csq0216 | sudo -S docker build --no-cache -f backend/Dockerfile -t moduforge-app backend/', timeout=900)
elapsed = time.time() - t0
print(f'\nBuild took: {elapsed:.1f}s')

# 4. Start with docker-compose using the correct service name
print('=== DOCKER UP ===')
run('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && echo csq0216 | sudo -S docker compose up -d --force-recreate --remove-orphans')

time.sleep(5)
print('\n=== STATUS ===')
run('echo csq0216 | sudo -S docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"')

ssh.close()
print('\n=== DEPLOY COMPLETE ===')
print('Access: http://192.168.2.9:8086')
