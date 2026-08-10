#!/usr/bin/env python3
"""Copy source code to container, fix zipper.go, recompile and deploy."""

import paramiko
import time

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

# 1. Stop container
print("1. Stopping container...")
run(f"docker stop {CONTAINER}", "Stop")

# 2. Copy source code from overlay2 to container
print("2. Copying source code...")
run(f"docker cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend {CONTAINER}:/tmp/backend", "Copy backend source")

# 3. Start container
print("3. Starting container...")
run(f"docker start {CONTAINER}", "Start")
time.sleep(3)

# 4. Check if source code is there
print("4. Checking source code...")
run(f"docker exec {CONTAINER} ls -la /tmp/backend/", "Source code")

# 5. Copy fixed zipper.go
print("5. Copying fixed zipper.go...")
run(f"docker cp /tmp/zipper.go {CONTAINER}:/tmp/backend/internal/service/zipper.go", "Copy zipper.go")

# 6. Check the fixed file
print("6. Checking fixed file...")
run(f"docker exec {CONTAINER} head -50 /tmp/backend/internal/service/zipper.go", "Check zipper.go")

# 7. Compile
print("7. Compiling...")
run(f"docker exec {CONTAINER} sh -c 'cd /tmp/backend && go build -o /tmp/server ./cmd/moduforge'", "Compile")

# 8. Check binary
print("8. Checking binary...")
run(f"docker exec {CONTAINER} ls -la /tmp/server", "Binary")

# 9. Deploy binary
print("9. Deploying binary...")
run(f"docker exec {CONTAINER} cp /tmp/server /app/server", "Deploy")
run(f"docker exec {CONTAINER} chmod +x /app/server", "Permissions")

# 10. Restart
print("10. Restarting...")
run(f"docker restart {CONTAINER}", "Restart")
time.sleep(5)

# 11. Check status
print("11. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'", "Status")
run("curl -s http://192.168.2.9:8086/health", "Health")

ssh.close()
print("\nDone!")
