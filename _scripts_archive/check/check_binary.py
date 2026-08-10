#!/usr/bin/env python3
"""Check binary in container"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', username='admin', password='csq0216')

# Check binary exists and is recent
print("1. Binary info:")
stdin, stdout, stderr = client.exec_command('docker exec moduforge ls -la /app/moduforge-server')
print(stdout.read().decode())

# Check running process
print("\n2. Running process:")
stdin, stdout, stderr = client.exec_command('docker exec moduforge ps aux | grep moduforge')
print(stdout.read().decode())

# Test existing agent endpoints
print("\n3. Test existing agent endpoints:")
stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/skills')
out = stdout.read().decode()
print(f"   Skills: {out[:200]}")

# Check all routes
print("\n4. All routes:")
stdin, stdout, stderr = client.exec_command('docker exec moduforge cat /proc/$(pgrep moduforge)/cmdline | tr "\\0" " "')
print(stdout.read().decode())

client.close()
