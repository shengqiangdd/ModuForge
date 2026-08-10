#!/usr/bin/env python3
"""Fix permissions and recompile."""

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

# 2. Fix permissions on the source code
print("2. Fixing permissions...")
run("chmod -R 755 /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/", "Fix permissions")

# 3. Copy source code to a location the container can access
print("3. Copying source code...")
run(f"docker cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend {CONTAINER}:/home/moduforge/backend", "Copy backend source")

# 4. Fix permissions in container
print("4. Fixing container permissions...")
run(f"docker run --rm --entrypoint sh -v /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend:/src {CONTAINER} -c 'chmod -R 777 /src && cp -r /src /home/moduforge/backend'", "Fix and copy")

# 5. Start container
print("5. Starting container...")
run(f"docker start {CONTAINER}", "Start")
time.sleep(3)

# 6. Check if source code is there
print("6. Checking source code...")
run(f"docker exec {CONTAINER} ls -la /home/moduforge/backend/", "Source code")

# 7. Copy fixed zipper.go
print("7. Copying fixed zipper.go...")
run(f"docker cp /tmp/zipper.go {CONTAINER}:/home/moduforge/backend/internal/service/zipper.go", "Copy zipper.go")

# 8. Check the fixed file
print("8. Checking fixed file...")
run(f"docker exec {CONTAINER} head -30 /home/moduforge/backend/internal/service/zipper.go", "Check zipper.go")

# 9. Compile
print("9. Compiling...")
run(f"docker exec {CONTAINER} sh -c 'cd /home/moduforge/backend && go build -o /tmp/server ./cmd/moduforge'", "Compile")

# 10. Check binary
print("10. Checking binary...")
run(f"docker exec {CONTAINER} ls -la /tmp/server", "Binary")

# 11. Deploy binary
print("11. Deploying binary...")
run(f"docker exec {CONTAINER} cp /tmp/server /app/server", "Deploy")
run(f"docker exec {CONTAINER} chmod +x /app/server", "Permissions")

# 12. Restart
print("12. Restarting...")
run(f"docker restart {CONTAINER}", "Restart")
time.sleep(5)

# 13. Check status
print("13. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'", "Status")
run("curl -s http://192.168.2.9:8086/health", "Health")

ssh.close()
print("\nDone!")
