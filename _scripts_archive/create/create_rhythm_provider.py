#!/usr/bin/env python3
"""Create rhythm custom provider via API and test agent"""
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

# Create rhythm custom provider
create_cmd = """curl -s -X POST http://localhost:8087/api/v1/llm/custom-providers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{{"name":"rhythm","endpoint":"https://token.rhythm.com/v1","api_key":"dsv4f","models_json":"[]"}}'""".format(token=token)

stdin, stdout, stderr = ssh.exec_command(create_cmd, timeout=10)
out = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\nCreate custom provider: {out[:500]}")

# List custom providers
stdin, stdout, stderr = ssh.exec_command(
    f"""curl -s http://localhost:8087/api/v1/llm/custom-providers -H "Authorization: Bearer {token}" 2>&1""",
    timeout=10
)
out = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\nCustom providers list: {out[:500]}")

# Also check provider-configs
stdin, stdout, stderr = ssh.exec_command(
    f"""curl -s http://localhost:8087/api/v1/llm/provider-configs -H "Authorization: Bearer {token}" 2>&1""",
    timeout=10
)
out = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\nProvider configs: {out[:500]}")

ssh.close()
