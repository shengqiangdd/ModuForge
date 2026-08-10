#!/usr/bin/env python3
"""Get full service/zipper.go content."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

sftp = ssh.open_sftp()
with sftp.open("/vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go", "r") as f:
    content = f.read().decode()
    print("=== service/zipper.go (full) ===")
    print(content)

sftp.close()
ssh.close()
