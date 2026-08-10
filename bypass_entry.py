#!/usr/bin/env python3
"""Bypass entrypoint to inspect container filesystem."""

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

# Use --entrypoint sh to bypass the entrypoint
base = f"docker run --rm --entrypoint sh -v /vol1/docker/volumes/moduforge_data/_data:/data {CONTAINER}"

# 1. Check /data directory
run(f"{base} -c 'ls -la /data/'", "/data listing")

# 2. Check if moduforge user can write
run(f"{base} -c 'id && whoami'", "User info")

# 3. Check DB_PATH
run(f"{base} -c 'echo DB_PATH=$DB_PATH'", "DB_PATH")

# 4. Try to create the database file
run(f"{base} -c 'touch /data/moduforge.db && echo OK && rm /data/moduforge.db'", "Create DB test")

# 5. Check the entrypoint script
run(f"{base} -c 'cat /docker-entrypoint.sh'", "Entrypoint content")

# 6. Check the binary
run(f"{base} -c 'ls -la /app/server /server 2>/dev/null'", "Binary check")

# 7. Check what the binary expects
run(f"{base} -c 'strings /app/server 2>/dev/null | grep -i db_path | head -5' || echo 'strings not available'", "Binary strings")

# 8. Check the server binary for DATABASE references
run(f"{base} -c 'strings /server 2>/dev/null | grep -i database | head -10' || echo 'strings not available'", "Binary DB refs")

# 9. Check if the binary is static or dynamic
run(f"{base} -c 'file /server /app/server 2>/dev/null'", "Binary type")

# 10. Check what happens when we try to run the server directly
run(f"{base} -c 'DB_PATH=/data/moduforge.db /server --help 2>&1 | head -20'", "Server help")

ssh.close()
print("Done!")
