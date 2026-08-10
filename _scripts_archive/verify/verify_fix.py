# -*- coding: utf-8 -*-
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    return o.read().decode().strip() or e.read().decode().strip()

# Login
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
token = json.loads(login).get("token", "")

# Check binary fix
print("=== Binary Fix Verification ===")
print("WHERE name= count:", run('docker exec moduforge strings /server | grep -c "WHERE name="'))

# Check custom provider exists
print("\nCustom providers:", run('curl -s http://localhost:8086/api/v1/llm/custom-providers -H "Authorization: Bearer %s"' % token)[:300])

# Test with a preset provider (should work)
print("\n=== Test with preset provider ===")
agent = run(
    'curl -s -X POST http://localhost:8086/api/v1/agent/run '
    '-H "Content-Type: application/json" '
    '-H "Authorization: Bearer %s" '
    '-d \'{"task":"say hello in one word","provider":"openai","model":"gpt-4o-mini","session_id":"test-preset-001"}\' '
    '--max-time 20' % token
)
print("Preset agent:", agent[:500])

# Check container health
print("\n=== Container Health ===")
print("Status:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("Health:", run('curl -s http://localhost:8086/health'))

ssh.close()
print("\nDone")
