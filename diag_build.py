"""Diagnose build page issues: 500 on clear failed, log refresh, build failure"""
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# 1. Check the 500 error on clear failed
print('=== Test clear failed API ===')
stdin, stdout, stderr = ssh.exec_command("""
# Login as csq
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Find project with failed builds
echo "=== Projects ==="
curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
projects = json.load(sys.stdin)
for p in projects:
    print(f'  {p[\"id\"][:16]}... {p[\"name\"]}')
"

# Try clear failed on AndroBoost project
echo ""
echo "=== Clear failed builds ==="
curl -sv -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" \
  -H "Authorization: Bearer $TOKEN" 2>&1 | tail -20
""", timeout=15)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print('STDERR:', err[:500])

# 2. Check build logs streaming endpoint
print('\n=== Check build log endpoints ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Check builds list
echo "=== Builds ==="
curl -s "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds?limit=3" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
data = json.load(sys.stdin)
builds = data if isinstance(data, list) else data.get('builds', [])
print(f'Total builds: {len(builds)}')
for b in builds[:3]:
    print(f'  id={b.get(\"id\",\"?\")[:16]} status={b.get(\"status\",\"?\")} created={b.get(\"created_at\",\"?\")[:19]}')
"
""", timeout=15)
print(stdout.read().decode(errors='replace'))

# 3. Check container logs for 500 errors
print('\n=== Recent container logs (errors) ===')
stdin, stdout, stderr = ssh.exec_command("""
docker logs moduforge --tail 50 2>&1 | grep -i "500\\|error\\|failed\\|panic" | tail -20
""", timeout=10)
print(stdout.read().decode(errors='replace'))

ssh.close()
