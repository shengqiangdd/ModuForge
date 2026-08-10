#!/usr/bin/env python3
"""Deploy binary to ModuForge container"""

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
print("Uploading binary to server...")
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

# Copy binary into container
print("Copying binary into container...")
stdin, stdout, stderr = ssh.exec_command('docker cp /tmp/moduforge-server moduforge:/app/moduforge-server')
exit_code = stdout.channel.recv_exit_status()
if exit_code != 0:
    print(f"✗ Copy failed: {stderr.read().decode()}")
    ssh.close()
    sys.exit(1)
print("✓ Binary copied into container")

# Make binary executable
print("Making binary executable...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge chmod +x /app/moduforge-server')
exit_code = stdout.channel.recv_exit_status()
if exit_code != 0:
    print(f"✗ Chmod failed: {stderr.read().decode()}")
    ssh.close()
    sys.exit(1)
print("✓ Binary made executable")

# Restart container
print("Restarting container...")
stdin, stdout, stderr = ssh.exec_command('docker restart moduforge')
exit_code = stdout.channel.recv_exit_status()
if exit_code != 0:
    print(f"✗ Restart failed: {stderr.read().decode()}")
    ssh.close()
    sys.exit(1)
print("✓ Container restarted")

# Wait for container to be healthy
print("Waiting for container to be healthy...")
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