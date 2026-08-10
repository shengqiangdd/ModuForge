#!/usr/bin/env python3
"""Debug security API"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', username='admin', password='csq0216')

# Check all agent routes
stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/skills')
print("Skills:", stdout.read().decode()[:300])

# Check security rules
stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/security/rules')
print("\nSecurity Rules:", stdout.read().decode()[:500])

# Check health
stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/health')
print("\nHealth:", stdout.read().decode())

# Check container logs for errors
stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 10 2>&1 | grep -i error')
print("\nErrors:", stdout.read().decode())

client.close()
