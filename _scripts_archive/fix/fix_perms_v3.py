#!/usr/bin/env python3
"""Fix: modify entrypoint to chmod /server before exec"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)

# Stop container
print("Stopping...")
ssh.exec_command(f"docker stop {CONTAINER}")
time.sleep(3)

# Check entrypoint script
print("\n--- Entrypoint ---")
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server", timeout=30)
print(f"cp: {stderr.read().decode().strip() or 'OK'}")

# The entrypoint is /docker-entrypoint.sh
# Let's see what it does
print("\n--- Entrypoint content ---")
stdin, stdout, stderr = ssh.exec_command(
    "docker create --name temp_moduforge moduforge:latest /bin/cat /docker-entrypoint.sh",
    timeout=10
)
print(stdout.read().decode().strip())
print(stderr.read().decode().strip())

# Actually let's just modify the approach:
# Instead of modifying entrypoint, let's mount a modified entrypoint
# OR: create a wrapper script

# Better approach: Create a wrapper that sets permissions then execs /server
stdin, stdout, stderr = ssh.exec_command(f"docker rm -f temp_moduforge 2>/dev/null; echo ok", timeout=10)

# Create a wrapper script
wrapper = '''#!/bin/sh
chmod 755 /server 2>/dev/null || true
exec /server "$@"
'''

# Write wrapper to host
sftp = ssh.open_sftp()
with sftp.open("/tmp/entrypoint-wrapper.sh", "w") as f:
    f.write(wrapper)
sftp.close()

# Copy wrapper into container
print("\n--- Deploy wrapper ---")
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/entrypoint-wrapper.sh {CONTAINER}:/docker-entrypoint.sh", timeout=10)
print(f"cp wrapper: {stderr.read().decode().strip() or 'OK'}")

# Also copy binary
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server", timeout=30)
print(f"cp binary: {stderr.read().decode().strip() or 'OK'}")

# Make wrapper executable (docker cp preserves source permissions, but this is from Linux host)
stdin, stdout, stderr = ssh.exec_command(
    f"docker create --name fix_perm moduforge:latest /bin/sh -c 'chmod 755 /docker-entrypoint.sh /server && cp /server /app/moduforge-server && chmod 755 /app/moduforge-server'",
    timeout=10
)
print(f"create fix container: {stdout.read().decode().strip()} {stderr.read().decode().strip()}")

# Actually, a simpler approach: create a container that just fixes perms
# docker create with different entrypoint

# Start with the wrapper
print("\n--- Start ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=10)
print(stdout.read().decode().strip())
time.sleep(3)

# Check
stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 5 2>&1", timeout=10)
logs = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\nLogs: {logs}")

# Health
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"Health: {stdout.read().decode().strip()}")

ssh.close()
