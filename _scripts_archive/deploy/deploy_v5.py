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

PROJ = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'
SUDO = 'echo "csq0216" | sudo -S'

# 1. Fix Dockerfile on server (add mkdir for /data and /app/uploads)
print('=== FIX DOCKERFILE ===')
# Insert mkdir line before "EXPOSE 8080"
run(f'{SUDO} sed -i "s|# 服务端口|# 创建数据目录并设置权限\\nRUN mkdir -p /data /app/uploads \\&\\& chown -R app:app /data /app/uploads\\n\\n# 服务端口|" {PROJ}/backend/Dockerfile')

# Verify
stdin, stdout, stderr = ssh.exec_command(f'grep -n "mkdir\|EXPOSE\|USER" {PROJ}/backend/Dockerfile')
print(stdout.read().decode(errors='replace'))

# 2. Stop
print('=== STOP ===')
run(f'{SUDO} docker stop moduforge 2>/dev/null; {SUDO} docker rm moduforge 2>/dev/null')

# 3. Build
print('=== BUILD ===')
t0 = time.time()
run(f'cd {PROJ} && {SUDO} docker build --no-cache -f backend/Dockerfile -t moduforge:latest .', timeout=900)
print(f'\nBuild: {time.time()-t0:.1f}s')

# 4. Start
print('=== UP ===')
run(f'cd {PROJ} && {SUDO} docker compose up -d --force-recreate --remove-orphans')

time.sleep(5)
print('\n=== STATUS ===')
run(f'{SUDO} docker ps --format "table {{{{.Names}}}}\\t{{{{.Status}}}}\\t{{{{.Ports}}}}"')

time.sleep(5)
print('\n=== LOGS ===')
run(f'{SUDO} docker logs moduforge --tail 15 2>&1')

ssh.close()
print('\n=== DONE ===')
