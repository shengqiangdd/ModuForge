#!/usr/bin/env python3
"""Check actual artifacts and download via docker cp."""

import paramiko
import os
import zipfile
from pathlib import Path

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
LOCAL_DIR = Path("C:/Users/22875/.qwenpaw/workspaces/default/ModuForge/build_artifacts")

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

# 1. List actual artifacts with full details
run(f"docker exec {CONTAINER} ls -la /data/storage/artifacts/", "Artifacts directory")

# 2. Check build cache
run(f"docker exec {CONTAINER} find /data/storage/build-cache -type f -name '*.zip' -exec ls -la {{}} \\;", "Build cache files")

# 3. Copy the latest build cache to a temp location
run(f"docker exec {CONTAINER} cp /data/storage/build-cache/1785249992652501794-1864/f10c08f818285d7bee1a54f7260f4d0e16e0f40e0e15741b2859900c4e872ef9.zip /tmp/build_cache_latest.zip", "Copy latest build cache")

# 4. Copy to host
run(f"docker cp {CONTAINER}:/tmp/build_cache_latest.zip /tmp/build_cache_latest.zip", "Copy to host")

# 5. Also check the output.zip
run(f"docker exec {CONTAINER} ls -la /data/storage/projects/output.zip", "Output.zip")

# 6. Copy output.zip to host
run(f"docker cp {CONTAINER}:/data/storage/projects/output.zip /tmp/output.zip", "Copy output.zip")

# 7. List what's in the project directory
run(f"docker exec {CONTAINER} ls -la /data/storage/projects/1785249992652501794-1864/", "Project directory")

ssh.close()
print("Done! Check /tmp/ on server for files.")
