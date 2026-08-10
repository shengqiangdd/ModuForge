#!/usr/bin/env python3
"""
Restart container with correct configuration.
"""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def restart():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop and remove
    print("1. Stopping and removing...")
    ssh.exec_command(f"docker stop {CONTAINER}")
    ssh.exec_command(f"docker rm {CONTAINER}")
    time.sleep(2)
    
    # Create with correct config
    print("\n2. Creating container with correct config...")
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
    
    print(f"  Running: docker run ...")
    stdin, stdout, stderr = ssh.exec_command(docker_run)
    output = stdout.read().decode().strip()
    error = stderr.read().decode().strip()
    
    if output:
        print(f"  Container ID: {output[:12]}")
    if error:
        print(f"  Error: {error}")
    
    # Wait
    print("\n3. Waiting for startup...")
    time.sleep(8)
    
    # Check
    print("\n4. Container status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Verify mounts
    print("\n5. Verifying mounts...")
    stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format='{{{{range .Mounts}}}}Type:{{.Type}} Src:{{.Source}} Dst:{{.Destination}}{{println}}{{end}}'")
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
    success = restart()
    if success:
        print("\n✅ Container is running!")
        # Deploy files
        import subprocess
        subprocess.run(["python", "ModuForge/deploy_local.py"])
    else:
        print("\n❌ Container failed to start")
    sys.exit(0 if success else 1)
