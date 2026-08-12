import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Check if server is running
print('=== Check server process ===')
_, o1, _ = c.exec_command('docker exec moduforge ps aux | grep server', timeout=10)
print(o1.read().decode().strip())

# Check API endpoints
print('\n=== Check API ===')
_, o2, _ = c.exec_command('curl -s http://localhost:8086/api/v1/projects 2>&1 | head -3', timeout=10)
print(o2.read().decode().strip())

# Check logs
print('\n=== Recent logs ===')
_, o3, _ = c.exec_command('docker logs moduforge --tail=5 2>&1', timeout=10)
print(o3.read().decode().strip())

c.close()
