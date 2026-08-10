# -*- coding: utf-8 -*-
"""
Build ModuForge image locally and deploy to server.
Steps:
1. Extract entrypoint from original image
2. Build new image locally with fixed binary
3. Save image, upload to server, load and deploy
"""
import paramiko
import subprocess
import time
import os

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
WORKSPACE = r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge"
BACKEND = os.path.join(WORKSPACE, "backend")

def run_local(cmd, cwd=None, timeout=120):
    result = subprocess.run(cmd, shell=True, cwd=cwd, capture_output=True, text=True, timeout=timeout)
    return result.returncode, result.stdout.strip(), result.stderr.strip()

def run_remote(ssh, cmd, timeout=60):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    return o.read().decode().strip() or e.read().decode().strip()

print("=== Step 1: Build binary ===")
rc, out, err = run_local("go build -o moduforge-server-linux ./cmd/moduforge", cwd=BACKEND)
if rc != 0:
    print("Build failed:", err)
    exit(1)
print("Binary built:", os.path.getsize(os.path.join(BACKEND, "moduforge-server-linux")), "bytes")

print("\n=== Step 2: Connect to server ===")
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
print("Connected")

print("\n=== Step 3: Extract entrypoint from original image ===")
# Create a temp container to extract the entrypoint
run_remote(ssh, "docker create --name temp-extract moduforge:latest 2>/dev/null")
run_remote(ssh, "docker cp temp-extract:/docker-entrypoint.sh /tmp/docker-entrypoint.sh")
run_remote(ssh, "docker rm temp-extract 2>/dev/null")
# Fix permissions on server side
run_remote(ssh, "chmod +x /tmp/docker-entrypoint.sh")
print("Entrypoint extracted and fixed")

print("\n=== Step 4: Upload binary ===")
sftp = ssh.open_sftp()
sftp.put(os.path.join(BACKEND, "moduforge-server-linux"), "/tmp/moduforge-server")
sftp.close()
print("Binary uploaded")

print("\n=== Step 5: Build image on server ===")
# Create Dockerfile on server
dockerfile = """FROM alpine:3.20
RUN apk add --no-cache wget ca-certificates tzdata openssl && \\
    addgroup -S moduforge && adduser -S moduforge -G moduforge
COPY server /server
RUN chmod +x /server
COPY entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
ENV PORT=:8080 DB_PATH=/data/moduforge.db BUILD_DIR=/data/builds MODULES_DIR=/data/modules PROJECTS_DIR=/data/projects GIN_MODE=release DIST_DIR=/app/dist
RUN mkdir -p /data /app/dist /app/uploads && chown -R moduforge:moduforge /data /app /app/uploads
EXPOSE 8080
USER moduforge
ENTRYPOINT ["/docker-entrypoint.sh"]
"""
run_remote(ssh, "mkdir -p /tmp/moduforge-build")
sftp = ssh.open_sftp()
# Create empty .dockerignore locally
with open(os.path.join(WORKSPACE, ".dockerignore_empty"), "w") as f:
    pass
sftp.put(os.path.join(WORKSPACE, ".dockerignore_empty"), "/tmp/moduforge-build/.dockerignore")
os.remove(os.path.join(WORKSPACE, ".dockerignore_empty"))
with sftp.open("/tmp/moduforge-build/Dockerfile", "w") as f:
    f.write(dockerfile)
sftp.close()

# Copy files to build context
run_remote(ssh, "cp /tmp/moduforge-server /tmp/moduforge-build/server")
run_remote(ssh, "cp /tmp/docker-entrypoint.sh /tmp/moduforge-build/entrypoint.sh")

# Build
result = run_remote(ssh, "cd /tmp/moduforge-build && docker build -t moduforge:patched .", timeout=180)
print("Build:", result[:500] if len(result) > 500 else result)

print("\n=== Step 6: Deploy ===")
run_remote(ssh, "docker stop %s 2>/dev/null" % CONTAINER)
run_remote(ssh, "docker rm %s 2>/dev/null" % CONTAINER)

result = run_remote(ssh,
    'docker run -d --name %s '
    '--restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8086:8080 '
    'moduforge:patched' % CONTAINER)
print("Container:", result)

print("\n=== Step 7: Verify ===")
time.sleep(8)

status = run_remote(ssh, 'docker inspect %s --format "{{.State.Status}}"' % CONTAINER)
print("Status:", status)

logs = run_remote(ssh, "docker logs %s --tail 20 2>&1" % CONTAINER)
print("Logs:", logs)

health = run_remote(ssh, "docker exec %s curl -s http://localhost:8080/health" % CONTAINER)
print("Health:", health)

fix_check = run_remote(ssh, 'docker exec %s strings /server | grep -c "WHERE name="' % CONTAINER)
print("Binary fix count:", fix_check)

ssh.close()
print("\n=== Done ===")
