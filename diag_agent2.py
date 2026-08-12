"""Diagnose ModuForge Agent engine source code"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Read runner.go - the agent engine
print('=== RUNNER.GO (first 300 lines) ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge cat /app/internal/agent/runner.go 2>/dev/null | head -300')
print(stdout.read().decode())

# 2. Read agent.md prompt
print('\n=== AGENT.MD ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge cat /app/internal/agent/prompts/agent.md 2>/dev/null')
print(stdout.read().decode())

# 3. Read act.md prompt
print('\n=== ACT.MD ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge cat /app/internal/agent/prompts/act.md 2>/dev/null')
print(stdout.read().decode())

# 4. Check what tools/skills the agent has
print('\n=== Agent Skills ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s http://localhost:8086/api/v1/agent/skills -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode())

# 5. Check agent loop/iteration limits
print('\n=== Agent limits in runner.go ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge grep -n "max_iter\\|MaxIter\\|timeout\\|Timeout\\|MAX_RESULT\\|MaxResult\\|tool_call\\|ToolCall\\|execute_tool\\|ExecuteTool\\|read_file\\|write_file\\|list_files\\|search" /app/internal/agent/runner.go 2>/dev/null | head -50')
print(stdout.read().decode())

# 6. Test agent with a simple coding task
print('\n=== TEST: Agent coding task ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Get project ID
PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; ps=json.load(sys.stdin)['projects']; print(ps[0]['id']) if ps else print('')")
echo "Project: $PID"

# Send coding task
curl -s -X POST "http://localhost:8086/api/v1/ai/stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"Read src/go/main.go and add a new GET /api/v1/health endpoint"}],"project_id":"'$PID'"}' \
  -m 120 2>&1 | head -100
""")
print(stdout.read().decode())

ssh.close()
