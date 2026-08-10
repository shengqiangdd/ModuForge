#!/usr/bin/env python3
"""Check container restart and logs"""

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

# Stop and restart the container
print("Stopping container...")
stdin, stdout, stderr = ssh.exec_command('docker stop moduforge')
stdout.read()
print("✓ Container stopped")

print("Starting container...")
stdin, stdout, stderr = ssh.exec_command('docker start moduforge')
stdout.read()
print("✓ Container started")

# Wait for container to be healthy
import time
time.sleep(5)

# Check container status
print("\nChecking container status...")
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format "{{.State.Status}}"')
status = stdout.read().decode().strip()
print(f"Container status: {status}")

# Check container logs
print("\nChecking container logs...")
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 30')
logs = stdout.read().decode()
print("Container logs (last 30 lines):")
print(logs)

# Check if the server is listening
print("\nChecking if server is listening...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge netstat -tlnp 2>/dev/null || ss -tlnp 2>/dev/null || echo "netstat/ss not available"')
output = stdout.read().decode()
print(output)

ssh.close()