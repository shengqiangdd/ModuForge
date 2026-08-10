#!/usr/bin/env python3
"""Fix: stop container, replace binary with executable permissions"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
CONTAINER = "moduforge"
LOCAL_BIN = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)

# Stop container first
print("Stopping container...")
ssh.exec_command(f"docker stop {CONTAINER}")
time.sleep(3)

# Verify stopped
stdin, stdout, stderr = ssh.exec_command(f"docker ps -a --filter name={CONTAINER} --format '{{.Status}}'")
print(f"Status: {stdout.read().decode().strip()}")

# Upload binary
print("Uploading...")
sftp = ssh.open_sftp()
sftp.put(LOCAL_BIN, "/tmp/moduforge-server-new")
sftp.close()

# Use docker cp to replace (it preserves the original file's permissions on Linux overlay)
# Actually we need to set permissions. Use a trick:
# 1. docker cp the file
# 2. Start container temporarily to fix permissions
# OR: use docker create + cp approach

# Approach: use docker create to get a temporary container, cp file, then start
print("Copying binary...")
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server", timeout=30)
err = stderr.read().decode().strip()
print(f"cp to /server: {'OK' if not err else err}")

# Now start with a custom command to fix permissions first
print("Starting with fix command...")
stdin, stdout, stderr = ssh.exec_command(
    f"docker start {CONTAINER}",
    timeout=10
)
print(stdout.read().decode().strip())
time.sleep(2)

# Check if running
stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format '{{.State.Status}}'")
status = stdout.read().decode().strip()
print(f"Container status: {status}")

if status == "running":
    # Fix permissions via exec
    stdin, stdout, stderr = ssh.exec_command(
        f"docker exec {CONTAINER} sh -c 'ls -la /server && chmod 755 /server 2>&1 || true'",
        timeout=10
    )
    print(f"ls+chmod: {stdout.read().decode().strip()}")
    
    # Also try: the entrypoint might use /server but we need to check if it's a symlink or copy
    stdin, stdout, stderr = ssh.exec_command(
        f"docker exec {CONTAINER} sh -c 'ls -la /proc/1/exe'",
        timeout=10
    )
    print(f"proc/1/exe: {stdout.read().decode().strip()}")
    
    # Now restart properly
    stdin, stdout, stderr = ssh.exec_command(f"docker restart {CONTAINER}", timeout=30)
    print(f"Restart: {stdout.read().decode().strip()}")
    time.sleep(5)
    
    # Health
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
    print(f"\nHealth: {stdout.read().decode().strip()}")
    
    # Verify
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name='", timeout=15)
    print(f"WHERE name=: {stdout.read().decode().strip()}")
else:
    print("Container not running, checking logs...")
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 5 2>&1", timeout=10)
    print(stdout.read().decode("utf-8", errors="ignore").strip())

ssh.close()
