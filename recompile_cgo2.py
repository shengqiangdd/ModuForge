#!/usr/bin/env python3
"""Recompile with CGO_ENABLED=1 inside the container and deploy."""

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
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=300)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out[-500:] if len(out) > 500 else out)
    if err: print(f"STDERR: {err[-500:] if len(err) > 500 else err}")
    print()
    return out

# 1. Stop container
print("1. Stopping container...")
run(f"docker stop {CONTAINER}")

# 2. Install gcc in the container using docker run (since container is stopped)
print("2. Installing gcc via temporary container...")
run(f"""docker run --rm -v /vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff:/fs alpine sh -c "apk add --no-cache gcc musl-dev 2>/dev/null; ls /fs/usr/bin/gcc 2>/dev/null || echo 'gcc not installed'" """, "Install gcc")

# 3. Start container and compile inside
print("3. Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(5)

# 4. Check if gcc is available now
print("4. Checking gcc...")
run(f"docker exec {CONTAINER} which gcc 2>/dev/null || echo 'gcc not found'")

# 5. If no gcc, install it via apk
print("5. Installing gcc in container...")
run(f"docker exec {CONTAINER} sh -c 'apk add --no-cache gcc musl-dev 2>&1 | tail -5'", "Install gcc")

# 6. Compile with CGO_ENABLED=1
print("6. Compiling with CGO_ENABLED=1...")
run(f"docker exec {CONTAINER} sh -c 'cd /home/moduforge/backend && CGO_ENABLED=1 go build -o /tmp/server_new ./cmd/moduforge 2>&1 | tail -20'", "Compile", timeout=300)

# 7. Check binary
print("7. Checking binary...")
run(f"docker exec {CONTAINER} ls -la /tmp/server_new 2>/dev/null || echo 'Binary not found'")

# 8. Deploy
print("8. Deploying...")
run(f"docker exec {CONTAINER} sh -c 'if [ -f /tmp/server_new ]; then cp /tmp/server_new /server && chmod +x /server && echo Deployed; fi'")

# 9. Restart
print("9. Restarting...")
run(f"docker restart {CONTAINER}")
time.sleep(8)

# 10. Check status
print("10. Status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
