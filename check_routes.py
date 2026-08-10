#!/usr/bin/env python3
"""Check API routes and test properly."""

import paramiko
import json

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

# Get JWT token
print("Getting JWT token...")
stdin, stdout, stderr = ssh.exec_command(
    """curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'"""
)
token_response = stdout.read().decode()
token = json.loads(token_response).get('token', '')
print(f"Token: {token[:20]}...")

# Check available routes
print("\n=== Checking API routes ===")
stdin, stdout, stderr = ssh.exec_command(f"""curl -s http://localhost:8086/api/v1/routes -H "Authorization: Bearer {token}" """)
print(stdout.read().decode())

# Check the actual handler code
print("\n=== Checking handler routes in code ===")
stdin, stdout, stderr = ssh.exec_command("grep -r 'app.Get\\|app.Post\\|app.Put\\|app.Delete' /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/cmd/ 2>/dev/null | head -30")
print(stdout.read().decode())

# Check the main.go routes
print("\n=== Main.go routes ===")
stdin, stdout, stderr = ssh.exec_command("cat /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/cmd/moduforge/main.go 2>/dev/null | grep -A2 'Group\\|Get\\|Post\\|Put\\|Delete' | head -100")
print(stdout.read().decode())

ssh.close()
print("\nDone!")
