#!/usr/bin/env python3
"""Fix using a running container to chmod"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)

# Remove broken container
print("Remove...")
ssh.exec_command(f"docker rm -f {CONTAINER}")
time.sleep(2)

# Create container with entrypoint override that fixes all permissions first
create_cmd = f"""docker create --name {CONTAINER} \
  -p 8087:8080 \
  -v /home/admin/moduforge_data/data:/data \
  -v /home/admin/moduforge_data/projects:/app/projects \
  -v /home/admin/moduforge_data/build-cache:/app/build-cache \
  -v /home/admin/moduforge_data/artifacts:/app/artifacts \
  --restart unless-stopped \
  moduforge:latest \
  /bin/sh -c 'chmod 755 /docker-entrypoint.sh /server /app/moduforge-server 2>/dev/null; exec /docker-entrypoint.sh'"""

print(f"\n--- Create ---")
stdin, stdout, stderr = ssh.exec_command(create_cmd, timeout=15)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Copy new binary
print("\n--- Copy binary ---")
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server", timeout=30)
print(f"cp /server: {stderr.read().decode().strip() or 'OK'}")

# Start
print("\n--- Start ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=10)
result = stdout.read().decode().strip() or stderr.read().decode().strip()
print(result)

time.sleep(5)

# Health
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
health = stdout.read().decode().strip()
print(f"\nHealth: {health}")

if "ok" in health:
    # Verify binary
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name=' 2>&1", timeout=15)
    print(f"WHERE name=: {stdout.read().decode().strip()}")
    
    # Check logs
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 10 2>&1", timeout=10)
    print(f"\nLogs: {stdout.read().decode('utf-8', errors='ignore').strip()}")
else:
    # Check logs for error
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 10 2>&1", timeout=10)
    print(f"\nLogs: {stdout.read().decode('utf-8', errors='ignore').strip()}")

ssh.close()
