#!/usr/bin/env python3
import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace').strip()

print("=== Test API endpoints ===")
# Login first
login_resp = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
print(f"Login: {login_resp[:100]}...")

import json
try:
    token = json.loads(login_resp).get('token', '')
    
    # Test projects endpoint
    projects_resp = run(f'docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"')
    print(f"\nProjects: {projects_resp[:200]}...")
    
    # Test health endpoint
    health_resp = run("docker exec moduforge curl -s http://localhost:8080/health")
    print(f"\nHealth: {health_resp}")
    
    # Check for any error logs
    print("\n=== Recent error logs ===")
    logs = run("docker logs --tail 50 moduforge 2>&1")
    for line in logs.split('\n'):
        if 'error' in line.lower() or 'fail' in line.lower():
            print(line)
except Exception as e:
    print(f"Error: {e}")

ssh.close()
