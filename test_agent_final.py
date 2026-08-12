"""Test agent with proper project_id"""
import paramiko
import sys
import io
import json

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Get auth token and project list
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])"
""")
token = stdout.read().decode('utf-8', errors='replace').strip()

# Get projects
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer ' + token + '"')
projects_raw = json.loads(stdout.read().decode('utf-8', errors='replace'))
# Handle both formats: {projects:[...]} or [...]
if isinstance(projects_raw, dict):
    projects_list = projects_raw.get('projects', [])
else:
    projects_list = projects_raw

print('Projects:')
for p in projects_list:
    print(f'  {p["id"]}: {p.get("name","?")} (files: {p.get("file_count",0)})')

# Use first project with files
pid = None
for p in projects_list:
    if p.get('file_count', 0) > 0:
        pid = p['id']
        break
if not pid and projects_list:
    pid = projects_list[0]['id']

if not pid:
    print('No projects found!')
    ssh.close()
    sys.exit(1)

print(f'\nUsing project: {pid}')

# 2. Test agent with proper project_id
print('\n=== Test agent: list files ===')
cmd = 'curl -s -X POST "http://localhost:8086/api/v1/agent/run" -H "Content-Type: application/json" -H "Authorization: Bearer ' + token + '" -d \'{"task":"List all files in this project using the list_dir tool with path .","project_id":"' + pid + '","agent_mode":"act","provider_id":"opencode-zen","model":"nemotron-3-ultra-free"}\' -m 120'
stdin, stdout, stderr = ssh.exec_command(cmd)
result = stdout.read().decode('utf-8', errors='replace')

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
            print(f'  [TOOL] {name}({json.dumps(args, ensure_ascii=False)[:200]})')
        elif t == 'tool_result':
            content = d.get('content', '')
            print(f'  [RESULT] {content[:500]}')
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
            print(f'  [FINAL] {content[:800]}')
    except:
        pass

# 3. Test agent: read a file
print('\n\n=== Test agent: read file ===')
cmd = 'curl -s -X POST "http://localhost:8086/api/v1/agent/run" -H "Content-Type: application/json" -H "Authorization: Bearer ' + token + '" -d \'{"task":"Read the first 50 lines of src/go/main.go using the read_file tool","project_id":"' + pid + '","agent_mode":"act","provider_id":"opencode-zen","model":"nemotron-3-ultra-free"}\' -m 120'
stdin, stdout, stderr = ssh.exec_command(cmd)
result = stdout.read().decode('utf-8', errors='replace')

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
            print(f'  [TOOL] {name}({json.dumps(args, ensure_ascii=False)[:200]})')
        elif t == 'tool_result':
            content = d.get('content', '')
            print(f'  [RESULT] {content[:500]}')
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
            print(f'  [FINAL] {content[:800]}')
    except:
        pass

ssh.close()
