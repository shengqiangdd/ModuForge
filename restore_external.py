#!/usr/bin/env python3
"""
Fix: Use the existing named volume, not new ones.
"""
import paramiko
import time
import os

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

# Fixed docker-compose that uses external volumes
DOCKER_COMPOSE_FIXED = """services:
  moduforge:
    image: moduforge:latest
    container_name: moduforge
    restart: unless-stopped
    ports:
      - "8086:8080"
    environment:
      - PORT=:8080
      - DB_PATH=/data/moduforge.db
      - BUILD_DIR=/data/builds
      - MODULES_DIR=/data/modules
      - PROJECTS_DIR=/data/projects
      - GIN_MODE=release
      - JWT_SECRET=${JWT_SECRET:-}
      - MODUFORGE_DEV=0
      - TZ=Asia/Shanghai
    volumes:
      - moduforge_data:/data
      - moduforge_uploads:/app/uploads
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  moduforge_data:
    external: true
  moduforge_uploads:
    external: true
"""

def restore():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop existing container
    print("1. Stopping existing containers...")
    ssh.exec_command(f"docker stop {CONTAINER} 2>/dev/null")
    ssh.exec_command(f"docker rm {CONTAINER} 2>/dev/null")
    time.sleep(2)
    
    # Upload fixed docker-compose
    print("\n2. Uploading fixed docker-compose.yml...")
    sftp = ssh.open_sftp()
    with sftp.open("/tmp/moduforge_deploy/docker-compose.yml", "w") as f:
        f.write(DOCKER_COMPOSE_FIXED)
    sftp.close()
    
    # Start with docker compose
    print("\n3. Starting with docker compose (external volumes)...")
    stdin, stdout, stderr = ssh.exec_command("cd /tmp/moduforge_deploy && docker compose up -d 2>&1")
    output = stdout.read().decode()
    print(f"  {output}")
    
    time.sleep(10)
    
    # Check status
    print("\n4. Container status...")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    status = stdout.read().decode()
    print(f"  {status}")
    
    # Health check
    print("\n5. Health check...")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = restore()
    sys.exit(0 if success else 1)
