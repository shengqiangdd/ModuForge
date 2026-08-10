#!/usr/bin/env python3
"""Deploy by stopping container, copying, and restarting"""
import paramiko, time, os

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
LOCAL_BIN = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)
print("Connected")

# Step 1: Check current binary version
print("\n--- Current binary version ---")
stdin, stdout, stderr = ssh.exec_command(
    f"docker exec {CONTAINER} /app/moduforge-server --version 2>&1 || docker exec {CONTAINER} strings /app/moduforge-server | grep -i 'custom provider' | head -5",
    timeout=10
)
print(stdout.read().decode("utf-8", errors="ignore").strip())

# Step 2: Stop container
print("\n--- Stopping container ---")
stdin, stdout, stderr = ssh.exec_command(f"docker stop {CONTAINER}", timeout=30)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Step 3: Upload binary to /tmp
print("\n--- Uploading new binary ---")
sftp = ssh.open_sftp()
sftp.put(LOCAL_BIN, "/tmp/moduforge-server-new")
sftp.close()
print("Uploaded")

# Step 4: Use docker cp to replace the binary (container is stopped, so we can use docker cp)
print("\n--- Replacing binary ---")
stdin, stdout, stderr = ssh.exec_command(
    f"docker cp /tmp/moduforge-server-new {CONTAINER}:/app/moduforge-server",
    timeout=30
)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Step 5: Start container
print("\n--- Starting container ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=30)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Step 6: Wait and health check
print("\nWaiting 5s...")
time.sleep(5)

stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"Health: {stdout.read().decode().strip()}")

# Step 7: Verify the new binary has the fix
print("\n--- Verify new binary ---")
stdin, stdout, stderr = ssh.exec_command(
    f"docker exec {CONTAINER} strings /app/moduforge-server | grep -c 'custom_providers WHERE name'",
    timeout=10
)
count = stdout.read().decode().strip()
print(f"Matches for 'custom_providers WHERE name' in binary: {count}")

# Step 8: Check container logs
stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 15 2>&1", timeout=10)
print(f"\n--- Container logs ---")
print(stdout.read().decode("utf-8", errors="ignore").strip())

ssh.close()
print("\nDone")
