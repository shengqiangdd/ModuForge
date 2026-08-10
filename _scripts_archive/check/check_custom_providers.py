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

# Check custom providers via the correct API path
for ep in ["/api/v1/llm/custom-providers", "/api/v1/llm/provider-configs", "/api/v1/llm/config"]:
    cmd = f"""curl -s {ep} -H "Authorization: Bearer {token}" 2>&1"""
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode("utf-8", errors="ignore").strip()
    print(f"\n{ep}: {out[:500]}")

# Check llm_config table (where the default config is stored)
stdin, stdout, stderr = ssh.exec_command(
    f"""curl -s http://localhost:8087/api/v1/llm/config -H "Authorization: Bearer {token}" 2>&1""",
    timeout=10
)
out = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\nllm/config: {out[:500]}")

ssh.close()
