#!/usr/bin/env python3
"""Deploy binary to ModuForge container - v2 with permission fix"""

import sys
import paramiko

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216')

# Upload binary to server
print("1. Uploading binary to server...")
sftp = ssh.open_sftp()
local_path = r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\moduforge-server.exe"
remote_path = "/tmp/moduforge-server"

try:
    sftp.put(local_path, remote_path)
    print(f"✓ Binary uploaded to {remote_path}")
except Exception as e:
    print(f"✗ Upload failed: {e}")
    ssh.close()
    sys.exit(1)
finally:
    sftp.close()

# Check container's current binary
print("2. Checking container's current binary...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/moduforge-server')
print(stdout.read().decode())

# Stop container
print("3. Stopping container...")
stdin, stdout, stderr = ssh.exec_command('docker stop moduforge')
exit_code = stdout.channel.recv_exit_status()
if exit_code != 0:
    print(f"✗ Stop failed: {stderr.read().decode()}")
    ssh.close()
    sys.exit(1)
print("✓ Container stopped")

# Copy binary into container (while stopped)
print("4. Copying binary into container...")
stdin, stdout, stderr = ssh.exec_command('docker cp /tmp/moduforge-server moduforge:/app/moduforge-server')
exit_code = stdout.channel.recv_exit_status()
if exit_code != 0:
    print(f"✗ Copy failed: {stderr.read().decode()}")
    ssh.close()
    sys.exit(1)
print("✓ Binary copied into container")

# Start container
print("5. Starting container...")
stdin, stdout, stderr = ssh.exec_command('docker start moduforge')
exit_code = stdout.channel.recv_exit_status()
if exit_code != 0:
    print(f"✗ Start failed: {stderr.read().decode()}")
    ssh.close()
    sys.exit(1)
print("✓ Container started")

# Wait for container to be healthy
print("6. Waiting for container to be healthy...")
import time
time.sleep(5)

# Check health
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format "{{.State.Status}}"')
status = stdout.read().decode().strip()
print(f"Container status: {status}")

if status != "running":
    print("✗ Container is not running")
    ssh.close()
    sys.exit(1)

# Check API health
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
health = stdout.read().decode().strip()
print(f"API health: {health}")

ssh.close()
print("\n✓ Deployment completed successfully!")