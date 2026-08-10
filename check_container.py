#!/usr/bin/env python3
"""Check container state and try to fix it."""

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

# Check container logs
print("Container logs:")
run(f"docker logs {CONTAINER} --tail 5 2>&1")

# Stop container
print("\nStopping container...")
run(f"docker stop {CONTAINER}")

# Check if the binary exists and its size
print("\nChecking binary in container...")
run(f"docker inspect {CONTAINER}} --format='{{{{.GraphDriver.Data.MergedDir}}}}' 2>/dev/null || echo 'Cannot inspect'")

# Try to use docker commit to save the current state and rebuild
print("\nTrying to rebuild container...")
run(f"docker rm -f {CONTAINER} 2>/dev/null || echo 'Cannot remove'")

# Find the original image
print("\nFinding original image...")
run("docker images | grep moduforge")

# Check if we have a docker-compose file
print("\nChecking docker-compose...")
run("find /vol1/docker -name 'docker-compose*' -type f 2>/dev/null | head -5")

# Check if there's a backup of the container
print("\nChecking for container backups...")
run("docker ps -a | grep moduforge")

ssh.close()
print("\nDone!")
