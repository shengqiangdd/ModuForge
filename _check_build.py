import paramiko
import time

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Check if build is running
_, o, _ = c.exec_command('ps aux | grep -E "docker|build" | grep -v grep | head -5', timeout=10)
print('Running processes:')
print(o.read().decode())

# Check docker images
_, o2, _ = c.exec_command('docker images | grep moduforge', timeout=10)
print('\nDocker images:')
print(o2.read().decode())

# Check if there's a build cache
_, o3, _ = c.exec_command('docker buildx ls 2>&1 | head -10', timeout=10)
print('\nBuildx info:')
print(o3.read().decode())

c.close()
