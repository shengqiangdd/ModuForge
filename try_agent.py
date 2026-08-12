"""Try agent first, then fallback to opencode"""
import paramiko, sys, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Try a simple agent task: list project files
print('=== Try agent: list files ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X POST http://localhost:8086/api/v1/ai/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"List the top-level directory structure of project 1785249992652501794-1864. Just run ls -la and show the output."}],"project_id":"1785249992652501794-1864"}' \
  -m 30 2>&1 | grep "delta" | python3 -c "
import sys
for line in sys.stdin:
    if 'delta' in line:
        try:
            data = line.split('data: ')[1]
            obj = json.loads(data)
            print(obj.get('content', ''), end='', flush=True)
        except: pass
" 2>&1
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check if agent can execute commands
print('\n=== Agent tool execution ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X POST http://localhost:8086/api/v1/ai/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"Check if the AndroBoost-SmartTune WebUI binary exists and what port it listens on. Run: ls -la /data/storage/projects/1785249992652501794-1864/system/bin/"}],"project_id":"1785249992652501794-1864"}' \
  -m 30 2>&1 | grep "delta" | python3 -c "
import sys
for line in sys.stdin:
    if 'delta' in line:
        try:
            data = line.split('data: ')[1]
            obj = json.loads(data)
            print(obj.get('content', ''), end='', flush=True)
        except: pass
" 2>&1
""")
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
