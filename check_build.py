"""Check build page issues: log refresh + build failures"""
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

# List all projects
echo "=== Projects ==="
curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
for p in json.load(sys.stdin):
    print(f'  {p[\"id\"][:16]}... {p[\"name\"]}')
"

# Check builds for AndroBoost project
echo ""
echo "=== AndroBoost builds ==="
curl -s "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds?limit=5" -H "Authorization: Bearer $TOKEN" 2>&1

echo ""
echo "=== Container logs (recent) ==="
docker logs moduforge --tail 20 2>&1 | grep -i "build\\|error\\|500" | tail -10
""")
print(stdout.read().decode(errors='replace'))

# Check frontend build page source for log refresh issue
print('\n=== Frontend build log streaming ===')
stdin, stdout, stderr = ssh.exec_command("""
# Check if there's a build log streaming endpoint
curl -s "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/stream" -H "Authorization: Bearer $(curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"csq","password":"csq0216"}' | python3 -c 'import sys,json; print(json.load(sys.stdin)[\"token\"])')" -m 3 2>&1 || echo "Stream endpoint not available"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
