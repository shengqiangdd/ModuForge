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

# 1. Stop old
print('=== STOP ===')
run('echo "csq0216" | sudo -S docker stop moduforge 2>/dev/null; echo "csq0216" | sudo -S docker rm moduforge 2>/dev/null')

# 2. Remove old images
print('=== PRUNE ===')
run('echo "csq0216" | sudo -S docker rmi moduforge:latest moduforge-app:latest 2>/dev/null')

# 3. Build with --no-cache (CGO change needs full rebuild)
print('=== DOCKER BUILD (CGO_ENABLED=1, no-cache) ===')
t0 = time.time()
run('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && echo "csq0216" | sudo -S docker build --no-cache -f backend/Dockerfile -t moduforge:latest .', timeout=900)
print(f'\nBuild took: {time.time()-t0:.1f}s')

# 4. Start
print('=== DOCKER UP ===')
run('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && echo "csq0216" | sudo -S docker compose up -d --force-recreate --remove-orphans')

# 5. Wait and check
time.sleep(5)
print('\n=== STATUS ===')
run('echo "csq0216" | sudo -S docker ps --format "table {{.Names}}\\t{{.Status}}\\t{{.Ports}}"')

# 6. Check logs
time.sleep(3)
print('\n=== LOGS (last 10) ===')
run('echo "csq0216" | sudo -S docker logs moduforge --tail 10 2>&1')

ssh.close()
print('\n=== DONE ===')
print('Access: http://192.168.2.9:8086')
