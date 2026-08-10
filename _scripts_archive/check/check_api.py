#!/usr/bin/env python3
"""Check API status"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', username='admin', password='csq0216')

# Check API
stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/health')
out = stdout.read().decode()
print(f"API Health: {out}")

# Check container logs
stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 20')
out = stdout.read().decode()
print(f"\nContainer Logs:\n{out}")

client.close()
