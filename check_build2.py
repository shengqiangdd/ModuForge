"""Check build issues"""
import paramiko

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
builds = stdout.read().decode(errors='replace')
print(builds[:500])

# Check container logs for build errors
print('\n=== Recent build logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 30 2>&1 | grep -i "build\\|error\\|failed\\|500" | tail -15')
print(stdout.read().decode(errors='replace'))

# Check if there's a build log streaming endpoint
print('\n=== Stream endpoint ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -m 3 "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/stream" -H "Authorization: Bearer $TOKEN" 2>&1 || echo "timeout/empty"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
