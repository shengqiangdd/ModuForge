#!/usr/bin/env python3
"""Wait for container and deploy"""
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
    
    # Wait for container to be healthy
    print("Waiting for container to be healthy...")
    for i in range(30):
        stdin, stdout, stderr = client.exec_command(
            f"docker inspect {CONTAINER} --format '{{{{.State.Health.Status}}}}'"
        )
        status = stdout.read().decode().strip()
        print(f"  [{i+1}] Status: {status}")
        if status == "healthy":
            break
        time.sleep(2)
    
    # Check health
    print("\nChecking health...")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/health')
    health = stdout.read().decode()
    print(f"Health: {health}")
    
    # Check container logs
    print("\nRecent logs:")
    stdin, stdout, stderr = client.exec_command('docker logs moduforge --tail 10 2>&1')
    print(stdout.read().decode())
    
    # Test security API
    print("\nTesting Security API:")
    stdin, stdout, stderr = client.exec_command('curl -s http://localhost:8086/api/v1/agent/security/rules')
    print(f"Rules: {stdout.read().decode()[:300]}")
    
    client.close()

if __name__ == "__main__":
    main()
