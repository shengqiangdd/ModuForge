#!/usr/bin/env python3
"""Wait for server and list files."""

import paramiko
import json
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Wait for server
print("Waiting for server...")
for i in range(5):
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode().strip()
    print(f"  Attempt {i+1}: {health}")
    if health:
        break
    time.sleep(3)

# Get token
stdin, stdout, stderr = ssh.exec_command("""curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'""")
token_resp = stdout.read().decode()
print(f"\nLogin response: {token_resp[:100]}")

token = json.loads(token_resp).get('token', '')
print(f"Token: {token[:30]}...")

# List projects
stdin, stdout, stderr = ssh.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
projects = json.loads(stdout.read().decode())
print(f"\nFound {len(projects)} projects")

for p in projects:
    pid = p['id']
    pname = p['name']
    print(f"\nProject: {pname} ({pid})")
    
    # List files
    stdin, stdout, stderr = ssh.exec_command(f'curl -s http://localhost:8086/api/v1/projects/{pid}/files -H "Authorization: Bearer {token}"')
    files_resp = stdout.read().decode()
    
    try:
        files = json.loads(files_resp)
        if files:
            for f in files:
                print(f"  {f.get('path', 'unknown')}")
        else:
            print("  (no files)")
    except:
        print(f"  Error: {files_resp[:100]}")

ssh.close()
print("\nDone!")
