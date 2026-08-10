#!/usr/bin/env python3
"""Deep investigation of the binary's database path."""

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

base = f"docker run --rm --entrypoint sh -v /vol1/docker/volumes/moduforge_data/_data:/data {CONTAINER}"

# 1. Check all env vars the binary might read
run(f"{base} -c 'strings /server | grep -iE \"(db|database|data|path|sqlite)\" | sort -u | head -30'", "DB-related strings")

# 2. Try with DATABASE_PATH instead of DB_PATH
run(f"{base} -c 'DATABASE_PATH=/data/moduforge.db DB_PATH=/data/moduforge.db /server 2>&1 | head -5'", "Test with DATABASE_PATH")

# 3. Check if the binary reads from a config file
run(f"{base} -c 'strings /server | grep -iE \"(config|yaml|toml|json)\" | head -20'", "Config-related strings")

# 4. Check what file the binary actually tries to open
run(f"{base} -c 'strace -e trace=open,openat /server 2>&1 | grep -i database | head -10' || echo 'strace not available'", "strace (may not be available)")

# 5. Check if there's a /data directory creation issue
run(f"{base} -c 'ls -la /data/ && echo --- && file /data/moduforge.db 2>/dev/null || echo no db'", "Check /data after previous touch")

# 6. Actually test running the binary as uid 100 (moduforge)
run(f"{base} -c 'rm -f /data/moduforge.db && su -s /bin/sh moduforge -c \"DB_PATH=/data/moduforge.db /server 2>&1 | head -5\"'", "Run as moduforge user")

# 7. Check the actual default path
run(f"{base} -c 'strings /server | grep -E \"^/\" | head -20'", "Absolute paths in binary")

# 8. Check the data directory in the image
run(f"{base} -c 'ls -la /app/data/ 2>/dev/null || echo no /app/data'", "App data dir")

# 9. Check the docker-compose or Dockerfile for clues
run(f"docker inspect {CONTAINER} --format '{{{{.Config.Env}}}}'", "Full env from inspect")

# 10. Check if there's a different volume being mounted
run(f"docker inspect {CONTAINER} --format '{{{{json .Mounts}}}}'", "Full mounts")

ssh.close()
print("Done!")
