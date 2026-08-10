#!/usr/bin/env python3
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Check provider_configs via sqlite dump
cmds = [
    # Install sqlite3 or use strings
    'docker exec moduforge sh -c "apt list --installed 2>/dev/null | grep sqlite"',
    # Use Go binary itself to query
    """docker exec moduforge sh -c "cat /data/.env" """,
    # Check via the settings provider API that Web UI uses
    """curl -s http://localhost:8087/api/v1/settings/providers -H "Authorization: Bearer test" 2>&1""",
    # Check the actual provider_configs table via Go's own API
]

for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode("utf-8", errors="ignore").strip()
    err = stderr.read().decode("utf-8", errors="ignore").strip()
    print(f"$ {cmd[:80]}")
    if out: print(f"  {out[:300]}")
    if err and "error" not in out.lower(): print(f"  ERR: {err[:200]}")

# Login and check the settings/llm endpoint which is the actual API used
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")

if token:
    # The web UI settings page uses these endpoints
    for ep in ["/api/v1/settings/providers", "/api/v1/settings/llm", "/api/v1/settings/custom-providers"]:
        cmd = f"""curl -s {ep} -H "Authorization: Bearer {token}" 2>&1"""
        stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
        out = stdout.read().decode("utf-8", errors="ignore").strip()
        if out:
            print(f"\n{ep}: {out[:500]}")

# Try the settings GET endpoint (what the frontend loads)
stdin, stdout, stderr = ssh.exec_command(
    f"""curl -s http://localhost:8087/api/v1/settings -H "Authorization: Bearer {token}" 2>&1""",
    timeout=10
)
out = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\n/api/v1/settings: {out[:500]}")

ssh.close()
