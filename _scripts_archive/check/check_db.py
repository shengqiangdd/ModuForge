#!/usr/bin/env python3
"""Check custom_providers table and fix it"""
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Check what tables exist and their schema
cmds = [
    # List all tables
    'docker exec moduforge sh -c "cat /data/moduforge.db | strings | grep -i custom"',
    # Try to query custom_providers via the API
    """curl -s http://localhost:8087/api/v1/health/cache 2>&1""",
    # Check provider_configs table
    """curl -s http://localhost:8087/api/v1/settings/providers -H "Authorization: Bearer test" 2>&1""",
    # Check llm_providers table
    """curl -s http://localhost:8087/api/v1/settings/llm -H "Authorization: Bearer test" 2>&1""",
]

for cmd in cmds:
    print(f"\n$ {cmd[:80]}...")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    if out:
        print(f"  stdout: {out[:300]}")
    if err:
        print(f"  stderr: {err[:300]}")

# Login and check settings
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")
print(f"\nLogin: {'OK' if token else 'FAILED'}")

if token:
    # Check custom providers via settings endpoint
    for endpoint in ["/api/v1/settings/providers", "/api/v1/settings/llm", "/api/v1/settings"]:
        cmd = f"""curl -s {endpoint} -H "Authorization: Bearer {token}" 2>&1"""
        stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
        out = stdout.read().decode().strip()
        if out:
            print(f"\n{endpoint}: {out[:400]}")

ssh.close()
