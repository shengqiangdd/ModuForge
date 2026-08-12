"""Check agent capabilities and try to delegate"""
import paramiko, sys, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Get full skills list
print('=== Full skills ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s http://localhost:8086/api/v1/agent/skills -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for s in data.get('skills', []):
    print(f'  {s[\"name\"]}: {s[\"description\"][:80]}')
"
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check if there's a chat endpoint
print('\n=== Check chat API ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
# Try to send a message to the agent
curl -s -X POST http://localhost:8086/api/v1/ai/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"List all files in the project 1785249992652501794-1864"}],"project_id":"1785249992652501794-1864"}' \
  -m 10 2>&1 | head -30
""")
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
