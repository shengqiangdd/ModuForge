#!/usr/bin/env python3
"""Stop container and check volume directly."""

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

# 1. Stop container
print("Stopping container...")
run(f"docker stop {CONTAINER}", "Stop")

# 2. Check volume directory directly
print("\n=== Volume directory (host) ===")
run("ls -la /vol1/docker/volumes/moduforge_data/_data/", "Volume listing")

# 3. Check if DB file exists
run("ls -la /vol1/docker/volumes/moduforge_data/_data/moduforge.db 2>/dev/null || echo 'DB not found'", "DB file")

# 4. Check permissions
run("stat /vol1/docker/volumes/moduforge_data/_data/", "Volume permissions")

# 5. Try to create the database file
print("\n=== Create test file as admin ===")
run("touch /vol1/docker/volumes/moduforge_data/_data/test_admin.txt && echo 'Admin can write' && rm /vol1/docker/volumes/moduforge_data/_data/test_admin.txt", "Admin write test")

# 6. Check .env file
run("cat /vol1/docker/volumes/moduforge_data/_data/.env", ".env file")

# 7. Check entrypoint script in the image
print("\n=== Image entrypoint ===")
run(f"docker inspect {CONTAINER} --format '{{{{.Config.Entrypoint}}}}'", "Entrypoint")

# 8. Check if there's a docker-compose file
run("ls -la /vol1/docker/volumes/moduforge_data/ 2>/dev/null", "Parent directory")

ssh.close()
print("\nDone!")
