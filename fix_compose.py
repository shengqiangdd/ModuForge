#!/usr/bin/env python3
"""Fix DATABASE_PATH in docker-compose.yml and restart."""

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
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()

# 1. Fix the docker-compose.yml
print("1. Fixing docker-compose.yml...")
sftp = ssh.open_sftp()
with sftp.open("/tmp/moduforge_deploy/docker-compose.yml", "r") as f:
    content = f.read().decode()
print("Before:")
print(content)

# Replace DB_PATH with DATABASE_PATH
content = content.replace("DB_PATH=/data/moduforge.db", "DATABASE_PATH=/data/moduforge.db")

with sftp.open("/tmp/moduforge_deploy/docker-compose.yml", "w") as f:
    f.write(content.encode())

sftp.close()
print("After:")
print(content)

# 2. Also fix the entrypoint to pass DATABASE_PATH
print("\n2. Checking entrypoint...")
sftp = ssh.open_sftp()
with sftp.open("/tmp/moduforge_deploy/docker-entrypoint.sh", "r") as f:
    entry_content = f.read().decode()
print(entry_content)
sftp.close()

# 3. Recreate the container with docker-compose
print("\n3. Recreating container with docker-compose...")
run("cd /tmp/moduforge_deploy && docker compose down", "Compose down")
run("cd /tmp/moduforge_deploy && docker compose up -d", "Compose up")

# 4. Wait and check status
import time
time.sleep(5)

print("\n4. Checking status...")
run(f"docker ps -f name={CONTAINER} --format 'table {{{{.Status}}}}\t{{{{.Ports}}}}'", "Container status")
run(f"docker logs --tail 5 {CONTAINER}", "Container logs")

# 5. Health check
print("\n5. Health check...")
run("curl -s http://192.168.2.9:8086/health", "Health check")

ssh.close()
print("\nDone!")
