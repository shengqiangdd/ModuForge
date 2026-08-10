#!/usr/bin/env python3
"""
Restore with proper directory creation.
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
    stdin, stdout, stderr = ssh.exec_command(f"docker stop {CONTAINER} 2>/dev/null; docker rm {CONTAINER} 2>/dev/null; echo done")
    stdout.channel.recv_exit_status()
    time.sleep(2)
    
    # Create directory properly
    print("\n2. Creating working directory...")
    stdin, stdout, stderr = ssh.exec_command("mkdir -p /opt/moduforge")
    stdout.channel.recv_exit_status()
    time.sleep(1)
    
    # Verify directory exists
    stdin, stdout, stderr = ssh.exec_command("ls -la /opt/moduforge/")
    print(f"  Dir contents: {stdout.read().decode().strip()}")
    
    # Upload docker-compose.yml
    print("\n3. Uploading docker-compose.yml...")
    sftp = ssh.open_sftp()
    remote_compose = "/opt/moduforge/docker-compose.yml"
    remote_entrypoint = "/opt/moduforge/docker-entrypoint.sh"
    
    local_compose = os.path.join(PROJECT_DIR, "docker-compose.yml")
    local_entrypoint = os.path.join(PROJECT_DIR, "docker-entrypoint.sh")
    
    print(f"  Uploading {local_compose} -> {remote_compose}")
    sftp.put(local_compose, remote_compose)
    print(f"  Uploading {local_entrypoint} -> {remote_entrypoint}")
    sftp.put(local_entrypoint, remote_entrypoint)
    sftp.close()
    
    # Start with docker compose
    print("\n4. Starting with docker compose...")
    stdin, stdout, stderr = ssh.exec_command("cd /opt/moduforge && docker compose up -d 2>&1")
    exit_status = stdout.channel.recv_exit_status()
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"  Output: {output}")
    if error:
        print(f"  Error: {error}")
    
    time.sleep(10)
    
    # Check status
    print("\n5. Container status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    status = stdout.read().decode()
    print(f"  {status}")
    
    # Health check
    print("\n6. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    # If health check passes, deploy files
    if '"status":"ok"' in health:
        print("\n7. Deploying files...")
        import subprocess
        subprocess.run(["python", os.path.join(PROJECT_DIR, "deploy_local.py")], check=True)
        
        # Final health check
        time.sleep(3)
        stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
        health = stdout.read().decode()
        print(f"  Final Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = restore()
    sys.exit(0 if success else 1)
