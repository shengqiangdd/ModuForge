#!/usr/bin/env python3
"""Get full provider list and try different models"""
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

# Get full provider list
print("=== Full LLM providers ===")
out, _ = run('curl -s http://localhost:8087/api/v1/llm/providers')
data = json.loads(out)
providers = data.get("providers", [])
for p in providers:
    print(f"\n  Provider: {p['name']} (id: {p['id']})")
    print(f"  Endpoint: {p['endpoint']}")
    for m in p.get("models", []):
        print(f"    - {m['id']} ({m['name']})")

# Login and get token for 8087
out, _ = run('curl -s -X POST http://localhost:8087/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(out).get("token", "")

# Try with opencode-zen provider which seems to have free models
print("\n\n=== Try agent with opencode-zen/deepseek-v4-flash-free ===")
task_json = json.dumps({"task":"say hello","provider_id":"opencode-zen","model":"deepseek-v4-flash-free"})
cmd = f'''curl -s -X POST http://localhost:8087/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task_json}' 2>&1'''
out, _ = run(cmd, timeout=60)
print(out[:3000])

client.close()
