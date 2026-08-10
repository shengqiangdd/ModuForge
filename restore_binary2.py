#!/usr/bin/env python3
"""Restore binary using docker cp with stopped container."""

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

# 1. Stop container
print("1. Stopping container...")
run(f"docker stop {CONTAINER}")
time.sleep(2)

# 2. Get the container's merged directory
print("2. Getting container filesystem path...")
merged_dir = run(f"docker inspect {CONTAINER} --format='{{{{.GraphDriver.Data.MergedDir}}}}'", "Merged dir")
merged_dir = merged_dir.strip()
print(f"Merged dir: {merged_dir}")

# 3. Copy the original binary directly to the merged directory
print("3. Copying original binary...")
run(f"cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/moduforge-linux {merged_dir}/server", "Copy binary")
run(f"chmod +x {merged_dir}/server", "Set permissions")
run(f"ls -la {merged_dir}/server", "Verify binary")

# 4. Start container
print("4. Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(8)

# 5. Check status
print("5. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
