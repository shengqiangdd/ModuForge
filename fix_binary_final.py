#!/usr/bin/env python3
"""Copy the correct working binary from another overlay layer."""

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

# Stop the container
print("Stopping container...")
run(f"docker stop {CONTAINER}")

# Use one of the working binaries from another container
# The binary at /vol1/docker/overlay2/3kk6a3nvdgjd5dz2cgyh5dz5o/diff/server (17741368 bytes) should work
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"

print("Copying working binary...")
run(f"cp /vol1/docker/overlay2/3kk6a3nvdgjd5dz2cgyh5dz5o/diff/server {upper_dir}/server", "Copy working binary")
run(f"chmod +x {upper_dir}/server", "Set permissions")
run(f"ls -la {upper_dir}/server", "Verify binary")

# Start container
print("Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# Check status
print("Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

# Check container logs
print("\nContainer logs:")
run(f"docker logs {CONTAINER} --tail 5 2>&1")

ssh.close()
print("\nDone!")
