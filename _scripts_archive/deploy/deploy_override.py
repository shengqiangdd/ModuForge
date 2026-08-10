#!/usr/bin/env python3
"""Deploy using original image + chmod via shell override"""
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

# Use original image with a modified entrypoint via command override
# This way we can chmod /server before running it
create_cmd = f"""docker create --name {CONTAINER} \
  -p 8087:8080 \
  -v /home/admin/moduforge_data/data:/data \
  -v /home/admin/moduforge_data/projects:/app/projects \
  -v /home/admin/moduforge_data/build-cache:/app/build-cache \
  -v /home/admin/moduforge_data/artifacts:/app/artifacts \
  --restart unless-stopped \
  moduforge:latest \
  /bin/sh -c 'chmod 755 /server 2>/dev/null; exec /docker-entrypoint.sh'"""

print(f"\n--- Create ---")
stdin, stdout, stderr = ssh.exec_command(create_cmd, timeout=15)
out = stdout.read().decode().strip()
err = stderr.read().decode().strip()
print(f"out: {out}")
if err: print(f"err: {err}")

# Copy new binary first
print("\n--- Copy binary ---")
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server", timeout=30)
err = stderr.read().decode().strip()
print(f"cp: {err or 'OK'}")

# Start
print("\n--- Start ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=10)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

time.sleep(5)

# Health
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"\nHealth: {stdout.read().decode().strip()}")

# Verify
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name=' 2>&1", timeout=15)
print(f"WHERE name=: {stdout.read().decode().strip()}")

# Check logs
stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 5 2>&1", timeout=10)
print(f"\nLogs: {stdout.read().decode('utf-8', errors='ignore').strip()}")

ssh.close()
