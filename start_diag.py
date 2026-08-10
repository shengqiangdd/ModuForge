#!/usr/bin/env python3
"""Start container and diagnose database issue."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, desc=""):
    print(f"=== {desc} ===")
    stdin, stdout, stderr = ssh.exec_command(f"{cmd}")
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()

# 1. Start container
print("Starting container...")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}")
print(stdout.read().decode())
print(stderr.read().decode())

import time
time.sleep(3)

# 2. Check if running
run(f"docker ps -f name={CONTAINER} --format '{{.Status}}'", "Container status")

# 3. Check /data directory
run(f"docker exec {CONTAINER} ls -la /data/", "/data directory")

# 4. Check if /data has proper permissions
run(f"docker exec {CONTAINER} stat /data/", "/data permissions")

# 5. Check entrypoint script
run(f"docker exec {CONTAINER} cat /docker-entrypoint.sh", "Entrypoint script")

# 6. Check if we can create the database directory
run(f"docker exec {CONTAINER} sh -c 'mkdir -p /data && touch /data/test_perm.txt && echo OK && rm /data/test_perm.txt'", "Write test")

# 7. Check the actual database path issue
run(f"docker exec {CONTAINER} ls -la /data/moduforge.db 2>/dev/null || echo 'DB file not found'", "DB file check")

# 8. Check if the binary can write to /data
run(f"docker exec {CONTAINER} sh -c 'touch /data/moduforge.db && rm /data/moduforge.db'", "Binary write test")

ssh.close()
print("Done!")
