#!/usr/bin/env python3
"""Check the actual deployed zipper code."""

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

# Check the deployed zipper.go
print("Checking deployed zipper.go...")
run(f"docker exec {CONTAINER} head -100 /home/moduforge/backend/internal/service/zipper.go 2>/dev/null || echo 'File not found'")

# Check if the server binary was updated
print("\nChecking server binary...")
run(f"docker exec {CONTAINER} ls -la /app/server 2>/dev/null || echo 'No /app/server'")

# Check the binary's embedded strings
print("\nChecking binary strings for webroot...")
run(f"docker exec {CONTAINER} strings /server 2>/dev/null | grep -i webroot | head -5")

# Check if the new zipper code is in the binary
print("\nChecking for isExcluded function...")
run(f"docker exec {CONTAINER} strings /server 2>/dev/null | grep -i 'isExcluded\\|DESIGN_DOC\\|tmp/' | head -10")

ssh.close()
print("\nDone!")
