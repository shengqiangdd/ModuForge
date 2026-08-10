#!/usr/bin/env python3
"""
Fix database - remove WAL files and try fresh start.
"""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def fix():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop container
    print("1. Stopping container...")
    ssh.exec_command(f"docker stop {CONTAINER}")
    time.sleep(2)
    
    # Backup and clean database files
    print("\n2. Cleaning database files...")
    commands = [
        "cd /vol1/docker/volumes/moduforge_data/_data",
        "rm -f moduforge.db-shm moduforge.db-wal",
        "cp moduforge.db moduforge.db.backup",
    ]
    for cmd in commands:
        ssh.exec_command(cmd)
        print(f"  {cmd}")
    
    # Start container
    print("\n3. Starting container...")
    ssh.exec_command(f"docker start {CONTAINER}")
    time.sleep(5)
    
    # Check status
    print("\n4. Checking status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Health check
    print("\n5. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    # Check logs
    print("\n6. Container logs...")
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} 2>&1 | tail -10")
    print(stdout.read().decode())
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = fix()
    sys.exit(0 if success else 1)
