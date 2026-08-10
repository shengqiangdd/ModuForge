#!/usr/bin/env python3
"""Check entrypoint script and fix database path issue."""

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
    stdin, stdout, stderr = ssh.exec_command(f"docker run --rm -v /vol1/docker/volumes/moduforge_data/_data:/data {CONTAINER} sh -c '{cmd}'")
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()

# 1. Check entrypoint script
run("cat /docker-entrypoint.sh", "Entrypoint script")

# 2. Check DB_PATH
run("echo $DB_PATH", "DB_PATH env")

# 3. Check if /data directory exists in a fresh container
run("ls -la /data/", "/data in fresh container")

# 4. Test creating DB as moduforge user (uid 1000)
run("id && touch /data/test_perm.txt && echo 'Can write to /data' && rm /data/test_perm.txt", "Write test")

# 5. Check the actual database path the binary uses
run("grep -r 'moduforge.db\\|DB_PATH\\|database' /app/ 2>/dev/null | head -20", "DB references in binary")

# 6. Check if there's a data directory issue
run("ls -la /app/", "/app directory")

# 7. Check the binary
run("ls -la /app/server /server 2>/dev/null", "Binary location")

# 8. Check what happens when we run the entrypoint
run("sh -c 'export DB_PATH=/data/moduforge.db && echo \"DB_PATH=$DB_PATH\" && ls -la /data/ && /server --help 2>&1 | head -20'", "Test entrypoint")

ssh.close()
print("Done!")
