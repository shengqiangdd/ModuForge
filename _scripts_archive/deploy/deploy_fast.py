import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Git pull
print('=== GIT PULL ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && git pull 2>&1')
print(stdout.read().decode())

# 2. Docker build (NO --no-cache, use layer cache!)
print('=== DOCKER BUILD (cached) ===')
t0 = time.time()
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && docker build -t moduforge-app . 2>&1')
output = stdout.read().decode()
print(output[-1000:] if len(output) > 1000 else output)
print(f'Build took: {time.time()-t0:.1f}s')

# 3. Docker up
print('=== DOCKER UP ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && docker compose up -d 2>&1')
print(stdout.read().decode())

# 4. Status
time.sleep(2)
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && docker compose ps 2>&1')
print('=== STATUS ===')
print(stdout.read().decode())

ssh.close()
print('=== DEPLOY COMPLETE ===')
