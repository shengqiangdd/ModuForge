#!/usr/bin/env python3
"""Test the fixed zipper by creating a test project and exporting zip."""

import paramiko
import json
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Get JWT token
print("Getting JWT token...")
stdin, stdout, stderr = ssh.exec_command(
    """curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'"""
)
token_response = stdout.read().decode()
token = json.loads(token_response).get('token', '')
print(f"Token: {token[:20]}...")

# Test the zip creation by creating a test project with files
print("\n=== Creating test project ===")

# Create a test project
test_project = {
    "name": "Test Module",
    "description": "Test module for zip verification"
}

# Create project via API
stdin, stdout, stderr = ssh.exec_command(f"""curl -s http://localhost:8086/api/v1/projects -X POST -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d '{json.dumps(test_project)}'""")
project_response = stdout.read().decode()
print(f"Project response: {project_response}")

# Parse project ID
try:
    project_id = json.loads(project_response).get('id', '')
    print(f"Project ID: {project_id}")
except:
    print("Failed to create project")
    project_id = None

if project_id:
    # Add test files to the project
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
        stdin, stdout, stderr = ssh.exec_command(f"""curl -s http://localhost:8086/api/v1/projects/{project_id}/files -X POST -H "Content-Type: application/json" -H "Authorization: Bearer {token}" -d '{json.dumps(file)}'""")
        result = stdout.read().decode()
        print(f"Added {file['path']}: {result[:100]}")
    
    # Export the zip
    print("\n=== Exporting zip ===")
    stdin, stdout, stderr = ssh.exec_command(f"""curl -s -o /tmp/test_module.zip http://localhost:8086/api/v1/projects/{project_id}/export -H "Authorization: Bearer {token}" -w "%{{http_code}}" """)
    http_code = stdout.read().decode()
    print(f"HTTP status: {http_code}")
    
    # Download and check the zip
    print("\n=== Downloading and checking zip ===")
    stdin, stdout, stderr = ssh.exec_command(f"""docker cp {CONTAINER}:/tmp/test_module.zip /tmp/test_module.zip 2>/dev/null || echo "File not found in container" """)
    print(stdout.read().decode())
    
    # Check if file exists on server
    stdin, stdout, stderr = ssh.exec_command("ls -la /tmp/test_module.zip 2>/dev/null || echo 'File not found'")
    print(stdout.read().decode())
    
    # List zip contents using python
    stdin, stdout, stderr = ssh.exec_command("""python3 -c "
import zipfile
import sys
try:
    with zipfile.ZipFile('/tmp/test_module.zip', 'r') as zip_ref:
        print('ZIP contents:')
        for info in zip_ref.infolist():
            print(f'  {info.filename} ({info.file_size} bytes)')
except Exception as e:
    print(f'Error: {e}')
" """)
    print(stdout.read().decode())

ssh.close()
print("\nDone!")
