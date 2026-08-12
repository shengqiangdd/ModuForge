"""Verify fix and test build APIs"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Wait for healthy
import time
for i in range(10):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'Container healthy at {i*2}s')
        break
else:
    print('Container not healthy!')
    # Check logs
    stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 20 2>&1')
    print(stdout.read().decode(errors='replace'))
    ssh.close()
    exit(1)

# Test APIs
print('\n=== Test APIs ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

echo "=== Projects ==="
curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
projects = json.load(sys.stdin)
for p in projects:
    print(f'  {p[\"id\"][:16]}... {p[\"name\"]}')
"

echo ""
echo "=== Clear failed builds ==="
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" \
  -H "Authorization: Bearer $TOKEN"

echo ""
echo ""
echo "=== Build tasks ==="
curl -s "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds?limit=5" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
data = json.load(sys.stdin)
builds = data if isinstance(data, list) else data.get('builds', [])
print(f'Total builds: {len(builds)}')
for b in builds[:5]:
    print(f'  status={b.get(\"status\",\"?\")} id={b.get(\"id\",\"?\")[:16]}')
" 2>&1
""", timeout=15)
print(stdout.read().decode(errors='replace'))

ssh.close()
