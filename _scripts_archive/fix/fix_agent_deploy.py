#!/usr/bin/env python3
"""Deploy moduforge binary + fix agent custom provider"""
import paramiko, sys, os

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
LOCAL_BIN = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"
REMOTE_TMP = "/tmp/moduforge-server"
CONTAINER = "moduforge"
CONTAINER_BIN = "/app/moduforge-server"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)
print("✓ Connected")

# Upload binary
sftp = ssh.open_sftp()
sftp.put(LOCAL_BIN, REMOTE_TMP)
sftp.close()
print("✓ Uploaded binary")

# Copy into container and restart
cmds = [
    f"docker cp {REMOTE_TMP} {CONTAINER}:{CONTAINER_BIN}",
    f"docker exec {CONTAINER} chmod +x {CONTAINER_BIN}",
    f"docker restart {CONTAINER}",
]
for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
    out = stdout.read().decode()
    err = stderr.read().decode()
    print(f"  $ {cmd}")
    if err.strip():
        print(f"    stderr: {err.strip()}")
    print(f"    ✓ done")

import time
print("Waiting 5s for container to start...")
time.sleep(5)

# Health check
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
out = stdout.read().decode().strip()
print(f"  Health: {out}")

# Check logs for custom provider resolution
stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 20 2>&1", timeout=10)
logs = stdout.read().decode()
print(f"\n--- Last 20 lines of container logs ---")
print(logs)

ssh.close()
print("✓ Done")
