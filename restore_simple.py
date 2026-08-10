#!/usr/bin/env python3
"""
Simple restore - upload files to /tmp and use docker compose from there.
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
    
    # Create directory in /tmp (writable by admin)
    print("\n2. Creating working directory...")
    stdin, stdout, stderr = ssh.exec_command("mkdir -p /tmp/moduforge_deploy")
    stdout.channel.recv_exit_status()
    time.sleep(1)
    
    # Verify
    stdin, stdout, stderr = ssh.exec_command("ls -la /tmp/moduforge_deploy/")
    print(f"  {stdout.read().decode().strip()}")
    
    # Upload files
    print("\n3. Uploading files...")
    sftp = ssh.open_sftp()
    
    files = [
        ("docker-compose.yml", "/tmp/moduforge_deploy/docker-compose.yml"),
        ("docker-entrypoint.sh", "/tmp/moduforge_deploy/docker-entrypoint.sh"),
    ]
    
    for local_name, remote_path in files:
        local_path = os.path.join(PROJECT_DIR, local_name)
        print(f"  Uploading {local_name}...")
        sftp.put(local_path, remote_path)
    
    sftp.close()
    print("  Files uploaded!")
    
    # Start with docker compose
    print("\n4. Starting with docker compose...")
    stdin, stdout, stderr = ssh.exec_command("cd /tmp/moduforge_deploy && docker compose up -d 2>&1")
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
