#!/usr/bin/env python3
"""Fix binary using docker run with volume mount."""

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

# 1. Stop the broken container
print("1. Stopping broken container...")
run(f"docker stop {CONTAINER}")

# 2. Get the container's volume mounts
print("2. Getting volume mounts...")
run(f"docker inspect {CONTAINER} --format='{{{{json .Mounts}}}}' 2>/dev/null | python3 -m json.tool")

# 3. Get the data volume name
print("3. Getting data volume...")
run(f"docker volume ls | grep moduforge")

# 4. Use docker run to fix the binary in the volume
print("4. Fixing binary in volume...")
# First, find the original binary
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/")

# 5. Create a temporary container to fix the binary
print("5. Creating temporary container to fix binary...")
run(f"docker run --rm -v moduforge_data:/data alpine sh -c 'ls -la /data/'")

# 6. Copy the original binary to a temp location
print("6. Copying original binary...")
run("cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/moduforge-linux /tmp/moduforge-original")
run("chmod +x /tmp/moduforge-original")
run("ls -la /tmp/moduforge-original")

# 7. Try to find where the /server binary is stored
print("7. Looking for /server binary location...")
run(f"docker inspect {CONTAINER} --format='{{{{json .GraphDriver.Data}}}}' 2>/dev/null | python3 -m json.tool")

# 8. Use docker commit to save the current state and then fix
print("8. Trying docker commit...")
run(f"docker commit {CONTAINER} moduforge-fixed 2>/dev/null || echo 'Commit failed'")

# 9. Check the new image
print("9. Checking new image...")
run("docker images | grep moduforge")

ssh.close()
print("\nDone!")
