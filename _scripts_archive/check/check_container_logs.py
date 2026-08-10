#!/usr/bin/env python3
"""Check container logs for errors"""

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

# Check container logs
print("Checking container logs...")
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 50')
logs = stdout.read().decode()
print("Container logs (last 50 lines):")
print(logs)

# Check if the container is running the latest binary
print("\nChecking container binary version...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/moduforge-server')
print(stdout.read().decode())

# Check if the prompts directory exists in the container
print("\nChecking prompts directory in container...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/prompts/ 2>/dev/null || echo "Prompts directory not found"')
print(stdout.read().decode())

ssh.close()