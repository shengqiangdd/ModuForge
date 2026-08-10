# -*- coding: utf-8 -*-
"""
Deploy: upload source to server, build with Docker, deploy.
The server has Docker with multi-stage build support.
"""
import paramiko
import time
import os
import tarfile
import io

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
WORKSPACE = r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, timeout=600):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode().strip()
    err = e.read().decode().strip()
    return out or err

# Step 1: Upload source code as tar
print("=== Step 1: Upload source ===")

# Create tar of backend + frontend + Dockerfile + entrypoint
tar_path = os.path.join(WORKSPACE, "source_upload.tar.gz")
with tarfile.open(tar_path, "w:gz") as tar:
    # Backend
    backend_dir = os.path.join(WORKSPACE, "backend")
    for root, dirs, files in os.walk(backend_dir):
        # Skip .mimocode, bin, binary, data, dist, docs
        dirs[:] = [d for d in dirs if d not in ['.mimocode', 'bin', 'binary', 'data', 'dist', 'docs', 'node_modules']]
        for f in files:
            if f.endswith('.exe') or f.endswith('.db') or f.endswith('.db-shm') or f.endswith('.db-wal'):
                continue
            fp = os.path.join(root, f)
            arcname = os.path.relpath(fp, WORKSPACE)
            tar.add(fp, arcname=arcname)
    
    # Frontend build output (dist)
    frontend_dist = os.path.join(WORKSPACE, "frontend", "dist")
    if os.path.exists(frontend_dist):
        for root, dirs, files in os.walk(frontend_dist):
            for f in files:
                fp = os.path.join(root, f)
                arcname = os.path.relpath(fp, WORKSPACE)
                tar.add(fp, arcname=arcname)
    
    # Dockerfile
    dockerfile = os.path.join(WORKSPACE, "backend", "Dockerfile")
    if os.path.exists(dockerfile):
        tar.add(dockerfile, arcname="backend/Dockerfile")
    
    # Entry point
    entrypoint = os.path.join(WORKSPACE, "backend", "docker-entrypoint.sh")
    if os.path.exists(entrypoint):
        tar.add(entrypoint, arcname="backend/docker-entrypoint.sh")

print("Tar created:", os.path.getsize(tar_path), "bytes")

# Upload
sftp = ssh.open_sftp()
sftp.put(tar_path, "/tmp/source_upload.tar.gz")
sftp.close()
print("Uploaded")

# Extract on server
run("rm -rf /tmp/moduforge-build && mkdir -p /tmp/moduforge-build")
run("cd /tmp/moduforge-build && tar xzf /tmp/source_upload.tar.gz")
print("Extracted")

# Check structure
print("Files:", run("find /tmp/moduforge-build -type f | head -20"))

# Step 2: Build with Docker
print("\n=== Step 2: Build ===")
result = run(
    "cd /tmp/moduforge-build && docker build -t moduforge:latest -f backend/Dockerfile . 2>&1",
    timeout=600
)
print("Build result:", result[-500:] if len(result) > 500 else result)

# Check if image exists
print("\nImage:", run("docker images moduforge:latest --format '{{.ID}} {{.Size}}'"))

# Step 3: Deploy
print("\n=== Step 3: Deploy ===")
run("docker rm -f %s 2>/dev/null" % CONTAINER)

result = run(
    'docker run -d --name %s '
    '--restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8087:8080 '
    'moduforge:latest' % CONTAINER
)
print("Run:", result)

# Wait and verify
print("\n=== Step 4: Verify ===")
time.sleep(10)

status = run('docker inspect %s --format "{{.State.Status}}"' % CONTAINER)
print("Status:", status)

logs = run("docker logs %s --tail 15 2>&1" % CONTAINER)
print("Logs:", logs)

health = run("curl -s http://localhost:8086/health")
print("Health (nginx):", health)

# Cleanup
os.remove(tar_path)
ssh.close()
print("\n=== Done ===")
