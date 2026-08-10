#!/usr/bin/env python3
"""Recompile with CGO_ENABLED=1 and deploy."""

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
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()
    return out

# 1. Start the container (it will fail but we need it running for docker exec)
print("1. Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(3)

# 2. Check if gcc is available in the container
print("2. Checking gcc availability...")
run(f"docker exec {CONTAINER} which gcc 2>/dev/null || echo 'gcc not found'")
run(f"docker exec {CONTAINER} gcc --version 2>/dev/null | head -1 || echo 'gcc not available'")

# 3. Try to compile with CGO_ENABLED=1
print("3. Compiling with CGO_ENABLED=1...")
run(f"docker exec {CONTAINER} sh -c 'cd /home/moduforge/backend && CGO_ENABLED=1 CC=gcc go build -o /tmp/server_cgo ./cmd/moduforge 2>&1'")

# 4. Check if compilation succeeded
print("4. Checking compiled binary...")
run(f"docker exec {CONTAINER} ls -la /tmp/server_cgo 2>/dev/null || echo 'Compilation failed'")

# 5. If compilation succeeded, deploy
run(f"docker exec {CONTAINER} sh -c 'if [ -f /tmp/server_cgo ]; then cp /tmp/server_cgo /server && chmod +x /server && echo Deployed successfully; fi'")

# 6. Restart container
print("6. Restarting container...")
run(f"docker restart {CONTAINER}")
time.sleep(8)

# 7. Check status
print("7. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
