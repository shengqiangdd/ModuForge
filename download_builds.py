#!/usr/bin/env python3
"""Download and analyze build artifacts from ModuForge."""

import paramiko
import os
import zipfile
from pathlib import Path

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

# 1. Find the build artifacts (zip files)
print("1. Looking for build artifacts...")
run(f"docker exec {CONTAINER} find /data -name '*.zip' -o -name '*.tar.gz' 2>/dev/null", "Build artifacts in /data")

# 2. Check the builds directory
run(f"docker exec {CONTAINER} ls -la /data/builds/ 2>/dev/null || echo 'No builds dir'", "Builds directory")

# 3. Check for any downloaded build artifacts
run("ls -la /tmp/*.zip /tmp/*.tar.gz 2>/dev/null || echo 'No local artifacts'", "Local artifacts")

# 4. Check the project storage
run(f"docker exec {CONTAINER} ls -la /data/storage/ 2>/dev/null", "Storage directory")

# 5. Find the AndroBoost-SmartTune project
run(f"docker exec {CONTAINER} find /data -name '*andro*' -o -name '*smart*' 2>/dev/null | head -20", "AndroBoost files")

# 6. Check the build tasks
run("curl -s http://192.168.2.9:8086/api/v1/builds -H 'Authorization: Bearer $(curl -s http://192.168.2.9:8086/api/v1/auth/login -X POST -H \"Content-Type: application/json\" -d \"{\\\"username\\\":\\\"admin\\\",\\\"password\\\":\\\"admin123\\\"}\" | python3 -c \"import sys,json; print(json.load(sys.stdin)['token'])\")' 2>/dev/null | head -500", "Build tasks API")

# 7. Check the project files for the AndroBoost project
run("curl -s http://192.168.2.9:8086/api/v1/projects -H \"Authorization: Bearer \$(curl -s http://192.168.2.9:8086/api/v1/auth/login -X POST -H \\\"Content-Type: application/json\\\" -d '{\\\"username\\\":\\\"admin\\\",\\\"password\\\":\\\"admin123\\\"}' | python3 -c \\\"import sys,json; print(json.load(sys.stdin)['token'])\\\")\" 2>/dev/null | head -500", "Projects API")

ssh.close()
print("Done!")
