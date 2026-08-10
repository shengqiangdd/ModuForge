#!/usr/bin/env python3
"""Test with a fresh volume - delete old one and create new."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop container
    print("1. Stopping container...")
    ssh.exec_command(f"docker stop {CONTAINER}")
    time.sleep(2)
    
    # Remove old volume
    print("\n2. Removing old volume...")
    ssh.exec_command("docker volume rm moduforge_data")
    ssh.exec_command("docker volume rm moduforge_uploads")
    time.sleep(1)
    
    # Create fresh volume
    print("\n3. Creating fresh volume...")
    ssh.exec_command("docker volume create moduforge_data")
    ssh.exec_command("docker volume create moduforge_uploads")
    time.sleep(1)
    
    # Start container
    print("\n4. Starting container...")
    cmd = f"""docker run -d \
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
    
    stdin, stdout, stderr = ssh.exec_command(cmd)
    container_id = stdout.read().decode().strip()
    print(f"  Container: {container_id[:12]}")
    
    time.sleep(8)
    
    # Check status
    print("\n5. Container status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Check logs
    print("\n6. Container logs...")
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} 2>&1 | tail -10")
    print(stdout.read().decode())
    
    # Health check
    print("\n7. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = test()
    sys.exit(0 if success else 1)
