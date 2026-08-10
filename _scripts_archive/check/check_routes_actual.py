#!/usr/bin/env python3
"""Check actual registered routes in the container"""

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

# Check if there's a route listing endpoint
print("1. Testing various API endpoints...")
tests = [
    "/api/v1/projects",
    "/api/v1/auth/login",
    "/api/v1/llm/config",
    "/api/v1/ai/prompts",
    "/api/v1/md-prompts",
    "/api/v1/skills",
    "/health",
]

for path in tests:
    stdin, stdout, stderr = ssh.exec_command(f'curl -s -o /dev/null -w "%{{http_code}}" http://localhost:8086{path}')
    code = stdout.read().decode().strip()
    print(f"  {path}: {code}")

# Check if there's a middleware issue
print("\n2. Checking if auth middleware is blocking routes...")
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/api/v1/md-prompts -H "Authorization: Bearer invalid"')
print(f"Invalid token response: {stdout.read().decode()[:200]}")

# Check the full error response
print("\n3. Getting full error response for /api/v1/md-prompts...")
stdin, stdout, stderr = ssh.exec_command('curl -s -v http://localhost:8086/api/v1/md-prompts 2>&1')
output = stdout.read().decode()
print(output[:500])

ssh.close()