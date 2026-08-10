#!/usr/bin/env python3
"""Find the actual API routes registered."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Check RegisterRoutes function
print("=== RegisterRoutes function ===")
stdin, stdout, stderr = ssh.exec_command("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/handler/routes.go 2>/dev/null")
print(stdout.read().decode())

ssh.close()
