"""Configure ModuForge agent to use working free model and test"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Get auth token
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])"
""")
token = stdout.read().decode('utf-8', errors='replace').strip()

# 2. Set LLM config to use nemotron-3-ultra-free
print('=== Setting LLM config ===')
stdin, stdout, stderr = ssh.exec_command('curl -s -X POST "http://localhost:8086/api/v1/llm/config" -H "Content-Type: application/json" -H "Authorization: Bearer ' + token + '" -d \'{"provider":"opencode-zen","model_id":"nemotron-3-ultra-free"}\'')
print(stdout.read().decode('utf-8', errors='replace'))

# 3. Get first project
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer ' + token + '" | python3 -c "import sys,json; ps=json.load(sys.stdin).get(\\\"projects\\\",[]); print(ps[0][\\\"id\\\"] if ps else \\\\")"')
pid = stdout.read().decode('utf-8', errors='replace').strip()
print(f'Project: {pid}')

# 4. Test agent with nemotron-3-ultra-free
print('\n=== Test agent ===')
cmd = 'curl -s -X POST "http://localhost:8086/api/v1/agent/run" -H "Content-Type: application/json" -H "Authorization: Bearer ' + token + '" -d \'{"task":"List all files using the list_dir tool with path .","project_id":"' + pid + '","agent_mode":"act","provider_id":"opencode-zen","model":"nemotron-3-ultra-free"}\' -m 120'
stdin, stdout, stderr = ssh.exec_command(cmd)
result = stdout.read().decode('utf-8', errors='replace')

# Parse SSE events
import json
for line in result.split('\n'):
    line = line.strip()
    if not line.startswith('data: '):
        continue
    try:
        d = json.loads(line[6:])
        t = d.get('type', '')
        if t == 'step':
            step = d.get('step', '?')
            content = d.get('content', '')
            print(f'  [{step}] {content}')
        elif t == 'tool':
            name = d.get('name', '?')
            args = d.get('args', {})
            print(f'  [TOOL] {name}({args})')
        elif t == 'tool_result':
            content = d.get('content', '')
            print(f'  [RESULT] {content[:300]}')
        elif t == 'reasoning':
            print(d.get('content', ''), end='')
        elif t == 'delta':
            print(d.get('content', ''), end='')
        elif t == 'done':
            print()
        elif t == 'error':
            print(f'  [ERROR] {d}')
        elif t == 'final':
            content = d.get('content', '')
            print(f'  [FINAL] {content[:500]}')
    except:
        pass

# 5. Check logs
print('\n=== Logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 20 2>&1 | grep -i "agent\\|llm\\|error\\|retry"')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
