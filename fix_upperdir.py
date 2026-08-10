#!/usr/bin/env python3
"""Copy original binary to UpperDir and restart."""

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
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()
    return out

# UpperDir from the inspection
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"

# 1. Copy original binary to UpperDir
print("1. Copying original binary to UpperDir...")
run(f"cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/moduforge-linux {upper_dir}/server", "Copy binary")
run(f"chmod +x {upper_dir}/server", "Set permissions")
run(f"ls -la {upper_dir}/server", "Verify binary")

# 2. Start container
print("2. Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# 3. Check status
print("3. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

# 4. If working, test the zip export
print("4. Testing zip export...")
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
print(f'Projects: {len(projects)}')

# Get first project ID
if projects:
    project_id = projects[0]['id']
    print(f'Testing with project: {project_id}')
    
    # Export zip
    req = urllib.request.Request(f'http://localhost:8086/api/v1/projects/{project_id}/export-zip', method='POST')
    req.add_header('Authorization', f'Bearer {token}')
    resp = urllib.request.urlopen(req)
    with open('/tmp/test_export.zip', 'wb') as f:
        f.write(resp.read())
    print('Export successful!')
    
    # Check zip contents
    import zipfile
    with zipfile.ZipFile('/tmp/test_export.zip', 'r') as z:
        print('Zip contents:')
        for info in z.infolist():
            print(f'  {info.filename}')
" """, "Test export")

ssh.close()
print("\nDone!")
