#!/usr/bin/env python3
"""Compile using separate alpine container with proper gcc setup."""

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
    if out: print(out[-800:] if len(out) > 800 else out)
    if err: print(f"STDERR: {err[-800:] if len(err) > 800 else err}")
    print()
    return out

# 1. Stop
print("1. Stopping...")
run(f"docker stop {CONTAINER}")

# 2. Copy source to /tmp on host
print("2. Copying source...")
src_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff/home/moduforge/backend"
run(f"cp -r {src_dir} /tmp/moduforge_src", "Copy source")

# 3. Compile using golang:alpine image (has go already, just need gcc)
print("3. Compiling with golang:alpine...")
run("""docker run --rm \
  -v /tmp/moduforge_src:/src \
  -v /tmp:/output \
  golang:alpine sh -c "apk add --no-cache gcc musl-dev && cd /src && CGO_ENABLED=1 go build -o /output/server_cgo ./cmd/moduforge 2>&1 | tail -10" """, "Compile")

# 4. Check
print("4. Checking...")
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Not found'")

# 5. Deploy
print("5. Deploying...")
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"
run(f"cp /tmp/server_cgo {upper_dir}/server && chmod +x {upper_dir}/server", "Deploy")
run(f"ls -la {upper_dir}/server", "Verify")

# 6. Start
print("6. Starting...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# 7. Status
print("7. Status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
