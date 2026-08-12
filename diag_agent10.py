"""Check if agent LLM calls are failing silently"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check container logs for LLM errors
print('=== Recent container logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 50 2>&1')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Check the LLM call function
print('\n=== LLM call function ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*callLLM\\|func.*CallLLM\\|func.*doLLM\\|func.*sendLLM" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
lines = stdout.read().decode('utf-8', errors='replace').strip()
print(lines)
if lines:
    first_line = int(lines.split('\n')[0].split(':')[0])
    stdin, stdout, stderr = ssh.exec_command(f'sed -n "{first_line},{first_line+60}p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
    print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check the LLM HTTP client
print('\n=== LLM HTTP client ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "LLMHTTPClient\\|func.*LLMHTTP\\|func.*newHTTP" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/llm*.go 2>/dev/null | head -10')
print(stdout.read().decode('utf-8', errors='replace'))

# 4. Find LLM-related files
print('\n=== LLM files ===')
stdin, stdout, stderr = ssh.exec_command('find /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal -name "*llm*" -o -name "*LLM*" 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 5. Check the llm package
print('\n=== llm package ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/llm/llm.go 2>/dev/null | head -100')
print(stdout.read().decode('utf-8', errors='replace'))

# 6. Test: send a task and capture full response
print('\n=== Full agent test ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; ps=json.load(sys.stdin).get('projects',[]); print(ps[0]['id'] if ps else '')")

# Send task and capture full response
curl -s -X POST "http://localhost:8086/api/v1/ai/stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"Read the file src/go/main.go and list its functions"}],"project_id":"'$PID'"}' \
  -m 60 2>&1

echo "---END---"
""")
print(stdout.read().decode('utf-8', errors='replace'))

# 7. Check logs after test
print('\n=== Logs after test ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 30 2>&1')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
