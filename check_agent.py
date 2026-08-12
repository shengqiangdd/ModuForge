"""Try to guide ModuForge agent to fix AndroBoost-SmartTune"""
import paramiko, sys, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Login and get token
print('=== Login ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}'
""")
login_resp = json.loads(stdout.read().decode())
token = login_resp.get('token', '')
print(f'Token: {token[:20]}...')

# Check if there's an agent endpoint
print('\n=== Check agent API ===')
stdin, stdout, stderr = ssh.exec_command(f"""
curl -s http://localhost:8086/api/v1/agent/status -H "Authorization: Bearer {token}"
""")
status = stdout.read().decode(errors='replace')
print(f'Agent status: {status[:200]}')

# Check agent config
stdin, stdout, stderr = ssh.exec_command(f"""
curl -s http://localhost:8086/api/v1/agent/settings -H "Authorization: Bearer {token}"
""")
settings = stdout.read().decode(errors='replace')
print(f'Agent settings: {settings[:200]}')

# Check available skills
stdin, stdout, stderr = ssh.exec_command(f"""
curl -s http://localhost:8086/api/v1/agent/skills -H "Authorization: Bearer {token}"
""")
skills = stdout.read().decode(errors='replace')
print(f'Agent skills: {skills[:300]}')

ssh.close()
