#!/usr/bin/env python3
"""Test the zip export with the working server."""

import paramiko
import json
import zipfile

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace')

# Get token
token = json.loads(run("""curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'""")).get('token', '')

# Get first project
projects = json.loads(run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"'))
print(f"Found {len(projects)} projects")

if projects:
    pid = projects[0]['id']
    print(f"Testing project: {projects[0]['name']} ({pid})")

    # Export zip
    run(f"""curl -s -o /tmp/export_test.zip http://localhost:8086/api/v1/projects/{pid}/export-zip -X POST -H "Authorization: Bearer {token}" """)

    # Copy to host
    run(f"docker cp moduforge:/tmp/export_test.zip /tmp/export_test.zip")

# List files
print("\nDownloading and analyzing...")
sftp = ssh.open_sftp()
sftp.get("/tmp/export_test.zip", "C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts/export_test.zip")
sftp.close()

# Analyze
with zipfile.ZipFile("C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts/export_test.zip", 'r') as z:
    print("\nZIP contents:")
    for info in z.infolist():
        print(f"  {info.filename} ({info.file_size} bytes)")

    print("\n--- webroot check ---")
    has_webroot = any('webroot/' in i.filename for i in z.infolist())
    print(f"  Has webroot: {has_webroot}")

    print("\n--- exclusion check ---")
    bad = ['DESIGN_DOC', 'tmp/', 'src/', '.build_cache', 'app/backend']
    for b in bad:
        found = any(b in i.filename for i in z.infolist())
        print(f"  {b}: {'FOUND (BAD!)' if found else 'excluded (GOOD)'}")

ssh.close()
