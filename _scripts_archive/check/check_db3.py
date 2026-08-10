#!/usr/bin/env python3
import paramiko
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Use python3 to check sqlite tables
cmds = [
    # Check if python3 or sqlite3 available
    'docker exec moduforge sh -c "which python3 || which python || which sqlite3 || echo none"',
    # Use the sqlite3 dump header to list tables
    'docker exec moduforge sh -c "head -c 200 /data/moduforge.db"',
    # Check web settings page for custom providers
    """curl -s http://localhost:8087/api/v1/settings -H "Authorization: Bearer test" 2>&1""",
    # Check what the settings endpoint returns
    """curl -s http://localhost:8087/settings -H "Authorization: Bearer test" 2>&1""",
]

for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    print(f"$ {cmd[:80]}")
    if out: print(f"  OUT: {out[:300]}")
    if err: print(f"  ERR: {err[:300]}")

# The web UI stores custom providers. Let's check the API
# that the frontend actually uses
import json

# Login
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")

if token:
    # Check providers endpoint
    endpoints = [
        "/api/v1/providers",
        "/api/v1/providers/custom",
        "/api/v1/settings/providers",
        "/api/v1/settings/custom-providers",
        "/api/v1/custom-providers",
    ]
    for ep in endpoints:
        cmd = f"""curl -s {ep} -H "Authorization: Bearer {token}" 2>&1"""
        stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
        out = stdout.read().decode().strip()
        if out and "Not Found" not in out:
            print(f"\n{ep}: {out[:400]}")

ssh.close()
