#!/usr/bin/env python3
"""Check container logs and try to fix properly."""

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

# Check container logs
print("Container logs:")
run(f"docker logs {CONTAINER} --tail 10 2>&1")

# Stop container
print("\nStopping container...")
run(f"docker stop {CONTAINER}")

# Check the UpperDir
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"
print(f"\nChecking UpperDir: {upper_dir}")
run(f"ls -la {upper_dir}/ 2>/dev/null | head -10")

# Check if the binary is in the right place
print("\nChecking binary location...")
run(f"ls -la {upper_dir}/server 2>/dev/null || echo 'Binary not in UpperDir'")

# The issue might be that the binary is in a lower layer
# Let's check the LowerDir
print("\nChecking LowerDir layers...")
lower_dirs = [
    "/vol1/docker/overlay2/2qsipjc6o3f8jkn7jy4p7lxgx/diff",
    "/vol1/docker/overlay2/thpiqp21e72mof29rlq5vdea8/diff",
    "/vol1/docker/overlay2/9bffph3l0sh31fozjrualzll8/diff",
    "/vol1/docker/overlay2/ba4brvwfh3zs7br9bp1279pib/diff",
]

for ld in lower_dirs:
    run(f"ls -la {ld}/server 2>/dev/null && echo 'Found in {ld}' || echo 'Not in {ld}'")

# Try to find where the /server binary is in the overlay layers
print("\nSearching for /server binary in all overlay layers...")
run("find /vol1/docker/overlay2 -name 'server' -type f 2>/dev/null | head -5")

# Check if the container image has the binary
print("\nChecking container image layers...")
run(f"docker history moduforge:latest --no-trunc | head -10")

ssh.close()
print("\nDone!")
