#!/usr/bin/env python3
"""List project files."""

import paramiko
import json

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Get token
stdin, stdout, stderr = ssh.exec_command("""curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'""")
token = json.loads(stdout.read().decode()).get('token', '')

# List projects
stdin, stdout, stderr = ssh.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
projects = json.loads(stdout.read().decode())

for p in projects:
    pid = p['id']
    pname = p['name']
    print(f"\nProject: {pname} ({pid})")
    
    # List files
    stdin, stdout, stderr = ssh.exec_command(f'curl -s http://localhost:8086/api/v1/projects/{pid}/files -H "Authorization: Bearer {token}"')
    files = json.loads(stdout.read().decode())
    
    if files:
        for f in files:
            print(f"  {f.get('path', 'unknown')}")
    else:
        print("  (no files)")

ssh.close()
