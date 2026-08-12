"""Find source code and check build routes"""
import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Find source code
print('=== Find source ===')
stdin, stdout, stderr = ssh.exec_command("""
find /vol1 -name "routes.go" -path "*moduforge*" 2>/dev/null | head -5
echo "---"
find /vol1 -name "build*.go" -path "*moduforge*" 2>/dev/null | head -10
echo "---"
find /vol1 -name "Build*.svelte" 2>/dev/null | head -5
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check what endpoints exist for builds
print('\n=== Build endpoints test ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Test various build endpoints
for ep in \
  "api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds" \
  "api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/logs" \
  "api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/build/stream" \
  "api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/build/logs"; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8086/$ep" -H "Authorization: Bearer $TOKEN" -m 3)
  echo "$ep -> $code"
done
""")
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
