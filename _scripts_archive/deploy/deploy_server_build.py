#!/usr/bin/env python3
"""Deploy security improvements - build on server, copy to container"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time
import os

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Step 1: Check if Go is installed on server
    print("=== Step 1: Check Go on server ===")
    out, err = run("which go && go version")
    print(f"Go: {out.strip()}")
    
    # Step 2: Check project location
    print("\n=== Step 2: Check project ===")
    out, err = run("ls -la /home/admin/ModuForge/backend/")
    print(f"Backend files:\n{out[:500]}")
    
    # Step 3: Check current binary in container
    print("\n=== Step 3: Check container binary ===")
    out, err = run(f"docker exec {CONTAINER} ls -la /app/")
    print(f"Container /app:\n{out[:500]}")
    
    # Step 4: Check go.mod
    print("\n=== Step 4: Check go.mod ===")
    out, err = run("cat /home/admin/ModuForge/backend/go.mod 2>/dev/null || echo 'No go.mod'")
    print(out[:300])
    
    # Step 5: Check main.go imports
    print("\n=== Step 5: Check main.go ===")
    out, err = run("head -30 /home/admin/ModuForge/backend/main.go 2>/dev/null || echo 'No main.go'")
    print(out[:500])
    
    client.close()

if __name__ == "__main__":
    main()
