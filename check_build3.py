"""Check build issues - fixed encoding"""
import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check build tasks
print('=== Build tasks ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds?limit=5" -H "Authorization: Bearer $TOKEN"
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check container logs
print('\n=== Container logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 20 2>&1')
sys.stdout.buffer.write(stdout.read())
print()

# Check build log streaming endpoint
print('\n=== Stream test ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -m 3 "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/stream" -H "Authorization: Bearer $TOKEN" 2>&1
""")
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
