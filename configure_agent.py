"""Configure ModuForge agent to use a working free LLM model"""
import paramiko
import sys
import io
import json

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Get auth token
print('=== Getting auth token ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])"
""")
token = stdout.read().decode('utf-8', errors='replace').strip()
print(f'Token: {token[:20]}...')

# 2. Test which free models actually work
print('\n=== Testing free models ===')
working_models = []
for model in ["deepseek-v4-flash-free", "mimo-v2.5-free", "nemotron-3-ultra-free", "laguna-s-2.1-free", "big-pickle"]:
    stdin, stdout, stderr = ssh.exec_command(f"""
curl -s -X POST "https://opencode.ai/zen/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{{"model":"{model}","messages":[{{"role":"user","content":"Say hi"}}],"stream":false,"max_tokens":20}}' \
  -m 15 2>&1 | head -c 300
""")
    result = stdout.read().decode('utf-8', errors='replace').strip()
    works = "choices" in result or "error" not in result.lower()
    if result and "choices" in result:
        working_models.append(model)
        print(f'  {model}: WORKS')
    else:
        print(f'  {model}: FAILS ({result[:100]})')

print(f'\nWorking models: {working_models}')

# 3. Set the agent to use a working model
if working_models:
    best_model = working_models[0]  # Use the first working model
    print(f'\n=== Setting agent to use {best_model} ===')
    
    # Update LLM config via API
    stdin, stdout, stderr = ssh.exec_command(f"""
curl -s -X POST "http://localhost:8086/api/v1/llm/config" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{{"provider":"opencode-zen","model_id":"{best_model}"}}'
""")
    result = stdout.read().decode('utf-8', errors='replace')
    print(f'Config update: {result}')
    
    # Also save provider config
    stdin, stdout, stderr = ssh.exec_command(f"""
curl -s -X PUT "http://localhost:8086/api/v1/llm/provider-config" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{{"id":"opencode-zen","endpoint":"https://opencode.ai/zen/v1/chat/completions"}}'
""")
    result = stdout.read().decode('utf-8', errors='replace')
    print(f'Provider config: {result}')

# 4. Test agent with the working model
print('\n=== Test agent with working model ===')
stdin, stdout, stderr = ssh.exec_command(f"""
PID=$(curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}" | python3 -c "import sys,json; ps=json.load(sys.stdin).get('projects',[]); print(ps[0]['id'] if ps else '')")

curl -s -X POST "http://localhost:8086/api/v1/agent/run" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{{"task":"List all files in this project using the list_dir tool","project_id":"{chr(36)}PID","agent_mode":"act","provider_id":"opencode-zen","model":"{best_model}"}}' \
  -m 60 2>&1 | head -50
""")
result = stdout.read().decode('utf-8', errors='replace')
print(result[:2000])

# 5. Check logs
print('\n=== Logs after test ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 15 2>&1')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
