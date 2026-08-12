"""Deep diagnose agent engine - why it's weak"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Find actual runner.go in running container
print('=== Runner.go location ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find /app -name "runner.go" -type f 2>/dev/null')
out = stdout.read().decode().strip()
print(out if out else "NOT FOUND in container")

# 2. Find agent prompts
print('\n=== Agent prompts location ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge find /app -name "agent.md" -o -name "act.md" -type f 2>/dev/null')
out = stdout.read().decode().strip()
print(out if out else "NOT FOUND")

# 3. Read the actual runner.go from running container
print('\n=== RUNNER.GO (actual) ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge cat /app/internal/agent/runner.go 2>/dev/null | head -100')
out = stdout.read().decode().strip()
print(out if out else "EMPTY")

# 4. Check container binary
print('\n=== Container binary ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /app/moduforge 2>/dev/null || docker exec moduforge ls -la /app/backend 2>/dev/null || docker exec moduforge ls -la /app/ | head -20')
print(stdout.read().decode())

# 5. Check what the container actually runs
print('\n=== Container CMD ===')
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format="{{.Config.Cmd}}" 2>/dev/null')
print(stdout.read().decode())

# 6. Check agent loop limits from the binary
print('\n=== Agent limits in source ===')
stdin, stdout, stderr = ssh.exec_command("""
docker exec moduforge sh -c 'strings /app/moduforge 2>/dev/null | grep -i "max_iter\\|MaxIter\\|max_result\\|MaxResult\\|MAX_LOOP\\|tool_timeout" | head -20'
""")
out = stdout.read().decode().strip()
print(out if out else "NOT FOUND")

# 7. Check what the agent actually does when asked to code
print('\n=== Full agent test ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Get first project
PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; ps=json.load(sys.stdin).get('projects',[]); print(ps[0]['id'] if ps else '')")
echo "PID: $PID"

# List files in project
echo "--- Files ---"
curl -s "http://localhost:8086/api/v1/projects/$PID/files" -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json
data=json.load(sys.stdin)
files=data if isinstance(data,list) else data.get('files',[])
for f in files[:10]:
    print(f.get('path','?'))
"

# Send a real coding task
echo "--- Agent test ---"
curl -s -X POST "http://localhost:8086/api/v1/ai/stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"List all files in the project, then read src/go/main.go and tell me what it does"}],"project_id":"'$PID'"}' \
  -m 120 2>&1 | python3 -c "
import sys
for line in sys.stdin:
    line=line.strip()
    if 'delta' in line and 'content' in line:
        try:
            data=line.split('data: ')[1]
            import json
            obj=json.loads(data)
            print(obj.get('content',''),end='',flush=True)
        except: pass
    elif 'error' in line.lower():
        print(line)
"
""")
print(stdout.read().decode())

ssh.close()
