#!/usr/bin/env python3
"""Restore original binary directly."""

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

# Stop container
print("Stopping container...")
run(f"docker stop {CONTAINER}")

# Copy the original binary from overlay2 to host
print("Copying original binary from overlay2...")
run("cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/moduforge-linux /tmp/moduforge-restore")
run("chmod +x /tmp/moduforge-restore")
run("ls -la /tmp/moduforge-restore")

# Copy to container (using docker cp with stopped container)
print("Copying to container...")
run(f"docker cp /tmp/moduforge-restore {CONTAINER}:/server")
run(f"docker exec {CONTAINER} chmod +x /server 2>/dev/null || echo 'chmod failed, trying alternative'")

# Start container
print("Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# Check status
print("Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
