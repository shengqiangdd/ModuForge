#!/usr/bin/env python3
"""Check the zip creation code in the backend."""

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

# 1. Find the zipper code
run(f"docker exec {CONTAINER} find /app -name '*zip*' -o -name '*package*' 2>/dev/null | head -20", "Zip-related files")

# 2. Check if there's source code in the container
run(f"docker exec {CONTAINER} ls -la /app/", "App directory")

# 3. Check the overlay2 for source code
run("ls /vol1/docker/overlay2/ | head -5", "Overlay2 layers")

# 4. Find zipper.go in overlay2
run("find /vol1/docker/overlay2 -name 'zipper.go' -o -name 'ziputil.go' 2>/dev/null | head -5", "Find zipper source")

# 5. Check the specific layer we found earlier
run("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go 2>/dev/null | head -100", "zipper.go")

# 6. Check ziputil.go
run("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/builder/ziputil.go 2>/dev/null | head -100", "ziputil.go")

ssh.close()
print("Done!")
