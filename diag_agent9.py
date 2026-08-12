"""Check LLM config and agent behavior"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check container env for LLM config
print('=== Container env for LLM ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge printenv 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 2. Check config.go
print('\n=== Config.go ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/config/config.go 2>/dev/null | head -120')
print(stdout.read().decode('utf-8', errors='replace'))

# 3. Check what provider_id the frontend sends
print('\n=== Frontend LLM config check ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
# Check if there's a user-level LLM config
curl -s http://localhost:8086/api/v1/llm/config -H "Authorization: Bearer $TOKEN" 2>&1
""")
print(stdout.read().decode('utf-8', errors='replace'))

# 4. Check what the /api/v1/ai/stream endpoint does
print('\n=== AI stream handler ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*Stream\\|func.*Chat\\|func.*AI\\|func.*ChatStream" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/handler/agent.go 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 5. Read the actual Run function to understand the loop
print('\n=== Run function (line 1687+) ===')
stdin, stdout, stderr = ssh.exec_command('sed -n "1680,1800p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

# 6. Check the callLLMWithTools function
print('\n=== callLLMWithTools ===')
stdin, stdout, stderr = ssh.exec_command('grep -n "func.*callLLM" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
lines = stdout.read().decode('utf-8', errors='replace').strip()
print(lines)
if lines:
    first_line = int(lines.split(':')[0])
    stdin, stdout, stderr = ssh.exec_command(f'sed -n "{first_line},{first_line+80}p" /vol1/docker/overlay2/eumu8o5ckz63u12lawk3tofea/diff/app/internal/agent/runner.go 2>/dev/null')
    print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
