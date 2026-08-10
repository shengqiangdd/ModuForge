#!/usr/bin/env python3
"""Find source code and recompile the server."""

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

# 1. Find the source code overlay
print("1. Finding source code...")
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/", "Source code directory")

# 2. Copy source code to temp location
print("2. Copying source code to temp...")
run("tar -czf /tmp/backend_source.tar.gz -C /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff backend/", "Create tarball")

# 3. Copy to container
print("3. Copying to container...")
run(f"docker cp /tmp/backend_source.tar.gz {CONTAINER}:/tmp/", "Copy tarball")

# 4. Extract in container
print("4. Extracting in container...")
run(f"docker exec {CONTAINER} mkdir -p /tmp/build && tar -xzf /tmp/backend_source.tar.gz -C /tmp/build/", "Extract")

# 5. Copy the fixed zipper.go
print("5. Copying fixed zipper.go...")
run(f"docker cp /tmp/zipper.go {CONTAINER}:/tmp/build/backend/internal/service/zipper.go", "Copy zipper.go")

# 6. Compile
print("6. Compiling...")
run(f"docker exec {CONTAINER} sh -c 'cd /tmp/build/backend && go build -o /tmp/server ./cmd/moduforge'", "Compile")

# 7. Deploy
print("7. Deploying...")
run(f"docker cp {CONTAINER}:/tmp/server /tmp/server_new", "Copy binary")
run(f"docker exec {CONTAINER} cp /tmp/server /app/server", "Deploy binary")
run(f"docker exec {CONTAINER} chmod +x /app/server", "Set permissions")

# 8. Restart
print("8. Restarting...")
run(f"docker restart {CONTAINER}", "Restart")

# 9. Wait and check
import time
time.sleep(5)

print("9. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'", "Status")
run("curl -s http://192.168.2.9:8086/health", "Health")

ssh.close()
print("\nDone!")
