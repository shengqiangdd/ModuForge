#!/usr/bin/env python3
"""Compile in a temporary container with gcc and deploy."""

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, desc=""):
    print(f"=== {desc} ===")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=300)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out[-300:] if len(out) > 300 else out)
    if err: print(f"STDERR: {err[-300:] if len(err) > 300 else err}")
    print()
    return out

# 1. Stop the container
print("1. Stopping container...")
run(f"docker stop {CONTAINER}")

# 2. Create a temp container from the same image to compile
print("2. Creating temp compile container...")
run(f"""docker run --rm --name moduforge-compile \
  -v /vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff/home/moduforge/backend:/src \
  -v /tmp:/output \
  moduforge:latest sh -c "apk add --no-cache gcc musl-dev 2>&1 | tail -3 && cd /src && CGO_ENABLED=1 go build -o /output/server_cgo ./cmd/moduforge 2>&1 | tail -10" """, "Compile in temp container")

# 3. Check the binary
print("3. Checking binary...")
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Binary not found'")

# 4. Copy to the main container
print("4. Deploying to main container...")
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"
run(f"cp /tmp/server_cgo {upper_dir}/server", "Copy binary")
run(f"chmod +x {upper_dir}/server", "Set permissions")
run(f"ls -la {upper_dir}/server", "Verify")

# 5. Start container
print("5. Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# 6. Check status
print("6. Status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
