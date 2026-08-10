#!/usr/bin/env python3
"""Find the correct working binary."""

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

# Check all server binaries
print("Checking all server binaries...")
run("find /vol1/docker/overlay2 -name 'server' -type f -exec ls -la {} \\; 2>/dev/null | head -10")

# Check the binary in the UpperDir
upper_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff"
print(f"\nChecking UpperDir binary: {upper_dir}/server")
run(f"file {upper_dir}/server 2>/dev/null || echo 'file command not available'")

# Check if the binary has CGO enabled by looking at strings
print("\nChecking binary strings for CGO...")
run(f"strings {upper_dir}/server 2>/dev/null | grep -i 'cgo\\|sqlite3' | head -5 || echo 'strings not available'")

# Check the binary size
print("\nChecking binary sizes...")
run(f"ls -la {upper_dir}/server")
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/moduforge-linux")

# Try to find a working binary from a previously working container
print("\nLooking for working container...")
run("docker ps -a | grep -i moduforge")

# Check if there's a backup of the working binary
print("\nChecking for backup binaries...")
run("find /vol1/docker -name '*.bak' -o -name '*backup*' -o -name '*old*' 2>/dev/null | grep -i server | head -5")

# The original container was working - let's check its image layers
print("\nChecking original image layers...")
run("docker history moduforge:latest --no-trunc 2>/dev/null | head -20")

ssh.close()
print("\nDone!")
