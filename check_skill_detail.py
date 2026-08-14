import paramiko
import json

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216')

stdin, stdout, stderr = c.exec_command("curl -s http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}'")
token_data = json.loads(stdout.read())
token = token_data.get('token', '')

stdin, stdout, stderr = c.exec_command(f"curl -s http://localhost:8086/api/v1/agent/skills -H 'Authorization: Bearer {token}'")
skills_data = json.loads(stdout.read())
skills = skills_data.get('skills', [])

for s in skills:
    if s['name'] == 'device_test':
        print(f"Name: {s['name']}")
        print(f"Description: {s['description']}")
        print(f"Metadata: {json.dumps(s.get('metadata', {}), indent=2)}")
        break

c.close()
