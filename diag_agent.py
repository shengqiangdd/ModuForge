"""Diagnose ModuForge Agent engine - why it's weak"""
import paramiko, sys, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check agent configuration
print('=== Agent Config ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Check agent settings
echo "--- Settings ---"
curl -s http://localhost:8086/api/v1/agent/settings -H "Authorization: Bearer $TOKEN" 2>&1

echo ""
echo "--- Provider configs ---"
curl -s http://localhost:8086/api/v1/llm/providers -H "Authorization: Bearer $TOKEN" 2>&1

echo ""
echo "--- Custom providers ---"
curl -s http://localhost:8086/api/v1/llm/custom-providers -H "Authorization: Bearer $TOKEN" 2>&1
""")
sys.stdout.buffer.write(stdout.read())
print()

# 2. Check agent engine source code
print('\n=== Agent Engine Source ===')
stdin, stdout, stderr = ssh.exec_command("""
# Find agent engine files
find /vol1/docker/overlay2 -name "runner.go" -path "*agent*" 2>/dev/null | head -3
echo "---"
find /vol1/docker/overlay2 -name "agent.go" -path "*agent*" 2>/dev/null | head -3
echo "---"
# Check if there's an agent config file
find /vol1/docker/overlay2 -name "act.md" -o -name "agent.md" 2>/dev/null | head -5
echo "---"
# Check the agent's system prompt
docker exec moduforge cat /app/act.md 2>/dev/null | head -30 || echo "no act.md"
echo "==="
docker exec moduforge cat /app/agent.md 2>/dev/null | head -30 || echo "no agent.md"
""")
sys.stdout.buffer.write(stdout.read())
print()

# 3. Test agent with a real coding task
print('\n=== Test agent: real coding task ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X POST http://localhost:8086/api/v1/ai/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"Read the file src/go/main.go and tell me what API endpoints it defines. Then add a new GET /api/health endpoint that returns {\"status\":\"ok\",\"uptime\":uptime_seconds}."}],"project_id":"1785249992652501794-1864"}' \
  -m 60 2>&1 | grep "delta" | python3 -c "
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

# 4. Check container logs for agent errors
print('\n=== Agent errors in logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 50 2>&1 | grep -i "agent\\|llm\\|error\\|403\\|401\\|timeout" | tail -20')
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
