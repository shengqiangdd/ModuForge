# -*- coding: utf-8 -*-
"""Quick rebuild and deploy."""
import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=300):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode().strip()
    err = e.read().decode().strip()
    return out or err

# Check if build context exists
print("Build context:", run("ls -la /tmp/moduforge-build/"))

# Build with longer timeout
print("\n=== Building ===")
result = run("cd /tmp/moduforge-build && docker build -t moduforge:patched . 2>&1", timeout=300)
print(result)

# Check if image exists
print("\n=== Image check ===")
print(run("docker images moduforge:patched"))

# If build failed, check what went wrong
if "patched" not in run("docker images --format '{{.Repository}}:{{.Tag}}'"):
    print("\nBuild failed! Checking Dockerfile content:")
    print(run("cat /tmp/moduforge-build/Dockerfile"))
    print("\nFiles in build context:")
    print(run("ls -la /tmp/moduforge-build/"))
    print("\nFile permissions:")
    print(run("stat /tmp/moduforge-build/server"))
    print(run("stat /tmp/moduforge-build/entrypoint.sh"))

ssh.close()
