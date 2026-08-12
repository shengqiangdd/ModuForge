import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Check server binary
print('=== Check /server directory ===')
_, o1, _ = c.exec_command('docker exec moduforge ls -la /server 2>/dev/null')
print(o1.read().decode().strip())

# Check source code location
print('\n=== Check source code in container ===')
_, o2, _ = c.exec_command('docker exec moduforge find / -name "build_module.go" 2>/dev/null | head -5')
print(o2.read().decode().strip())

# Check backend source in container
print('\n=== Check /app/backend ===')
_, o3, _ = c.exec_command('docker exec moduforge ls -la /app/backend 2>/dev/null')
print(o3.read().decode().strip())

c.close()
