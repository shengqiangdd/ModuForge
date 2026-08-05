#!/usr/bin/env python3
"""Get full DESIGN_DOC from ModuForge"""
import paramiko
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace')

# Get auth token
login_resp = run("curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"csq\",\"password\":\"csq0216\"}'")
token = json.loads(login_resp).get('token', '')

# Get DESIGN_DOC.md
design_resp = run(f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files/DESIGN_DOC.md -H 'Authorization: Bearer {token}'")
design = json.loads(design_resp)
content = design.get('content', '')

# Save to local file
with open('DESIGN_DOC_REMOTE.md', 'w', encoding='utf-8') as f:
    f.write(content)

print(f"Saved DESIGN_DOC.md ({len(content)} chars)")
print("\n=== First 3000 chars ===")
print(content[:3000])

ssh.close()
