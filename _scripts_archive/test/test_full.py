# -*- coding: utf-8 -*-
"""Check DB and create user if needed."""
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check DB tables
print("=== DB Tables ===")
print(run("docker exec moduforge ls -la /data/"))

# Check if users exist via API
print("\n=== Try register ===")
register = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/register '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123","email":"admin@moduforge.local"}\''
)
print("Register:", register[:300])

# Try login again
print("\n=== Login ===")
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login[:300])

# If login works, test rhythm
try:
    data = json.loads(login)
    token = data.get("token", "")
    if token:
        print("\n=== Custom Providers ===")
        providers = run(
            'curl -s http://localhost:8086/api/v1/llm/custom-providers '
            '-H "Authorization: Bearer %s"' % token
        )
        print("Providers:", providers[:500])
        
        # Check if rhythm provider exists, if not create it
        if '"providers":[]' in providers or not providers.strip():
            print("\n=== Creating rhythm provider ===")
            create = run(
                'curl -s -X POST http://localhost:8086/api/v1/llm/custom-providers '
                '-H "Content-Type: application/json" '
                '-H "Authorization: Bearer %s" '
                '-d \'{"name":"rhythm","endpoint":"https://tokenrhythm.studio/v1","api_key":"sk_tr_umv3XPUo3qlkF-ojwKMgjFtWRczPPiI_Q18fAKWTXRo"}\'' % token
            )
            print("Create:", create[:300])
            
            # Verify
            providers = run(
                'curl -s http://localhost:8086/api/v1/llm/custom-providers '
                '-H "Authorization: Bearer %s"' % token
            )
            print("After create:", providers[:500])
        
        # Test Agent
        print("\n=== Agent Test ===")
        agent = run(
            'curl -s -X POST http://localhost:8086/api/v1/agent/run '
            '-H "Content-Type: application/json" '
            '-H "Authorization: Bearer %s" '
            '-d \'{"task":"say hello in one word","provider":"rhythm","model":"dsv4f","session_id":"test-rhythm-002"}\' '
            '--max-time 30' % token
        )
        print("Agent:", agent[:500])
        
        # Check logs
        print("\n=== Logs ===")
        print(run("docker logs moduforge --tail 10 2>&1"))
except Exception as e:
    print("Error:", e)

ssh.close()
print("\nDone")
