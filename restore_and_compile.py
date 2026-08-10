#!/usr/bin/env python3
"""Restore working binary and try a different approach."""

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
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=120)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out[-300:] if len(out) > 300 else out)
    if err: print(f"STDERR: {err[-300:] if len(err) > 300 else err}")
    print()
    return out

upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"

# 1. Restore working binary
print("1. Restoring working binary...")
run(f"cp /vol1/docker/overlay2/3kk6a3nvdgjd5dz2cgyh5dz5o/diff/server {upper_dir}/server", "Restore")
run(f"chmod +x {upper_dir}/server", "Permissions")

# 2. Start container
print("2. Starting...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# 3. Status
print("3. Status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

# 4. Now try to compile INSIDE the running container
# First, let's install gcc using a bind mount trick
print("\n4. Installing gcc via bind mount...")
run(f"docker run --rm --entrypoint sh -v /usr/bin/gcc:/host_gcc alpine sh -c 'ls -la /host_gcc 2>/dev/null || echo no gcc'")

# Try a different approach: compile using the container's go but with a separate gcc
print("\n5. Trying to compile with separate gcc container...")
# Use golang image which has go, and install gcc
run("""docker run --rm --entrypoint sh \
  -v /tmp/moduforge_src:/src \
  -v /tmp:/output \
  golang:1.24-alpine sh -c "
    apk add --no-cache gcc musl-dev 2>&1 | tail -3
    which gcc
    cd /src
    CGO_ENABLED=1 go build -o /output/server_cgo ./cmd/moduforge 2>&1 | tail -10
    ls -la /output/server_cgo 2>/dev/null || echo 'Build failed'
  " """, "Compile with golang:alpine")

# Check result
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Not found'")

ssh.close()
print("\nDone!")
