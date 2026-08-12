"""Debug: test opencode-zen with verbose output"""
import paramiko
import sys
import io

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Test with verbose curl
print('=== Verbose test opencode-zen ===')
stdin, stdout, stderr = ssh.exec_command("""
curl -v -X POST "https://opencode.ai/zen/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model":"nemotron-3-ultra-free","messages":[{"role":"user","content":"hi"}],"max_tokens":10}' \
  -m 30 2>&1
""")
print(stdout.read().decode('utf-8', errors='replace'))

# Check DNS resolution
print('\n=== DNS check ===')
stdin, stdout, stderr = ssh.exec_command('nslookup opencode.ai 2>&1 | head -10')
print(stdout.read().decode('utf-8', errors='replace'))

# Check if we can reach the endpoint
print('\n=== HTTP check ===')
stdin, stdout, stderr = ssh.exec_command('curl -s -o /dev/null -w "%{http_code} %{time_total}s" https://opencode.ai/zen/v1/models -m 10 2>&1')
print(stdout.read().decode('utf-8', errors='replace'))

# Check container network
print('\n=== Container network test ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge curl -s -o /dev/null -w "%{http_code}" https://opencode.ai/zen/v1/models -m 10 2>&1')
print(stdout.read().decode('utf-8', errors='replace'))

# Check if there's a proxy or firewall
print('\n=== Network config ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge env | grep -i "proxy\\|http_proxy\\|https_proxy\\|no_proxy" 2>/dev/null')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
