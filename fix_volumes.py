#!/usr/bin/env python3
"""
Fix volume mount issue - the database directory doesn't exist.
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
    
    # Check volume contents
    print("\n2. Checking volume contents...")
    stdin, stdout, stderr = ssh.exec_command("docker volume inspect moduforge_data --format='{{.Mountpoint}}'")
    mountpoint = stdout.read().decode().strip()
    print(f"  Mountpoint: {mountpoint}")
    
    stdin, stdout, stderr = ssh.exec_command(f"ls -la {mountpoint}")
    print(f"  Contents: {stdout.read().decode()}")
    
    # Create database directory if needed
    print("\n3. Creating database directory...")
    ssh.exec_command(f"mkdir -p {mountpoint}")
    ssh.exec_command(f"touch {mountpoint}/.gitkeep")
    
    # Check if there's a database file
    stdin, stdout, stderr = ssh.exec_command(f"find {mountpoint} -name '*.db' 2>/dev/null")
    db_files = stdout.read().decode()
    print(f"  Database files: {db_files or 'none found'}")
    
    # Recreate container with correct path
    print("\n4. Recreating container...")
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
    
    stdin, stdout, stderr = ssh.exec_command(docker_run)
    print(f"  Container ID: {stdout.read().decode().strip()}")
    
    time.sleep(5)
    
    # Check status
    print("\n5. Checking status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Health check
    print("\n6. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = fix()
    sys.exit(0 if success else 1)
