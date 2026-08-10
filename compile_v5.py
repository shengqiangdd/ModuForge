#!/usr/bin/env python3
"""Compile with explicit shell path."""

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

# Try with /bin/sh explicitly
print("Compiling with /bin/sh...")
run("""docker run --rm \
  -v /tmp/moduforge_src:/src \
  -v /tmp:/output \
  golang:1.24-alpine /bin/sh -c "apk add --no-cache gcc musl-dev 2>&1 | tail -3 && which gcc && cd /src && CGO_ENABLED=1 go build -o /output/server_cgo ./cmd/moduforge 2>&1 | tail -10" """, "Compile")

# Check
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Not found'")

ssh.close()
print("\nDone!")
