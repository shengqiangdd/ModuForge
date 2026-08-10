#!/usr/bin/env python3
"""Test the fixed zipper with correct API endpoint."""

import paramiko
import json
import zipfile
import os
from pathlib import Path

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
LOCAL_DIR = Path("C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts")

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
print("Getting JWT token...")
token_out = run("""curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'""", "Login")
token = json.loads(token_out).get('token', '')
print(f"Token: {token[:30]}...")

# Create a test project
print("\n=== Creating test project ===")
project_out = run(f"""curl -s http://localhost:8086/api/v1/projects -X POST -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d '{{"name":"Test Module","description":"Test module for zip verification"}}'""", "Create project")
project_id = json.loads(project_out).get('id', '')
print(f"Project ID: {project_id}")

if project_id:
    # Add test files via the correct API
    test_files = [
        {"path": "module.prop", "content": "id=test_module\nname=Test Module\nversion=1.0\nversionCode=1\nauthor=Test"},
        {"path": "index.html", "content": "<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Test Module</h1></body></html>"},
        {"path": "css/styles.css", "content": "body { font-family: Arial; }"},
        {"path": "js/app.js", "content": "console.log('Test module');"},
        {"path": "service.sh", "content": "#!/system/bin/sh\n# Service script"},
        {"path": "customize.sh", "content": "#!/system/bin/sh\n# Install script"},
        {"path": "src/main.go", "content": "package main\nfunc main() {}"},
        {"path": "tmp/test.sh", "content": "#!/bin/sh\n# Test script"},
        {"path": "DESIGN_DOC.md", "content": "# Design Document"},
        {"path": "system/bin/test", "content": "binary content"},
    ]
    
    for file in test_files:
        # Use PUT to save files (correct method)
        file_out = run(f"""curl -s http://localhost:8086/api/v1/projects/{project_id}/files/{file['path']} -X PUT -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d '{{"content":"{file['content']}"}}'""", f"Add {file['path']}")
    
    # Export the zip using correct endpoint
    print("\n=== Exporting zip ===")
    run(f"""curl -s -o /tmp/test_module_fixed.zip http://localhost:8086/api/v1/projects/{project_id}/export-zip -X POST -H "Authorization: Bearer {token}" """, "Export zip")
    
    # Download and check
    run(f"docker cp {CONTAINER}:/tmp/test_module_fixed.zip /tmp/test_module_fixed.zip 2>/dev/null || echo 'Not in container'", "Copy from container")
    
    # Check the zip
    print("\n=== Analyzing zip contents ===")
    run("""python3 -c "
import zipfile
try:
    with zipfile.ZipFile('/tmp/test_module_fixed.zip', 'r') as zip_ref:
        print('ZIP contents:')
        for info in zip_ref.infolist():
            print(f'  {info.filename} ({info.file_size} bytes)')
except Exception as e:
    print(f'Error: {e}')
" """, "Zip analysis")

ssh.close()
print("\nDone!")
