#!/usr/bin/env python3
"""Check if the server binary has the new zipper code."""

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

# Check the binary's strings for exclusion patterns
print("Checking binary for exclusion patterns...")
run(f"docker exec {CONTAINER} strings /server | grep -E 'DESIGN_DOC|tmp/|rust_files_list' | head -5")

# Check the binary's strings for webroot
print("\nChecking binary for webroot...")
run(f"docker exec {CONTAINER} strings /server | grep -i webroot | head -5")

# Check the binary's modification time
print("\nChecking binary modification time...")
run(f"docker exec {CONTAINER} ls -la /server")

# Check if there are multiple server binaries
print("\nChecking for multiple server binaries...")
run(f"docker exec {CONTAINER} find / -name 'server' -type f 2>/dev/null | head -5")

# Check the overlay2 layer for the original binary
print("\nChecking overlay2 layer...")
run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/ 2>/dev/null || echo 'No bin dir'")

ssh.close()
print("\nDone!")
