#!/usr/bin/env python3
"""Test with properly escaped content."""

import paramiko
import json
import subprocess
from pathlib import Path

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, desc=""):
    print(f"=== {desc} ===")
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()
    return out

# Get JWT token
token_out = run("""curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'""", "Login")
token = json.loads(token_out).get('token', '')

# Create a test project
project_out = run(f"""curl -s http://localhost:8086/api/v1/projects -X POST -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d '{{"name":"Test Module","description":"Test"}}'""", "Create project")
project_id = json.loads(project_out).get('id', '')
print(f"Project ID: {project_id}")

if project_id:
    # Add files using heredoc to avoid JSON escaping issues
    files_to_add = [
        ("module.prop", "id=test_module\\nname=Test Module\\nversion=1.0\\nversionCode=1\\nauthor=Test"),
        ("index.html", "<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Test</h1></body></html>"),
        ("css/styles.css", "body { font-family: Arial; }"),
        ("js/app.js", "console.log('test');"),
        ("service.sh", "#!/system/bin/sh\\n# Service"),
        ("customize.sh", "#!/system/bin/sh\\n# Install"),
        ("src/main.go", "package main\\nfunc main() {}"),
        ("tmp/test.sh", "#!/bin/sh\\n# Temp"),
        ("DESIGN_DOC.md", "# Design"),
        ("system/bin/test", "binary"),
    ]
    
    for path, content in files_to_add:
        # Use a Python script to make the API call with proper escaping
        cmd = f'''python3 -c "
import urllib.request
import json

url = 'http://localhost:8086/api/v1/projects/{project_id}/files/{path}'
data = json.dumps({{'content': '{content}'}}).encode()
req = urllib.request.Request(url, data=data, method='PUT')
req.add_header('Content-Type', 'application/json')
req.add_header('Authorization', 'Bearer {token}')
try:
    resp = urllib.request.urlopen(req)
    print(resp.read().decode())
except Exception as e:
    print(f'Error: {{e}}')
"'''
        run(cmd, f"Add {path}")
    
    # Export zip
    print("\n=== Exporting zip ===")
    cmd = f'''python3 -c "
import urllib.request

url = 'http://localhost:8086/api/v1/projects/{project_id}/export-zip'
req = urllib.request.Request(url, method='POST')
req.add_header('Authorization', 'Bearer {token}')
try:
    resp = urllib.request.urlopen(req)
    with open('/tmp/test_module_v2.zip', 'wb') as f:
        f.write(resp.read())
    print('Downloaded successfully')
except Exception as e:
    print(f'Error: {{e}}')
"'''
    run(cmd, "Export zip")
    
    # Check the zip
    run("""python3 -c "
import zipfile
try:
    with zipfile.ZipFile('/tmp/test_module_v2.zip', 'r') as zip_ref:
        print('ZIP contents:')
        for info in zip_ref.infolist():
            print(f'  {info.filename} ({info.file_size} bytes)')
        print()
        print('Checking for webroot:')
        has_webroot = any('webroot/' in info.filename for info in zip_ref.infolist())
        print(f'  Has webroot: {has_webroot}')
        print()
        print('Checking for excluded files:')
        excluded = ['.build_cache', 'src/', 'tmp/', 'DESIGN_DOC.md', 'app/backend/']
        for exc in excluded:
            found = any(exc in info.filename for info in zip_ref.infolist())
            print(f'  {exc}: {\"FOUND (bad!)\" if found else \"excluded (good)\"}')
except Exception as e:
    print(f'Error: {e}')
" """, "Analyze zip")

ssh.close()
print("\nDone!")
