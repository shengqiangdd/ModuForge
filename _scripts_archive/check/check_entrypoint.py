#!/usr/bin/env python3
"""Check container entrypoint"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', username='admin', password='csq0216')

# Check entrypoint script
print("1. Entrypoint script:")
stdin, stdout, stderr = client.exec_command('docker exec moduforge cat /docker-entrypoint.sh')
print(stdout.read().decode()[:1000])

# Check binary location
print("\n2. Binary location:")
stdin, stdout, stderr = client.exec_command('docker exec moduforge find / -name "moduforge-server" 2>/dev/null')
print(stdout.read().decode())

# Check if binary is being executed
print("\n3. Process info:")
stdin, stdout, stderr = client.exec_command('docker exec moduforge ps aux')
print(stdout.read().decode())

client.close()
