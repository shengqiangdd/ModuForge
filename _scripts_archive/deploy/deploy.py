import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Git pull
print('=== GIT PULL ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && git pull 2>&1')
print(stdout.read().decode())
err = stderr.read().decode()
if err: print(err)

# 2. Docker build (no cache)
print('=== DOCKER BUILD ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose build --no-cache 2>&1')
# Wait up to 5 minutes
output = b''
while not stdout.channel.exit_status_ready():
    chunk = stdout.read(4096)
    if chunk:
        output += chunk
    else:
        time.sleep(1)
output += stdout.read()
text = output.decode(errors='replace')
print(text[-3000:] if len(text) > 3000 else text)

# 3. Docker up
print('=== DOCKER UP ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())
print(stderr.read().decode())

# 4. Check status
print('=== STATUS ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose ps 2>&1')
print(stdout.read().decode())

ssh.close()
print('=== DEPLOY COMPLETE ===')
