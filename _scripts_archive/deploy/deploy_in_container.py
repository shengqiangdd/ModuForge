#!/usr/bin/env python3
"""Deploy by building in container"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import os
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    print("1. Uploading source code...")
    
    # Upload the modified files
    files_to_upload = [
        ("backend/internal/agent/security.go", "/tmp/security.go"),
        ("backend/internal/agent/security_test.go", "/tmp/security_test.go"),
        ("backend/internal/agent/runner.go", "/tmp/runner.go"),
        ("backend/internal/handler/agent.go", "/tmp/agent_handler.go"),
        ("backend/internal/handler/routes.go", "/tmp/routes.go"),
        ("backend/internal/agent/skills/bash.go", "/tmp/bash.go"),
    ]
    
    sftp = client.open_sftp()
    for local_file, remote_file in files_to_upload:
        local_path = os.path.join(os.path.dirname(__file__), local_file)
        if os.path.exists(local_path):
            print(f"   Uploading {local_file}...")
            sftp.put(local_path, remote_file)
    sftp.close()
    
    print("\n2. Building in container...")
    
    # Copy files to container build directory
    for local_file, remote_file in files_to_upload:
        filename = os.path.basename(remote_file)
        stdin, stdout, stderr = client.exec_command(
            f"docker cp {remote_file} {CONTAINER}:/tmp/{filename}"
        )
        stdout.read()
    
    # Build in container
    build_cmd = """docker exec moduforge bash -c '
        cd /app && 
        cp /tmp/security.go internal/agent/security.go &&
        cp /tmp/security_test.go internal/agent/security_test.go &&
        cp /tmp/runner.go internal/agent/runner.go &&
        cp /tmp/agent_handler.go internal/handler/agent.go &&
        cp /tmp/routes.go internal/handler/routes.go &&
        cp /tmp/bash.go internal/agent/skills/bash.go &&
        go build -o /server ./cmd/moduforge &&
        echo "Build successful"
    '"""
    
    stdin, stdout, stderr = client.exec_command(build_cmd)
    out = stdout.read().decode()
    err = stderr.read().decode()
    print(f"   {out}")
    if err:
        print(f"   Error: {err[:500]}")
    
    print("\n3. Restarting container...")
    client.exec_command(f"docker restart {CONTAINER}")
    
    time.sleep(10)
    
    print("\n4. Testing...")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/health')
    print(f"   Health: {stdout.read().decode()}")
    
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/security/rules')
    print(f"   Security Rules: {stdout.read().decode()[:300]}")
    
    client.close()

if __name__ == "__main__":
    main()
