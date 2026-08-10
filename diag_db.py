#!/usr/bin/env python3
"""Diagnose database path and permissions in ModuForge container."""

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
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} sh -c '{cmd}'")
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()

# 1. Check entrypoint script
run("cat /docker-entrypoint.sh 2>/dev/null || cat /entrypoint.sh 2>/dev/null || echo 'No entrypoint found'", "Entrypoint script")

# 2. Check environment variables
run("env | grep -i 'db\\|data\\|sqlite\\|moduforge' | sort", "Environment variables")

# 3. Check what the entrypoint does with DATABASE_PATH
run("grep -r 'DATABASE\\|/data\\|db_path' /docker-entrypoint.sh /entrypoint.sh /app/ 2>/dev/null | head -30", "Database path references")

# 4. Check /data directory
run("ls -la /data/ 2>/dev/null && echo '---' && stat /data/ 2>/dev/null", "/data directory")

# 5. Test writing to /data as moduforge user (uid 1000)
run("su -s /bin/sh -c 'touch /data/test_write.txt && echo OK && rm /data/test_write.txt' 1000", "Write test as uid 1000")

# 6. Check if /data has proper permissions
run("ls -la / | grep data", "/ (root) listing")

# 7. Check the docker-compose or run command
print("=== Docker inspect (Volumes + Entrypoint) ===")
stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format '{{{{.Config.Entrypoint}}}} {{{{.Config.Cmd}}}} {{{{.Config.Env}}}}'")
print(stdout.read().decode())

# 8. Check volume mounts
print("\n=== Volume mounts ===")
stdin, stdout, stderr = ssh.exec_command(f"docker inspect {CONTAINER} --format '{{{{range .Mounts}}}}Source: {{{{.Source}}}} -> Dest: {{{{.Destination}}}} (RW: {{{{.RW}}}}){{{{println}}}}{{{{end}}}}'")
print(stdout.read().decode())

ssh.close()
print("Done!")
