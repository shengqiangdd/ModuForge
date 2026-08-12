"""Diagnose why agent doesn't use tools"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check what model the agent uses
print('=== Check agent LLM config ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Check DB for agent settings
docker exec moduforge sqlite3 /data/moduforge.db "SELECT key, value FROM settings WHERE key LIKE '%llm%' OR key LIKE '%model%' OR key LIKE '%provider%' OR key LIKE '%agent%';" 2>/dev/null

echo "---"
# Check what model is configured
docker exec moduforge sqlite3 /data/moduforge.db "SELECT * FROM settings;" 2>/dev/null | head -20
""")
print(stdout.read().decode())

# 2. Check the agent handler - how it resolves LLM config
print('\n=== Agent handler ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/agent.go 2>/dev/null | head -150')
print(stdout.read().decode())

# 3. Check how tools are registered and executed
print('\n=== Tool execution in runner.go ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*executeTool\|func.*ExecuteTool\|func.*handleToolCall\|func.*HandleToolCall\|func.*runTool\|func.*RunTool\|func.*toolExec\|ToolExec" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -20')
print(stdout.read().decode())

# 4. Check what the LLM actually receives (tools definition)
print('\n=== Tool definitions sent to LLM ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "ToolDef\|toolDef\|tools.*append\|tools.*json\|function.*name" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -20')
print(stdout.read().decode())

# 5. Check the actual tool definitions
print('\n=== Tool definitions (first few) ===')
stdin, stdout, stderr = ssh.exec_command('grep -A 5 "func.*toolDefs\|func.*ToolDefs\|func.*getTools\|func.*GetTools\|func.*buildToolDefs" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -30')
print(stdout.read().decode())

# 6. Check the LLM call format
print('\n=== LLM call format ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "chat/completions\|messages.*append\|tools.*json\|\"role\".*\"tool\"\|\"role\".*\"assistant\"" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -20')
print(stdout.read().decode())

# 7. Test with opencode-go free model (should support tools)
print('\n=== Test opencode-go LLM with tools ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST "https://opencode.ai/zen/go/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mimo-v2.5",
    "messages": [{"role":"user","content":"List all files in the project using the list_dir tool"}],
    "tools": [{"type":"function","function":{"name":"list_dir","description":"List files","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":[]}}}],
    "stream": false
  }' -m 30 2>&1 | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d,indent=2)[:2000])" 2>&1
""")
print(stdout.read().decode())

ssh.close()
