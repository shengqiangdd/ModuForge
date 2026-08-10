#!/usr/bin/env python3
"""Check container state and fix it."""

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

# Check if we have docker-compose
print("\nChecking for docker-compose...")
run("find /vol1 -name 'docker-compose*' -type f 2>/dev/null | head -10")

# Check the original image
print("\nDocker images:")
run("docker images | grep -i moduforge")

# List all containers
print("\nAll containers:")
run("docker ps -a | head -10")

# Try to restart with original image
print("\nTrying to restart with original settings...")
# First, let's check if the container was created with docker-compose
run("docker inspect moduforge --format='{{.Config.Image}}' 2>/dev/null || echo 'Cannot inspect'")

ssh.close()
print("\nDone!")
