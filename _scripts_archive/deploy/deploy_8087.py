# -*- coding: utf-8 -*-
"""Deploy ModuForge with correct port mapping (8087 for nginx proxy)."""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=60)
    return o.read().decode().strip() or e.read().decode().strip()

# Cleanup
print("=== Cleanup ===")
run("docker rm -f moduforge fix_perm 2>/dev/null")

# Start with port 8087 (nginx proxies 8086 -> 8087)
print("\n=== Start ModuForge ===")
result = run(
    'docker run -d --name moduforge '
    '--restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8087:8080 '
    'moduforge:patched'
)
print("Run:", result)

# Wait
print("\n=== Waiting ===")
time.sleep(8)

# Verify
status = run('docker inspect moduforge --format "{{.State.Status}}"')
print("Status:", status)

logs = run("docker logs moduforge --tail 20 2>&1")
print("Logs:", logs)

# Direct health check on 8087
health = run("docker exec moduforge curl -s http://localhost:8080/health")
print("Health (direct):", health)

# Through nginx (8086)
health_nginx = run("curl -s http://localhost:8086/health")
print("Health (nginx):", health_nginx)

# Binary fix check
fix = run('docker exec moduforge strings /server | grep -c "WHERE name="')
print("Binary fix count:", fix)

# Test login through nginx
print("\n=== Test API ===")
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login[:300] if login else "empty")

import json
try:
    data = json.loads(login)
    token = data.get("token", "")
    if token:
        # Test custom providers
        providers = run(
            'curl -s http://localhost:8086/api/v1/llm/custom-providers '
            '-H "Authorization: Bearer %s"' % token
        )
        print("Custom providers:", providers[:500] if providers else "empty")
        
        # Test Agent with rhythm
        print("\n=== Agent Test ===")
        agent = run(
            'curl -s -X POST http://localhost:8086/api/v1/agent/run '
            '-H "Content-Type: application/json" '
            '-H "Authorization: Bearer %s" '
            '-d \'{"task":"say hello in one word","provider":"rhythm","model":"dsv4f","session_id":"test-fix-001"}\' '
            '--max-time 20' % token
        )
        print("Agent:", agent[:500] if agent else "empty")
except Exception as e:
    print("Error:", e)

# Check container logs for rhythm resolution
print("\n=== Agent Logs ===")
print(run("docker logs moduforge --tail 10 2>&1 | grep -i rhythm"))

ssh.close()
print("\n=== Done ===")
