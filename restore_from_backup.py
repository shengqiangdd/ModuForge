#!/usr/bin/env python3
"""
Restore the working container state by copying docker-compose.yml to server
and using docker compose up.
"""
import paramiko
import time
import os

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
PROJECT_DIR = os.path.dirname(os.path.abspath(__file__))

def restore():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop all moduforge containers
    print("1. Stopping all containers...")
    ssh.exec_command(f"docker stop {CONTAINER} 2>/dev/null")
    ssh.exec_command(f"docker rm {CONTAINER} 2>/dev/null")
    time.sleep(2)
    
    # Upload docker-compose.yml
    print("\n2. Uploading docker-compose.yml...")
    sftp = ssh.open_sftp()
    sftp.put(os.path.join(PROJECT_DIR, "docker-compose.yml"), "/tmp/moduforge-docker-compose.yml")
    sftp.close()
    
    # Upload entrypoint
    print("3. Uploading entrypoint...")
    sftp = ssh.open_sftp()
    sftp.put(os.path.join(PROJECT_DIR, "docker-entrypoint.sh"), "/tmp/docker-entrypoint.sh")
    sftp.close()
    
    # Create a working directory on server
    print("\n4. Setting up working directory...")
    commands = [
        "mkdir -p /opt/moduforge",
        "cp /tmp/moduforge-docker-compose.yml /opt/moduforge/docker-compose.yml",
        "cp /tmp/docker-entrypoint.sh /opt/moduforge/docker-entrypoint.sh",
    ]
    for cmd in commands:
        ssh.exec_command(cmd)
    
    # Check if docker compose is available
    print("\n5. Checking docker compose...")
    stdin, stdout, stderr = ssh.exec_command("docker compose version 2>/dev/null || docker-compose version 2>/dev/null || echo 'no compose'")
    print(f"  {stdout.read().decode().strip()}")
    
    # Try running with docker compose
    print("\n6. Starting with docker compose...")
    stdin, stdout, stderr = ssh.exec_command("cd /opt/moduforge && docker compose up -d 2>&1")
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"  Output: {output}")
    if error:
        print(f"  Error: {error}")
    
    time.sleep(5)
    
    # Check status
    print("\n7. Container status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Health check
    print("\n8. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = restore()
    sys.exit(0 if success else 1)
