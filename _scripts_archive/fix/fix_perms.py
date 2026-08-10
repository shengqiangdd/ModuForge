#!/usr/bin/env python3
"""Fix permissions and test"""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Check status
stdin, stdout, stderr = ssh.exec_command("docker ps --format '{{.Names}}\t{{.Status}}'", timeout=10)
print(f"Containers:\n{stdout.read().decode().strip()}")

# The container is likely failing because /server is not executable
# Let's check the entrypoint script
stdin, stdout, stderr = ssh.exec_command("docker inspect moduforge --format '{{.Config.Entrypoint}} {{.Config.Cmd}}'", timeout=10)
print(f"Entrypoint: {stdout.read().decode().strip()}")

# Try to start with a custom entrypoint that fixes permissions
stdin, stdout, stderr = ssh.exec_command(
    "docker cp /tmp/moduforge-server-new moduforge:/server",
    timeout=30
)
print(f"Copy: {stdout.read().decode().strip()} or {stderr.read().decode().strip()}")

# Use nsenter or docker exec with root to chmod
# Actually, we can use docker exec since the container user is moduforg
# Let's try installing busybox or using a different approach
stdin, stdout, stderr = ssh.exec_command(
    "docker exec -u root moduforge chmod 755 /server 2>&1",
    timeout=10
)
print(f"chmod as root: {stdout.read().decode().strip()} {stderr.read().decode().strip()}")

# Also copy to /app/moduforge-server
stdin, stdout, stderr = ssh.exec_command(
    "docker exec -u root moduforge cp /server /app/moduforge-server && docker exec -u root moduforge chmod 755 /app/moduforge-server",
    timeout=10
)
print(f"cp+chmod: {stdout.read().decode().strip()} {stderr.read().decode().strip()}")

# Restart
stdin, stdout, stderr = ssh.exec_command("docker restart moduforge", timeout=30)
print(f"Restart: {stdout.read().decode().strip()}")
time.sleep(5)

# Health check
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"\nHealth: {stdout.read().decode().strip()}")

# Verify
stdin, stdout, stderr = ssh.exec_command("docker exec moduforge strings /server | grep -c 'WHERE name='", timeout=15)
print(f"WHERE name= count: {stdout.read().decode().strip()}")

stdin, stdout, stderr = ssh.exec_command("docker exec moduforge strings /server | grep -c 'not in DB'", timeout=15)
print(f"not in DB count: {stdout.read().decode().strip()}")

ssh.close()
