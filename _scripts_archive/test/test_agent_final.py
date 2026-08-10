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
data = json.loads(login)
token = data.get("token", "")
print("Token:", token[:50] + "...")

# Check custom providers
print("\n=== Custom Providers ===")
providers = run('curl -s http://localhost:8086/api/v1/llm/custom-providers -H "Authorization: Bearer %s"' % token)
print("Providers:", providers[:500])

# Create rhythm provider if not exists
if '"providers":[]' in providers or not providers.strip():
    print("\n=== Creating rhythm provider ===")
    create = run(
        'curl -s -X POST http://localhost:8086/api/v1/llm/custom-providers '
        '-H "Content-Type: application/json" '
        '-H "Authorization: Bearer %s" '
        '-d \'{"name":"rhythm","endpoint":"https://tokenrhythm.studio/v1","api_key":"sk_tr_umv3XPUo3qlkF-ojwKMgjFtWRczPPiI_Q18fAKWTXRo"}\'' % token
    )
    print("Create:", create[:300])

# Test Agent
print("\n=== Agent Test ===")
agent = run(
    'curl -s -X POST http://localhost:8086/api/v1/agent/run '
    '-H "Content-Type: application/json" '
    '-H "Authorization: Bearer %s" '
    '-d \'{"task":"say hello in one word","provider":"rhythm","model":"dsv4f","session_id":"test-fix-003"}\' '
    '--max-time 30' % token
)
print("Agent:", agent[:500])

# Check logs for rhythm resolution
print("\n=== Logs ===")
print(run("docker logs moduforge --tail 10 2>&1 | grep -i rhythm"))

ssh.close()
print("\nDone")
