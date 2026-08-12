"""Find where the backend actually runs"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check all containers
print('=== All containers ===')
stdin, stdout, stderr = ssh.exec_command('docker ps -a --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | head -20')
print(stdout.read().decode())

# 2. Check moduforge container details
print('\n=== ModuForge container inspect ===')
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format="{{json .Mounts}}" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -40')
print(stdout.read().decode())

# 3. Check what's running inside moduforge
print('\n=== Processes in moduforge ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ps aux 2>/dev/null | head -20')
print(stdout.read().decode())

# 4. Check if backend is a separate container
print('\n=== Backend container ===')
stdin, stdout, stderr = ssh.exec_command('docker ps -a --filter "name=backend" --filter "name=app" --format "table {{.Names}}\t{{.Status}}\t{{.Image}}" | head -10')
print(stdout.read().decode())

# 5. Check the dist directory
print('\n=== dist contents ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/dist/ 2>/dev/null')
print(stdout.read().decode())

# 6. Check the actual docker-compose
print('\n=== docker-compose.yml ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge cat /app/docker-compose.yml 2>/dev/null || cat /vol1/docker/overlay2/*/diff/app/docker-compose.yml 2>/dev/null | head -80')
print(stdout.read().decode())

# 7. Check if there's a separate backend binary somewhere
print('\n=== Backend binary search ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find / -name "moduforge" -type f -executable 2>/dev/null | head -5')
print(stdout.read().decode())

# 8. Check what ports the container exposes
print('\n=== Container ports ===')
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format="{{json .HostConfig.PortBindings}}" 2>/dev/null | python3 -m json.tool 2>/dev/null')
print(stdout.read().decode())

# 9. Check if nginx or something else is serving
print('\n=== Nginx/processes ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge sh -c "cat /etc/nginx/nginx.conf 2>/dev/null || cat /etc/nginx/conf.d/default.conf 2>/dev/null | head -30"')
print(stdout.read().decode())

# 10. Check the actual entrypoint
print('\n=== Container entrypoint ===')
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format="{{.Config.Entrypoint}}" 2>/dev/null')
print(stdout.read().decode())
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format="{{.Config.Cmd}}" 2>/dev/null')
print(stdout.read().decode())

ssh.close()
