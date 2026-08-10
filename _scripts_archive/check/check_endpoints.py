#!/usr/bin/env python3
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Login
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")
print(f"Login: {'OK' if token else 'FAILED'}")

if not token:
    import sys; sys.exit(1)

# Check ALL possible endpoints for custom providers
endpoints = [
    "/api/v1/providers",
    "/api/v1/providers/custom",
    "/api/v1/settings/providers",
    "/api/v1/settings/custom-providers",
    "/api/v1/settings/llm",
    "/api/v1/custom-providers",
    "/api/v1/settings",
]
for ep in endpoints:
    cmd = f"""curl -s {ep} -H "Authorization: Bearer {token}" 2>&1"""
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode().strip()
    if out and "Not Found" not in out:
        print(f"\n{ep}: {out[:500]}")

# Check the web UI settings page HTML to find API endpoints
stdin, stdout, stderr = ssh.exec_command(
    """curl -s http://localhost:8087/settings 2>&1""",
    timeout=10
)
html = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\n/settings page length: {len(html)}")

# Look at what the frontend code uses for custom provider endpoints
stdin, stdout, stderr = ssh.exec_command(
    """docker exec moduforge sh -c "grep -r 'custom.provider\\|customProvider\\|custom_provider' /app/dist/ 2>/dev/null | head -5" """,
    timeout=10
)
out = stdout.read().decode("utf-8", errors="ignore").strip()
if out:
    print(f"\nFrontend custom provider refs:\n{out[:500]}")

ssh.close()
