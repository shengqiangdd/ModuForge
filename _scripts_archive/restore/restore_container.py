#!/usr/bin/env python3
"""Restore container from original image"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    print("1. Stopping container...")
    client.exec_command(f"docker stop {CONTAINER}")
    time.sleep(3)
    
    print("2. Removing container...")
    client.exec_command(f"docker rm {CONTAINER}")
    time.sleep(2)
    
    print("3. Starting fresh container from image...")
    # Start container with volume mount to preserve data
    cmd = f"""docker run -d \
        --name {CONTAINER} \
        -p 8086:8080 \
        -v /home/admin/moduforge_data:/data \
        moduforge:latest"""
    
    stdin, stdout, stderr = client.exec_command(cmd)
    out = stdout.read().decode()
    err = stderr.read().decode()
    print(f"   Container ID: {out[:12]}")
    if err:
        print(f"   Error: {err}")
    
    print("\n4. Waiting for container to start...")
    time.sleep(10)
    
    print("5. Checking health...")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/health')
    health = stdout.read().decode()
    print(f"   Health: {health}")
    
    print("\n6. Recent logs:")
    stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 15 2>&1')
    print(stdout.read().decode())
    
    client.close()

if __name__ == "__main__":
    main()
