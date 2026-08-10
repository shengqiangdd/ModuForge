#!/usr/bin/env python3
"""Check provider config and fix region issue"""
import sys, io, json
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=30):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

# Login
out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(out).get("token", "")

# Check providers
print("=== Provider list ===")
out, _ = run(f'curl -s http://localhost:8086/api/v1/providers -H "Authorization: Bearer {token}"')
print(out[:3000])

# Check agent settings
print("\n=== Agent settings ===")
out, _ = run(f'curl -s http://localhost:8086/api/v1/agent/settings -H "Authorization: Bearer {token}"')
print(out[:2000])

# Check .env for available keys
print("\n=== .env keys ===")
out, _ = run('grep -i "key\\|api\\|token\\|provider" /vol1/docker/volumes/moduforge_moduforge_data/.env 2>/dev/null | head -20')
print(out[:1000])

# Check the resolveProvider logic in the container
print("\n=== Check agent runner resolveLLMConfig ===")
out, _ = run('grep -n "resolveLLMConfig\\|resolveProvider\\|providerID\\|opencode-go\\|rhythm" /app/src/agent/runner.go 2>/dev/null | head -20')
print(out[:1000])

# Try with a different model/provider
print("\n=== Try with different provider ===")
task_json = json.dumps({"task":"say hello","provider_id":"rhythm","model":"deepseek-v4-flash"})
cmd = f'''curl -s -X POST http://localhost:8086/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task_json}' 2>&1'''
out, _ = run(cmd, timeout=60)
print(out[:2000])

client.close()
