#!/usr/bin/env python3
"""Test agent with rhythm/dsv4f custom provider"""
import paramiko, json, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)
print("Connected")

# Login
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")
print(f"Login: {'OK' if token else 'FAILED'}")

if not token:
    sys.exit(1)

# Test agent call with rhythm/dsv4f
agent_cmd = """curl -s -X POST http://localhost:8086/api/v1/agent/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{{"task":"say hello in one word","provider_id":"rhythm","model":"dsv4f"}}' \
  --max-time 30""".format(token=token)

stdin, stdout, stderr = ssh.exec_command(agent_cmd, timeout=35)
resp = stdout.read().decode().strip()
print(f"\nAgent response ({len(resp)} bytes):")
print(resp[:800])

# Check relevant logs
print("\n--- Container logs (last 30 lines) ---")
stdin, stdout, stderr = ssh.exec_command("docker logs moduforge --tail 30 2>&1", timeout=10)
logs = stdout.read().decode()
for line in logs.split("\n"):
    if any(kw in line.lower() for kw in ["agent", "resolve", "custom", "provider", "rhythm", "error"]):
        print(f"  {line}")

# Also check custom_providers table
print("\n--- custom_providers table ---")
stdin, stdout, stderr = ssh.exec_command(
    'docker exec moduforge sqlite3 /data/moduforge.db "SELECT id, user_id, name, endpoint, model_id, length(api_key) FROM custom_providers"',
    timeout=10
)
print(stdout.read().decode().strip())

ssh.close()
