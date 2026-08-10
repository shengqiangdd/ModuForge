import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

# 1. Check container status
stdin, stdout, stderr = ssh.exec_command('docker ps -a --filter name=moduforge --format "{{.Names}} {{.Status}} {{.Image}}"')
print('=== Container status ===')
print(stdout.read().decode())

# 2. Check data directory on server
stdin, stdout, stderr = ssh.exec_command('ls -la /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/data/ 2>&1')
print('=== data/ directory ===')
print(stdout.read().decode())

# 3. Check database files
stdin, stdout, stderr = ssh.exec_command('find /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge -name "*.db" -type f 2>&1')
print('=== Database files ===')
print(stdout.read().decode())

# 4. Check container /app/data
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge-app-1 ls -la /app/data/ 2>&1 || docker exec moduforge ls -la /app/data/ 2>&1')
print('=== Container /app/data/ ===')
print(stdout.read().decode())

# 5. Check database content via API
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
print('=== Health check ===')
print(stdout.read().decode())

# 6. Check if login works
stdin, stdout, stderr = ssh.exec_command('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"admin123\"}" 2>&1')
print('=== Login test ===')
print(stdout.read().decode())

ssh.close()
