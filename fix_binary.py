#!/usr/bin/env python3
"""Check which server binary is running and fix it."""

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

# Check which binary is running
print("Checking running process...")
run(f"docker exec {CONTAINER} ps aux | grep server")

# Check the entrypoint script
print("\nChecking entrypoint script...")
run(f"docker exec {CONTAINER} cat /docker-entrypoint.sh 2>/dev/null || cat /entrypoint.sh 2>/dev/null || echo 'No entrypoint found'")

# Check the Dockerfile CMD
print("\nChecking Dockerfile CMD...")
run("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/Dockerfile | grep -E 'CMD|ENTRYPOINT' | head -5")

# Stop container, replace binary, restart
print("\n=== Fixing binary ===")
run(f"docker stop {CONTAINER}", "Stop")

# Replace /server with the new binary
run(f"docker run --rm -v /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin:/bin_src alpine sh -c 'cp /bin_src/moduforge-linux /server_old && chmod +x /server_old'", "Backup old binary")
run(f"docker cp {CONTAINER}:/app/server /tmp/server_new", "Copy new binary")
run(f"docker run --rm -v /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff:/diff alpine sh -c 'cp /diff/backend/bin/moduforge-linux /diff/backend/bin/moduforge-linux.old'", "Backup overlay binary")
run(f"docker cp /tmp/server_new {CONTAINER}:/server", "Deploy new binary to /server")

# Start container
run(f"docker start {CONTAINER}", "Start")
time.sleep(5)

# Check status
print("\n=== Checking status ===")
run(f"docker exec {CONTAINER} ps aux | grep server", "Running process")
run(f"docker exec {CONTAINER} ls -la /server", "Binary size")
run("curl -s http://192.168.2.9:8086/health", "Health")

ssh.close()
print("\nDone!")
