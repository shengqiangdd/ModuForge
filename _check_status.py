import paramiko
import time

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Wait for build to complete
print('Waiting for build to complete...')
time.sleep(120)

# Check status
_, o, _ = c.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && docker compose ps 2>&1', timeout=30)
print(o.read().decode())

# Check if container is healthy
_, o2, _ = c.exec_command('docker inspect --format="{{.State.Health.Status}}" moduforge 2>&1', timeout=10)
print('Health:', o2.read().decode().strip())

c.close()
