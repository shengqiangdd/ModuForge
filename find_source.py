#!/usr/bin/env python3
"""Find and copy source code properly."""

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
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()
    return out

# 1. Check the overlay2 directory structure
print("1. Checking overlay2...")
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/", "Overlay diff")

# 2. Check if backend exists
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/ 2>/dev/null || echo 'No backend dir'", "Backend dir")

# 3. Find the backend source
run("find /vol1/docker/overlay2 -name 'go.mod' -path '*/backend/*' 2>/dev/null | head -5", "Find go.mod")

# 4. Check the Dockerfile to understand the build process
run("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/Dockerfile 2>/dev/null | head -50", "Dockerfile")

# 5. Check if there's a build cache or compiled binary
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/bin/ 2>/dev/null", "Bin directory")

# 6. Check the actual container filesystem
run(f"docker exec {CONTAINER} find / -name 'server' -type f 2>/dev/null | head -5", "Find server binary")

# 7. Check where the server binary is
run(f"docker exec {CONTAINER} ls -la /app/", "App directory")

# 8. Check if we can find the source code in the container
run(f"docker exec {CONTAINER} find / -name 'go.mod' -type f 2>/dev/null | head -5", "Find go.mod in container")

ssh.close()
print("Done!")
