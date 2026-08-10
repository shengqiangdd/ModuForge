# -*- coding: utf-8 -*-
"""Test Agent with rhythm provider."""
import paramiko, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    return o.read().decode().strip() or e.read().decode().strip()

# Login
print("=== Login ===")
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login[:200])
data = json.loads(login)
token = data.get("token", "")

# Check custom providers
print("\n=== Custom Providers ===")
providers = run(
    'curl -s http://localhost:8086/api/v1/llm/custom-providers '
    '-H "Authorization: Bearer %s"' % token
)
print("Providers:", providers[:500])

# Check LLM config
print("\n=== LLM Config ===")
config = run(
    'curl -s http://localhost:8086/api/v1/llm/config '
    '-H "Authorization: Bearer %s"' % token
)
print("Config:", config[:300])

# Test Agent with rhythm
print("\n=== Agent Test ===")
agent = run(
    'curl -s -X POST http://localhost:8086/api/v1/agent/run '
    '-H "Content-Type: application/json" '
    '-H "Authorization: Bearer %s" '
    '-d \'{"task":"say hello in one word","provider":"rhythm","model":"dsv4f","session_id":"test-rhythm-001"}\' '
    '--max-time 30' % token
)
print("Agent response:", agent[:500])

# Check container logs for rhythm resolution
print("\n=== Container Logs ===")
logs = run("docker logs moduforge --tail 15 2>&1")
print(logs)

ssh.close()
print("\nDone")
