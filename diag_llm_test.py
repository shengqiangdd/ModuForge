"""Test if opencode-zen LLM endpoint is working"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Test opencode-zen endpoint directly
print('=== Test opencode-zen endpoint ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST "https://opencode.ai/zen/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash-free",
    "messages": [{"role":"user","content":"Say hello"}],
    "stream": false,
    "max_tokens": 100
  }' -m 30 2>&1
""")
result = stdout.read().decode('utf-8', errors='replace')
print(result[:1000])

# Test opencode-go endpoint
print('\n=== Test opencode-go endpoint ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -s -X POST "https://opencode.ai/zen/go/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-zen" \
  -d '{
    "model": "mimo-v2.5",
    "messages": [{"role":"user","content":"Say hello"}],
    "stream": false,
    "max_tokens": 100
  }' -m 30 2>&1
""")
result = stdout.read().decode('utf-8', errors='replace')
print(result[:1000])

# Check if there's a working free model
print('\n=== Test other free models ===')
for model in ["mimo-v2.5-free", "nemotron-3-ultra-free", "laguna-s-2.1-free"]:
    stdin, stdout, stderr = ssh.exec_command(f"""
curl -s -X POST "https://opencode.ai/zen/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{{
    "model": "{model}",
    "messages": [{{"role":"user","content":"Say hello"}}],
    "stream": false,
    "max_tokens": 50
  }}' -m 15 2>&1 | head -c 200
""")
    result = stdout.read().decode('utf-8', errors='replace')
    print(f'{model}: {result[:200]}')

ssh.close()
