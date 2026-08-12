import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# List Go files in container
print('=== List Go files in /app ===')
_, o1, _ = c.exec_command('docker exec moduforge ls -la /app/*.go 2>/dev/null')
print(o1.read().decode().strip())

# Check for build-related files
print('\n=== Check /app directory ===')
_, o2, _ = c.exec_command('docker exec moduforge ls -la /app/ 2>/dev/null')
print(o2.read().decode().strip())

c.close()
