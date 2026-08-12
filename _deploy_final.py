import paramiko
import time
import sys

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)
print('Connected!')

base = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'

# Check existing containers
_, o, _ = c.exec_command(f'cd {base} && docker compose ps 2>&1', timeout=30)
status = o.read().decode()
print('Current status:', status)

# If container is missing, rebuild
if 'moduforge' not in status or 'Up' not in status:
    print('Container not running, rebuilding...')
    _, o2, _ = c.exec_command(f'cd {base} && docker compose down 2>&1', timeout=60)
    print(o2.read().decode())
    
    # Remove old image
    _, o3, _ = c.exec_command('docker rmi moduforge:latest 2>&1 || true', timeout=30)
    print(o3.read().decode())
    
    # Build and start
    _, o4, _ = c.exec_command(f'cd {base} && docker compose up -d --build 2>&1', timeout=600)
    # Just read the last part of output
    output = o4.read().decode()
    # Find the last 20 lines
    lines = output.strip().split('\n')
    for line in lines[-20:]:
        print(line)

# Wait for healthy
print('\nWaiting for container to be healthy...')
for i in range(30):
    time.sleep(10)
    _, oh, _ = c.exec_command('docker inspect --format="{{.State.Health.Status}}" moduforge 2>&1', timeout=10)
    health = oh.read().decode().strip()
    print(f'  [{i*10}s] Health: {health}')
    if health == 'healthy':
        print('Container is healthy!')
        break

# Final check
_, of, _ = c.exec_command(f'cd {base} && docker compose ps 2>&1', timeout=30)
print('\nFinal status:')
print(of.read().decode())

c.close()
print('Done!')
