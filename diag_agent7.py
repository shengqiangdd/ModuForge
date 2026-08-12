"""Deep diagnose: why agent doesn't use tools - check LLM config and tool defs"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check what LLM config the agent uses
print('=== LLM Config from env/config ===')
stdin, stdout, stderr = ssh.exec_command("""
docker exec moduforge sh -c '
echo "=== env ==="
env | grep -i "LLM\\|MODEL\\|PROVIDER\\|API_KEY\\|OPENAI" 2>/dev/null
echo "=== config file ==="
cat /app/.env 2>/dev/null || cat /app/config.yaml 2>/dev/null || echo "no config file"
echo "=== DB settings ==="
cd /data && python3 -c "
import sqlite3
conn = sqlite3.connect(\"moduforge.db\")
try:
    rows = conn.execute(\"SELECT key, value FROM settings\").fetchall()
    for k,v in rows:
        print(f\"{k}={v}\")
except:
    print(\"no settings table\")
" 2>/dev/null
'
""")
print(stdout.read().decode())

# 2. Check the getToolDefinitions function
print('\n=== getToolDefinitions function ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*getToolDef\|func.*GetToolDef" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
print(stdout.read().decode())

# Read the function
stdin, stdout, stderr = ssh.exec_command('sed -n "/func.*getToolDef/,/^func /p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -80')
print(stdout.read().decode())

# 3. Check how tools are actually called (tool_call handling)
print('\n=== Tool call handling ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "tool_call\|toolCall\|ToolCall\|function_call\|functionCall" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -30')
print(stdout.read().decode())

# 4. Check the LLM response parsing
print('\n=== LLM response parsing ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "tool_calls\|toolCall\|finish_reason\|function_call\|delta.*tool\|delta.*function" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -20')
print(stdout.read().decode())

# 5. Check if there's a "no tools" mode or tool filtering
print('\n=== Tool filtering ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "skipTool\|filterTool\|excludeTool\|planMode\|PlanMode\|noTool\|NoTool\|toolFilter\|ToolFilter" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -10')
print(stdout.read().decode())

# 6. Check the Run function - what happens in the loop
print('\n=== Run function main loop ===')
stdin, stdout, stderr = ssh.exec_command('sed -n "/func.*Run\(/,/^func /p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -100')
print(stdout.read().decode())

# 7. Test with a working API key
print('\n=== Test LLM with API key ===')
stdin, stdout, stderr = ssh.exec_command("""
# Try opencode-go with the right key
curl -s -X POST "https://opencode.ai/zen/go/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-zen" \
  -d '{
    "model": "mimo-v2.5",
    "messages": [{"role":"user","content":"Say hello"}],
    "stream": false,
    "max_tokens": 100
  }' -m 30 2>&1 | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d,indent=2)[:1000])" 2>&1
""")
print(stdout.read().decode())

ssh.close()
