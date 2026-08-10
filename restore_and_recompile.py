#!/usr/bin/env python3
"""Restore old binary and recompile with CGO_ENABLED=1."""

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
run(f"docker stop {CONTAINER}")

# 2. Restore the old binary
print("2. Restoring old binary...")
run(f"docker cp {CONTAINER}:/tmp/server /tmp/server_new_compiled 2>/dev/null || echo 'No new binary in /tmp'")
# The original binary should still be in the overlay
run(f"docker run --rm -v /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin:/bin_src alpine sh -c 'cp /bin_src/moduforge-linux /server_restored && chmod +x /server_restored'", "Extract original binary")
run(f"docker cp /server_restored {CONTAINER}:/server", "Restore binary")

# 3. Start container
print("3. Starting container...")
run(f"docker start {CONTAINER}")
time.sleep(5)

# 4. Check status
print("4. Checking status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

# 5. Now recompile with CGO_ENABLED=1
print("\n5. Recompiling with CGO_ENABLED=1...")
run(f"docker exec {CONTAINER} sh -c 'cd /home/moduforge/backend && CGO_ENABLED=1 go build -o /tmp/server_cgo ./cmd/moduforge 2>&1'")

# 6. Check if compilation succeeded
print("6. Checking compiled binary...")
run(f"docker exec {CONTAINER} ls -la /tmp/server_cgo 2>/dev/null || echo 'Compilation failed'")

# 7. If compilation succeeded, deploy
run(f"docker exec {CONTAINER} sh -c 'if [ -f /tmp/server_cgo ]; then cp /tmp/server_cgo /server && chmod +x /server && echo Deployed; fi'")

# 8. Restart
print("8. Restarting...")
run(f"docker restart {CONTAINER}")
time.sleep(5)

# 9. Final check
print("9. Final status...")
run(f"docker ps -f name={CONTAINER} --format '{{{{.Status}}}}'")
run("curl -s http://192.168.2.9:8086/health")

ssh.close()
print("\nDone!")
