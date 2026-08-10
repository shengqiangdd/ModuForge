#!/usr/bin/env python3
"""
Final fix - check permissions and start container.
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
    
    # Check if any container is running
    print("1. Checking running containers...")
    stdin, stdout, stderr = ssh.exec_command("docker ps -a | grep moduforge")
    print(stdout.read().decode())
    
    # Stop all moduforge containers
    print("\n2. Stopping all containers...")
    ssh.exec_command("docker stop $(docker ps -q --filter name=moduforge) 2>/dev/null")
    ssh.exec_command("docker rm $(docker ps -aq --filter name=moduforge) 2>/dev/null")
    time.sleep(2)
    
    # Fix volume permissions
    print("\n3. Fixing volume permissions...")
    stdin, stdout, stderr = ssh.exec_command("docker volume inspect moduforge_data --format='{{.Mountpoint}}'")
    mountpoint = stdout.read().decode().strip()
    
    # Ensure proper permissions
    ssh.exec_command(f"chmod -R 777 {mountpoint}")
    ssh.exec_command(f"chown -R 1000:1001 {mountpoint}")
    
    # Start fresh container
    print("\n4. Starting container...")
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
    container_id = stdout.read().decode().strip()
    error = stderr.read().decode()
    
    if error:
        print(f"  Error: {error}")
    
    print(f"  Container: {container_id[:12]}")
    
    # Wait for startup
    print("\n5. Waiting for startup...")
    time.sleep(8)
    
    # Check status
    print("\n6. Container status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Check logs
    print("\n7. Container logs...")
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} 2>&1 | tail -10")
    print(stdout.read().decode())
    
    # Health check
    print("\n8. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    # Deploy frontend and backend
    if '"status":"ok"' not in health:
        print("\n9. Deploying files...")
        import subprocess
        subprocess.run(["python", "ModuForge/deploy_local.py"], check=True)
        
        # Final check
        time.sleep(3)
        stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
        health = stdout.read().decode()
        print(f"  Final Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = fix()
    sys.exit(0 if success else 1)
