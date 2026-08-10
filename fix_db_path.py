#!/usr/bin/env python3
"""Fix DATABASE_PATH env var and restart container."""

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

# 1. Stop the broken container
print("1. Stopping broken container...")
run(f"docker stop {CONTAINER}", "Stop")

# 2. Remove the broken container
print("2. Removing broken container...")
run(f"docker rm {CONTAINER}", "Remove")

# 3. Clean up the bad DB files from the volume
print("3. Cleaning up bad DB files...")
run("rm -f /vol1/docker/volumes/moduforge_data/_data/moduforge.db /vol1/docker/volumes/moduforge_data/_data/moduforge.db-shm /vol1/docker/volumes/moduforge_data/_data/moduforge.db-wal", "Clean DB files")

# 4. Find the docker-compose file or the run command
print("4. Looking for docker-compose file...")
run("find /vol1/docker/ -name 'docker-compose*' -o -name 'compose*' 2>/dev/null | head -10", "Find compose")

# 5. Check if there's a compose file in the moduforge directory
run("ls -la /vol1/docker/moduforge/ 2>/dev/null || echo 'No moduforge dir'", "Moduforge dir")

# 6. Check the current container's original run config
run(f"docker inspect {CONTAINER} 2>/dev/null | head -50 || echo 'Container removed'", "Original config")

# 7. Check docker-compose.yml in the project
run("ls -la /home/admin/moduforge/ 2>/dev/null || echo 'No home moduforge dir'", "Home dir")

# 8. Search for docker-compose files
run("find / -name 'docker-compose*' -not -path '/proc/*' -not -path '/sys/*' 2>/dev/null | head -10", "All compose files")

ssh.close()
print("Done!")
