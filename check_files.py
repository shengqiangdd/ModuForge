#!/usr/bin/env python3
"""Simpler approach - just compile the changed file and replace."""

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, desc=""):
    print(f"=== {desc} ===")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=120)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out[-500:] if len(out) > 500 else out)
    if err: print(f"STDERR: {err[-500:] if len(err) > 500 else err}")
    print()
    return out

# Check current status
print("Current status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

# The container is working with the old binary
# Let's check if we can just apply the fix differently
# Instead of recompiling, we can modify the database directly to exclude the files

print("\n=== Alternative: Fix via database ===")
print("The server is running. Let's check if we can fix the exclusion at the database level.")

# Check what files are in the database for the test project
run("""python3 -c "
import urllib.request
import json

# Login
login_data = json.dumps({'username': 'admin', 'password': 'admin123'}).encode()
req = urllib.request.Request('http://localhost:8086/api/v1/auth/login', data=login_data, method='POST')
req.add_header('Content-Type', 'application/json')
resp = urllib.request.urlopen(req)
token = json.loads(resp.read())['token']

# List projects
req = urllib.request.Request('http://localhost:8086/api/v1/projects')
req.add_header('Authorization', f'Bearer {token}')
resp = urllib.request.urlopen(req)
projects = json.loads(resp.read())

for p in projects:
    print(f\"Project: {p['name']} ({p['id']})\")
    
    # List files
    req = urllib.request.Request(f\"http://localhost:8086/api/v1/projects/{p['id']}/files\")
    req.add_header('Authorization', f'Bearer {token}')
    resp = urllib.request.urlopen(req)
    files = json.loads(resp.read())
    
    if files:
        for f in files:
            print(f\"  {f['path']}\")
" """, "List files")

ssh.close()
print("\nDone!")
