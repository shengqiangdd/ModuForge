#!/usr/bin/env python3
"""Debug: check custom_providers data and user_id"""
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Check the rhythm entry details
stdin, stdout, stderr = ssh.exec_command(
    """curl -s http://localhost:8087/api/v1/llm/custom-providers -H "Authorization: Bearer test" 2>&1""",
    timeout=10
)
out = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"Custom providers (no auth): {out[:500]}")

# Login and check
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")

if token:
    # Check what user_id the JWT decodes to
    import base64
    parts = token.split(".")
    if len(parts) >= 2:
        payload = parts[1] + "=" * (4 - len(parts[1]) % 4)
        decoded = json.loads(base64.urlsafe_b64decode(payload))
        print(f"\nJWT payload: {json.dumps(decoded, indent=2)}")

    # Check custom providers with proper auth
    stdin, stdout, stderr = ssh.exec_command(
        f"""curl -s http://localhost:8087/api/v1/llm/custom-providers -H "Authorization: Bearer {token}" 2>&1""",
        timeout=10
    )
    out = stdout.read().decode("utf-8", errors="ignore").strip()
    print(f"\nCustom providers (auth): {out[:800]}")

    # Also check provider-configs
    stdin, stdout, stderr = ssh.exec_command(
        f"""curl -s http://localhost:8087/api/v1/llm/provider-configs -H "Authorization: Bearer {token}" 2>&1""",
        timeout=10
    )
    out = stdout.read().decode("utf-8", errors="ignore").strip()
    print(f"\nProvider configs: {out[:500]}")

ssh.close()
