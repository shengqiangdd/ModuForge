#!/usr/bin/env python3
"""Fix DATABASE_PATH and recreate container."""

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

# 1. Check the docker-compose file
run("cat /tmp/moduforge_deploy/docker-compose.yml 2>/dev/null || echo 'Not found'", "Docker-compose file")

# 2. Check the original container's full config
run(f"docker inspect moduforge 2>/dev/null | python3 -c 'import sys,json; c=json.load(sys.stdin); print(json.dumps(c[0][\"Config\"][\"Env\"],indent=2))' 2>/dev/null || echo 'Container not found'", "Original env vars")

# 3. Check if there's a run script
run("ls -la /tmp/moduforge_deploy/", "Deploy directory")

# 4. Check the entrypoint script for DATABASE_PATH
run("docker run --rm --entrypoint sh moduforge:latest -c 'cat /docker-entrypoint.sh'", "Entrypoint script")

# 5. Check the binary for DATABASE_PATH
run("docker run --rm --entrypoint sh moduforge:latest -c 'strings /server | grep -i DATABASE_PATH | head -5'", "Binary DATABASE_PATH")

# 6. Check the binary for DB_PATH
run("docker run --rm --entrypoint sh moduforge:latest -c 'strings /server | grep -i DB_PATH | head -5'", "Binary DB_PATH")

# 7. Check if the binary reads from a config file
run("docker run --rm --entrypoint sh moduforge:latest -c 'strings /server | grep -iE \"database_path|db_path\" | head -10'", "Binary path refs")

ssh.close()
print("Done!")
