#!/usr/bin/env python3
"""Compile with overridden entrypoint."""

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

# 1. Stop the container
print("1. Stopping container...")
run(f"docker stop {CONTAINER}")

# 2. Compile with overridden entrypoint
print("2. Compiling in temp container...")
run(f"""docker run --rm --entrypoint sh \
  -v /vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff/home/moduforge/backend:/src:ro \
  -v /tmp:/output \
  moduforge:latest -c "apk add --no-cache gcc musl-dev 2>&1 | tail -3 && cd /src && CGO_ENABLED=1 go build -o /output/server_cgo ./cmd/moduforge 2>&1 | tail -20" """, "Compile")

# 3. Check binary
print("3. Checking binary...")
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Binary not found'")

# 4. Deploy
print("4. Deploying...")
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"
run(f"cp /tmp/server_cgo {upper_dir}/server && chmod +x {upper_dir}/server", "Deploy")
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
