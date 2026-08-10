#!/usr/bin/env python3
"""Properly recreate container with correct mounts"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)

# Remove existing broken container
print("Remove old...")
ssh.exec_command(f"docker rm -f {CONTAINER}")
time.sleep(2)

# Create with correct mounts (from the inspect output)
create_cmd = f"""docker create --name {CONTAINER} \
  -p 8087:8080 \
  -v /home/admin/moduforge_data/data:/data \
  -v /home/admin/moduforge_data/projects:/app/projects \
  -v /home/admin/moduforge_data/build-cache:/app/build-cache \
  -v /home/admin/moduforge_data/artifacts:/app/artifacts \
  --restart unless-stopped \
  moduforge:latest"""

print(f"\n--- Create ---")
stdin, stdout, stderr = ssh.exec_command(create_cmd, timeout=15)
out = stdout.read().decode().strip()
err = stderr.read().decode().strip()
print(f"out: {out}")
print(f"err: {err}")

# Start
print("\n--- Start ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=10)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

time.sleep(5)

# Health
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"\nHealth: {stdout.read().decode().strip()}")

# Check entrypoint
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} cat /docker-entrypoint.sh 2>&1", timeout=10)
print(f"\nEntrypoint:\n{stdout.read().decode('utf-8', errors='ignore').strip()}")

# Check running binary
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} ls -la /server 2>&1", timeout=10)
print(f"\n/server: {stdout.read().decode().strip()}")

# Check strings
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name=' 2>&1", timeout=15)
print(f"\nWHERE name=: {stdout.read().decode().strip()}")

ssh.close()
