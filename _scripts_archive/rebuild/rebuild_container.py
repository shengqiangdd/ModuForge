#!/usr/bin/env python3
"""Rebuild container with proper permissions using docker commit"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
CONTAINER = "moduforge"
LOCAL_BIN = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)

# First, stop the broken container
print("Stopping...")
ssh.exec_command(f"docker stop {CONTAINER}")
time.sleep(2)

# Create a temporary container from the original image with a shell
print("\n--- Get original image and run config ---")
stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format '{{{{.Config.Image}}}}'", timeout=10)
image = stdout.read().decode().strip().strip("'")
print(f"Image: {image}")

# Get volumes and env
stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format '{{{{range .Mounts}}}}{{.Source}}:{{.Destination}} {{end}}'", timeout=10)
mounts = stdout.read().decode().strip()
print(f"Mounts: {mounts}")

stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format '{{{{range .Config.Env}}}}{{.}} {{end}}'", timeout=10)
env = stdout.read().decode().strip()
print(f"Env: {env[:200]}")

# Strategy: Create a new container with proper entrypoint that does chmod first
# Then docker commit to save the state

# Create a wrapper entrypoint
wrapper = '''#!/bin/sh
chmod 755 /server 2>/dev/null || true
chmod 755 /app/moduforge-server 2>/dev/null || true
exec /server "$@"
'''

# Write wrapper to host
sftp = ssh.open_sftp()
with sftp.open("/tmp/wrapper.sh", "w") as f:
    f.write(wrapper)
sftp.close()

# Create a temp container to fix the image
print("\n--- Create temp container ---")
# Use the image directly, not the broken container
stdin, stdout, stderr = ssh.exec_command(
    f"docker create --name temp_fix {image} /bin/sh -c 'chmod 755 /server /app/moduforge-server 2>/dev/null; exec /server'",
    timeout=10
)
print(stdout.read().decode().strip())

# Copy new binary
print("\n--- Copy binary ---")
stdin, stdout, stderr = ssh.exec_command(f"docker cp {LOCAL_BIN.replace(chr(92), '/')} temp_fix:/server", timeout=30)
# Oops, Windows path. Need to use /tmp path
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new temp_fix:/server", timeout=30)
print(f"cp /server: {stderr.read().decode().strip() or 'OK'}")

# Copy entrypoint wrapper
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/wrapper.sh temp_fix:/docker-entrypoint.sh", timeout=10)
print(f"cp wrapper: {stderr.read().decode().strip() or 'OK'}")

# Copy to /app as well
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new temp_fix:/app/moduforge-server", timeout=30)
print(f"cp /app: {stderr.read().decode().strip() or 'OK'}")

# Commit the temp container as a new image
print("\n--- Commit ---")
stdin, stdout, stderr = ssh.exec_command(f"docker commit temp_fix {image}", timeout=60)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Clean up temp container
ssh.exec_command("docker rm -f temp_fix")

# Remove old broken container
print("\n--- Remove old container ---")
ssh.exec_command(f"docker rm -f {CONTAINER}")

# Create new container with same config
print("\n--- Create new container ---")
# Get the port mapping
stdin, stdout, stderr = ssh.exec_command(f"docker inspect {image} --format '{{{{range .Config.ExposedPorts}}}}{{.}} {{end}}' 2>/dev/null", timeout=10)
ports = stdout.read().decode().strip()
print(f"Exposed: {ports}")

# Recreate with volumes
create_cmd = (
    f"docker create --name {CONTAINER} "
    f"-p 8087:8080 "
    f"-v /home/admin/moduforge_data/data:/data "
    f"-v /home/admin/moduforge_data/projects:/app/projects "
    f"-v /home/admin/moduforge_data/build-cache:/app/build-cache "
    f"-v /home/admin/moduforge_data/artifacts:/app/artifacts "
    f"--restart unless-stopped "
    f"{image}"
)
stdin, stdout, stderr = ssh.exec_command(create_cmd, timeout=15)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Start
print("\n--- Start ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=10)
print(stdout.read().decode().strip())
time.sleep(5)

# Health
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"\nHealth: {stdout.read().decode().strip()}")

# Verify
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name='", timeout=15)
print(f"WHERE name=: {stdout.read().decode().strip()}")

ssh.close()
print("\nDone")
