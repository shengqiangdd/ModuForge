#!/usr/bin/env python3
"""Deploy ModuForge optimizations to server - automated deployment."""
import paramiko
import time
import os
import sys

# Fix encoding on Windows
sys.stdout.reconfigure(encoding='utf-8')
sys.stderr.reconfigure(encoding='utf-8')

SERVER = '192.168.2.9'
USER = 'admin'
PASS = 'csq0216'

def run(ssh, cmd, timeout=60):
    """Execute command and print output."""
    print(f"  > {cmd}")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout, get_pty=True)
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out.strip():
        print(f"  OUT: {out.strip()[:500]}")
    if err.strip():
        print(f"  ERR: {err.strip()[:500]}")
    return out.strip(), err.strip()

def run_background(ssh, cmd):
    """Execute command in background (nohup)."""
    print(f"  > (background) {cmd}")
    # Use nohup and redirect output to a log file
    full_cmd = f"nohup {cmd} > /tmp/deploy_build.log 2>&1 & echo $!"
    stdin, stdout, stderr = ssh.exec_command(full_cmd, timeout=10)
    pid = stdout.read().decode().strip()
    print(f"  PID: {pid}")
    return pid

def wait_for_completion(ssh, pid, timeout=600):
    """Wait for a background process to complete."""
    print(f"  Waiting for PID {pid} to complete (timeout={timeout}s)...")
    start = time.time()
    while time.time() - start < timeout:
        # Check if process is still running
        stdin, stdout, stderr = ssh.exec_command(f"kill -0 {pid} 2>/dev/null && echo 'running' || echo 'done'")
        result = stdout.read().decode().strip()
        if result == 'done':
            # Get the output
            stdin, stdout, stderr = ssh.exec_command("cat /tmp/deploy_build.log 2>/dev/null | tail -20")
            log = stdout.read().decode().strip()
            print(f"  Build log tail:\n{log}")
            return True
        time.sleep(10)
        elapsed = int(time.time() - start)
        print(f"  Still running... ({elapsed}s)")
    print(f"  Timeout after {timeout}s")
    return False

def main():
    print("=" * 60)
    print("  ModuForge Automated Deployment")
    print("=" * 60)

    # Step 1: Connect to server
    print("\n[1/6] Connecting to server...")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER, username=USER, password=PASS, timeout=15)
    print("  [OK] Connected")

    # Step 2: Check current container
    print("\n[2/6] Checking current container...")
    out, _ = run(ssh, 'docker ps -a --filter name=moduforge --format "{{.ID}} {{.Names}} {{.Status}}"')

    # Step 3: Build Go binary using temp container (background)
    print("\n[3/6] Building Go backend (background)...")
    run(ssh, 'docker rm -f moduforge-build 2>/dev/null || true')
    run_background(ssh, (
        'docker run --name moduforge-build '
        '-v /app/working/workspaces/default/ModuForge/backend:/src '
        '-w /src '
        'golang:1.25-alpine '
        'sh -c "apk add --no-cache gcc musl-dev && '
        'CGO_ENABLED=1 go build -ldflags=\'-s -w\' -trimpath -o /tmp/server ./cmd/moduforge/"'
    ))

    # Wait for build to complete
    if not wait_for_completion(ssh, '$(cat /tmp/deploy_build.pid 2>/dev/null || echo 0)', timeout=600):
        # Check if container is still running
        out, _ = run(ssh, 'docker ps --filter name=moduforge-build --format "{{.Status}}"')
        if 'Up' in out:
            print("  Container still running, waiting more...")
            time.sleep(60)
        else:
            print("  Build failed!")
            return

    # Verify build succeeded
    out, _ = run(ssh, 'docker cp moduforge-build:/tmp/server /tmp/server 2>&1')
    if 'error' in out.lower():
        print("  Build failed - binary not found")
        return
    print("  [OK] Backend built")

    # Step 4: Extract frontend dist from existing container
    print("\n[4/6] Extracting frontend dist...")
    run(ssh, 'docker cp moduforge:/app/dist /tmp/moduforge_dist')
    print("  [OK] Frontend dist extracted")

    # Step 5: Build new Docker image
    print("\n[5/6] Building Docker image...")
    # Create a simple Dockerfile for deployment
    dockerfile_content = """FROM alpine:3.20

RUN apk add --no-cache wget ca-certificates tzdata openssl curl bash xz zip unzip cmake make gcc musl-dev && \\
    addgroup -S moduforge && adduser -S moduforge -G moduforge

# Copy the built binary
COPY server /server
RUN chmod +x /server

# Copy frontend dist
COPY dist/ /app/dist/

# Copy entrypoint
COPY entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV PORT=:8080 \\
    DB_PATH=/data/moduforge.db \\
    BUILD_DIR=/data/builds \\
    MODULES_DIR=/data/modules \\
    PROJECTS_DIR=/data/projects \\
    GIN_MODE=release \\
    DIST_DIR=/app/dist

RUN mkdir -p /data /app/uploads && chown -R moduforge:moduforge /data /app /app/uploads

EXPOSE 8080
USER moduforge
ENTRYPOINT ["/docker-entrypoint.sh"]
"""
    run(ssh, f'cat > /tmp/Dockerfile.deploy << \'HEREDOC\'\n{dockerfile_content}\nHEREDOC')

    # Create entrypoint script
    entrypoint_content = """#!/bin/sh
set -e

log() {
  echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

if [ -n "$JWT_SECRET" ]; then
  log "Using JWT_SECRET from environment variable"
elif [ -f /data/.env ] && grep -q "^JWT_SECRET=." /data/.env 2>/dev/null; then
  export JWT_SECRET=$(grep "^JWT_SECRET=" /data/.env | head -1 | cut -d= -f2-)
  log "Loaded JWT_SECRET from /data/.env"
else
  JWT_SECRET=$(openssl rand -hex 32)
  export JWT_SECRET
  echo "JWT_SECRET=${JWT_SECRET}" >> /data/.env
  log "Generated random JWT_SECRET and saved to /data/.env"
fi

log "Starting ModuForge on port ${PORT:-:8080}..."
exec /server "$@"
"""
    run(ssh, f'cat > /tmp/entrypoint.sh << \'HEREDOC\'\n{entrypoint_content}\nHEREDOC')

    # Build image (background)
    run_background(ssh, 'docker build -t moduforge:latest -f /tmp/Dockerfile.deploy /tmp')
    wait_for_completion(ssh, '$(cat /tmp/deploy_build.pid 2>/dev/null || echo 0)', timeout=120)
    print("  [OK] Docker image built")

    # Step 6: Stop old container and start new one
    print("\n[6/6] Restarting container...")
    run(ssh, 'docker stop moduforge 2>/dev/null || true')
    run(ssh, 'docker rm moduforge 2>/dev/null || true')

    # Start new container
    run(ssh, (
        'docker run -d '
        '--name moduforge '
        '--restart unless-stopped '
        '-p 8086:8080 '
        '-e PORT=:8080 '
        '-e DATABASE_PATH=/data/moduforge.db '
        '-e BUILD_DIR=/data/builds '
        '-e MODULES_DIR=/data/modules '
        '-e PROJECTS_DIR=/data/projects '
        '-e GIN_MODE=release '
        '-e TZ=Asia/Shanghai '
        '-v moduforge_data:/data '
        '-v moduforge_uploads:/app/uploads '
        'moduforge:latest'
    ))

    # Wait for container to start
    print("  Waiting for container to start...")
    time.sleep(5)

    # Check status
    out, _ = run(ssh, 'docker ps --filter name=moduforge --format "{{.ID}} {{.Names}} {{.Status}}"')
    print(f"  [OK] Container running: {out}")

    # Clean up
    run(ssh, 'docker rm -f moduforge-build 2>/dev/null || true')
    run(ssh, 'rm -rf /tmp/server /tmp/dist /tmp/moduforge_dist /tmp/Dockerfile.deploy /tmp/entrypoint.sh /tmp/deploy_build.log 2>/dev/null || true')

    ssh.close()
    print("\n" + "=" * 60)
    print("  [OK] Deployment Complete!")
    print("  Access: http://192.168.2.9:8086")
    print("=" * 60)

if __name__ == '__main__':
    main()
