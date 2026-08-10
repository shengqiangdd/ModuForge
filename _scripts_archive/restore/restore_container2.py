#!/usr/bin/env python3
"""Restore container from original image - fix permissions"""
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
    
    print("1. Stopping and removing container...")
    client.exec_command(f"docker stop {CONTAINER} 2>/dev/null")
    time.sleep(2)
    client.exec_command(f"docker rm {CONTAINER} 2>/dev/null")
    time.sleep(2)
    
    print("2. Fixing data directory permissions...")
    client.exec_command("sudo chown -R 1000:1001 /home/admin/moduforge_data 2>/dev/null || true")
    client.exec_command("sudo chmod -R 755 /home/admin/moduforge_data 2>/dev/null || true")
    
    print("3. Starting fresh container...")
    # Don't mount volume, let container create its own
    cmd = f"""docker run -d \
        --name {CONTAINER} \
        -p 8086:8080 \
        moduforge:latest"""
    
    stdin, stdout, stderr = client.exec_command(cmd)
    out = stdout.read().decode()
    err = stderr.read().decode()
    print(f"   Container ID: {out[:12]}")
    if err:
        print(f"   Error: {err}")
    
    print("\n4. Waiting for container to start...")
    time.sleep(15)
    
    print("5. Checking health...")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/health')
    health = stdout.read().decode()
    print(f"   Health: {health}")
    
    print("\n6. Recent logs:")
    stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 20 2>&1')
    print(stdout.read().decode())
    
    # Test existing endpoints
    print("\n7. Testing existing endpoints:")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/skills -H "Authorization: Bearer test"')
    print(f"   Skills: {stdout.read().decode()[:200]}")
    
    client.close()

if __name__ == "__main__":
    main()
