"""Check what endpoint the frontend actually calls and the agent run endpoint"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check the StreamChat handler
print('=== StreamChat handler ===')
stdin, stdout, stderr = ssh.exec_command('find /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler -name "*stream*" -o -name "*Stream*" 2>/dev/null')
out = stdout.read().decode('utf-8', errors='replace').strip()
print(out)

# Find the StreamChat function
stdin, stdout, stderr = ssh.exec_command('grep -rn "func.*StreamChat" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/ 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Read the stream handler
print('\n=== Stream handler file ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/ai_stream.go 2>/dev/null | head -150')
print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check what the frontend JS calls
print('\n=== Frontend API calls ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge grep -rn "ai/stream\\|ai/chat\\|agent/run\\|/api/v1" /app/dist/assets/*.js 2>/dev/null | head -20')
print(stdout.read().decode('utf-8', errors='replace'))

# 4. Test the agent/run endpoint directly
print('\n=== Test agent/run endpoint ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; ps=json.load(sys.stdin).get('projects',[]); print(ps[0]['id'] if ps else '')")

# Test agent/run (the full agent with tools)
curl -s -X POST "http://localhost:8086/api/v1/agent/run" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"task":"List all files in the project using the list_dir tool","project_id":"'$PID'","agent_mode":"act"}' \
  -m 60 2>&1 | head -30
""")
print(stdout.read().decode('utf-8', errors='replace'))

# 5. Check the agent run handler
print('\n=== Agent Run handler (from agent.go) ===')
stdin, stdout, stderr = ssh.exec_command('sed -n "100,200p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/agent.go 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
