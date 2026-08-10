#!/usr/bin/env python3
"""Check container status after fix - with proper encoding."""

import paramiko
import sys

# Fix encoding for Windows
sys.stdout.reconfigure(encoding='utf-8', errors='replace')
sys.stderr.reconfigure(encoding='utf-8', errors='replace')

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

# 1. Container status
run(f"docker ps -f name={CONTAINER} --format '{{.Status}}'", "Container status")

# 2. Container logs (just the last few lines, avoid Chinese)
run(f"docker logs --tail 20 {CONTAINER} 2>&1 | cat", "Container logs")

# 3. Health check
run("curl -s http://192.168.2.9:8086/health", "Health check")

# 4. Check if DB was created
run(f"docker exec {CONTAINER} ls -la /data/moduforge.db 2>/dev/null || echo 'DB not found'", "DB file")

# 5. Test API
run("curl -s http://192.168.2.9:8086/api/v1/projects 2>/dev/null | head -500", "Projects API")

ssh.close()
print("Done!")
