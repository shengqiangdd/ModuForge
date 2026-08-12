"""Check what LLM the agent actually uses and if it's working"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check container logs for LLM calls
print('=== Recent LLM-related logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 100 2>&1 | grep -i "llm\\|provider\\|model\\|endpoint\\|api_key\\|token\\|403\\|401\\|200\\|agent.*run\\|agent.*iter" | tail -30')
print(stdout.read().decode())

# 2. Check the config that's passed to the agent
print('\n=== Config initialization ===')
stdin, stdout, stderr = ssh.exec_command("""
docker exec moduforge sh -c '
echo "=== ENV ==="
printenv | grep -i "LLM\\|MODEL\\|PROVIDER\\|API" 2>/dev/null
echo "=== /etc/environment ==="
cat /etc/environment 2>/dev/null
echo "=== /root/.bashrc ==="
cat /root/.bashrc 2>/dev/null | grep -i "export" | head -10
echo "=== process env ==="
cat /proc/1/environ 2>/dev/null | tr "\\0" "\\n" | grep -i "LLM\\|MODEL\\|PROVIDER\\|API" | head -10
'
""")
print(stdout.read().decode())

# 3. Check what the agent handler uses for LLM
print('\n=== Agent handler LLM resolution ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "EffectiveLLMKey\|LLMEndpoint\|LLMModel\|LLMProvider\|LLMApiKey\|cfg\\.LLM" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null | head -20')
print(stdout.read().decode())

# 4. Check config package
print('\n=== Config package ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "EffectiveLLMKey\\|LLMEndpoint\\|LLMModel\\|LLMProvider\\|LLMApiKey" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/config/config.go 2>/dev/null | head -20')
print(stdout.read().decode())

# 5. Read the config.go to understand how LLM is configured
print('\n=== Config.go full ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/config/config.go 2>/dev/null | head -100')
print(stdout.read().decode())

# 6. Check if opencode-go is the default and what key it needs
print('\n=== LLM providers (opencode-go) ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -s http://localhost:8086/api/v1/llm/providers | python3 -c "
import sys,json
providers = json.load(sys.stdin)['providers']
for p in providers:
    if 'opencode' in p['id'].lower():
        print(f\"ID: {p['id']}, endpoint: {p['endpoint']}, requires_key: {p.get('requires_key', 'N/A')}\")
        print(f\"  tier: {p.get('tier','N/A')}, is_free: {p.get('is_free','N/A')}\")
"
""")
print(stdout.read().decode())

# 7. Test: send a message and watch the logs
print('\n=== Test agent + watch logs ===')
stdin, stdout, stderr = ssh.exec_command("""
# Clear logs first
docker logs --tail 0 moduforge 2>/dev/null

TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Get first project
PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; ps=json.load(sys.stdin).get('projects',[]); print(ps[0]['id'] if ps else '')")

# Send a simple task
curl -s -X POST "http://localhost:8086/api/v1/ai/stream" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"messages":[{"role":"user","content":"List all files in this project"}],"project_id":"'$PID'"}' \
  -m 30 2>&1 | head -20

# Check logs
sleep 2
echo "--- LOGS ---"
docker logs moduforge --tail 20 2>&1
""")
print(stdout.read().decode())

ssh.close()
