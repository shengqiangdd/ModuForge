#!/usr/bin/env python3
"""Get AndroBoost-SmartTune project details and find PRD"""
import paramiko
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=15)
    return stdout.read().decode('utf-8', errors='replace')

# First get auth token
print("=== Getting Auth Token ===")
login_resp = run("curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"csq\",\"password\":\"csq0216\"}'")
try:
    token_data = json.loads(login_resp)
    token = token_data.get('token', '')
    print(f"Token obtained: {token[:30]}...")
except:
    print("Failed to get token")
    print(login_resp[:200])
    token = ""

# Get project details
print("\n=== Project Details ===")
project_resp = run(f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864 -H 'Authorization: Bearer {token}'")
try:
    project = json.loads(project_resp)
    print(f"Project: {project.get('name', 'Unknown')}")
    print(f"Description: {project.get('description', '')[:200]}")
except:
    print(project_resp[:300])

# Get project files
print("\n=== Project Files ===")
files_resp = run(f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files -H 'Authorization: Bearer {token}'")
try:
    files = json.loads(files_resp)
    if isinstance(files, list):
        print(f"Total files: {len(files)}")
        # Look for PRD or design doc
        for f in files:
            path = f.get('path', '')
            if 'prd' in path.lower() or 'design' in path.lower() or 'readme' in path.lower():
                print(f"  Found: {path}")
    else:
        print("Files response not a list")
        print(str(files)[:200])
except:
    print(files_resp[:300])

# Search for DESIGN_DOC
print("\n=== Search for DESIGN_DOC ===")
design_resp = run(f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files/DESIGN_DOC.md -H 'Authorization: Bearer {token}'")
print(design_resp[:500])

ssh.close()
