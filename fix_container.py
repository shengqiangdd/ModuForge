#!/usr/bin/env python3
"""
Fix and restore ModuForge container properly.
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
    
    # Stop all old containers
    print("1. Cleaning up old containers...")
    ssh.exec_command(f"docker stop {CONTAINER} 2>/dev/null")
    ssh.exec_command(f"docker rm {CONTAINER} 2>/dev/null")
    ssh.exec_command("docker stop vibrant_shannon 2>/dev/null")
    ssh.exec_command("docker rm vibrant_shannon 2>/dev/null")
    time.sleep(2)
    
    # Find docker-compose.yml
    print("\n2. Finding docker-compose.yml...")
    stdin, stdout, stderr = ssh.exec_command("find / -name 'docker-compose.yml' 2>/dev/null | head -5")
    print(stdout.read().decode())
    
    # Try to start with docker run using correct volumes
    print("\n3. Starting container with correct config...")
    docker_run = f"""docker run -d \
        --name {CONTAINER} \
        --restart unless-stopped \
        -p 8086:8080 \
        -e PORT=:8080 \
        -e DB_PATH=/data/moduforge.db \
        -e BUILD_DIR=/data/builds \
        -e MODULES_DIR=/data/modules \
        -e PROJECTS_DIR=/data/projects \
        -e GIN_MODE=release \
        -e MODUFORGE_DEV=0 \
        -e TZ=Asia/Shanghai \
        -v moduforge_data:/data \
        -v moduforge_uploads:/app/uploads \
        moduforge:latest"""
    
    print(f"  > {docker_run[:100]}...")
    stdin, stdout, stderr = ssh.exec_command(docker_run)
    exit_status = stdout.channel.recv_exit_status()
    output = stdout.read().decode()
    error = stderr.read().decode()
    
    if output:
        print(f"    {output}")
    if error:
        print(f"    Error: {error}")
    
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
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = fix()
    sys.exit(0 if success else 1)
