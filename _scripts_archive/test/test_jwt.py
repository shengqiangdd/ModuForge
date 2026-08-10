# -*- coding: utf-8 -*-
import paramiko, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check JWT_SECRET
print("=== JWT_SECRET ===")
print(run("docker exec moduforge env | grep JWT"))

# Wait for rate limit
print("\nWaiting 65s for rate limit...")
time.sleep(65)

# Try login
print("\n=== Login ===")
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login[:300])

# If token, test
try:
    data = json.loads(login)
    token = data.get("token", "")
    if token:
        print("\n=== Custom Providers ===")
        print(run('curl -s http://localhost:8086/api/v1/llm/custom-providers -H "Authorization: Bearer %s"' % token)[:500])
        
        print("\n=== Agent Test ===")
        print(run(
            'curl -s -X POST http://localhost:8086/api/v1/agent/run '
            '-H "Content-Type: application/json" '
            '-H "Authorization: Bearer %s" '
            '-d \'{"task":"say hello in one word","provider":"rhythm","model":"dsv4f","session_id":"test-001"}\' '
            '--max-time 20' % token
        )[:500])
except:
    pass

# Check container logs
print("\n=== Logs ===")
print(run("docker logs moduforge --tail 10 2>&1"))

ssh.close()
