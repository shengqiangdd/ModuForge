#!/usr/bin/env python3
"""Compile with gcc full path."""

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
    if out: print(out[-500:] if len(out) > 500 else out)
    if err: print(f"STDERR: {err[-500:] if len(err) > 500 else err}")
    print()
    return out

# 1. Stop container
print("1. Stopping...")
run(f"docker stop {CONTAINER}")

# 2. Compile with full PATH
print("2. Compiling...")
run(f"""docker run --rm --entrypoint sh \
  -v /vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff/home/moduforge/backend:/src:ro \
  -v /tmp:/output \
  moduforge:latest -c "apk add --no-cache gcc musl-dev 2>/dev/null && export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin && which gcc && cd /src && CGO_ENABLED=1 CC=gcc go build -o /output/server_cgo ./cmd/moduforge 2>&1 | tail -20" """, "Compile")

# 3. Check
print("3. Checking...")
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Not found'")

# 4. Deploy
print("4. Deploying...")
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"
run(f"cp /tmp/server_cgo {upper_dir}/server && chmod +x {upper_dir}/server", "Deploy")

# 5. Start
print("5. Starting...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# 6. Status
print("6. Status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
