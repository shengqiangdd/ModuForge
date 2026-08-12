"""ModuForge Agent诊断报告"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Test agent/run with proper project context
print('=== Test agent/run with project context ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Get first project with files
PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json
ps=json.load(sys.stdin).get('projects',[])
for p in ps:
    if p.get('file_count',0) > 0:
        print(p['id'])
        break
else:
    if ps: print(ps[0]['id'])
    else: print('')
")
echo "Project: $PID"

# Test agent with project context
curl -s -X POST "http://localhost:8086/api/v1/agent/run" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"task":"List all files in this project using the list_dir tool","project_id":"'$PID'","agent_mode":"act","provider_id":"opencode-zen","model":"deepseek-v4-flash-free"}' \
  -m 60 2>&1 | python3 -c "
import sys
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    if line.startswith('data: '):
        import json
        try:
            d=json.loads(line[6:])
            t=d.get('type','')
            if t=='step': print(f'  [{d.get(\"step\",\"?\")}] {d.get(\"content\",\"\")}')
            elif t=='tool': print(f'  [tool:{d.get(\"name\",\"?\")}] args={d.get(\"args\",{})}')
            elif t=='tool_result': print(f'  [result:{d.get(\"name\",\"?\")}] {d.get(\"content\",\"\")[:200]}')
            elif t=='reasoning': print(f'  [think] {d.get(\"content\",\"\")}',end='')
            elif t=='delta': print(d.get('content',''),end='')
            elif t=='done': print()
            elif t=='error': print(f'  [ERROR] {d}')
            else: print(f'  [{t}] {str(d)[:200]}')
        except: print(f'  raw: {line[:200]}')
" 2>&1
""")
print(stdout.read().decode('utf-8', errors='replace'))

# Check logs after test
print('\n=== Logs after agent test ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 10 2>&1')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
