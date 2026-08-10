#!/usr/bin/env python3
"""Get full ziputil.go content."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Get full ziputil.go
sftp = ssh.open_sftp()
with sftp.open("/vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/builder/ziputil.go", "r") as f:
    content = f.read().decode()
    print("=== ziputil.go (full) ===")
    print(content)

# Get full zipper.go from handler
with sftp.open("/vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/handler/zipper.go", "r") as f:
    content = f.read().decode()
    print("\n=== handler/zipper.go (full) ===")
    print(content)

sftp.close()
ssh.close()
