"""Restore DB from old volume backup"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Copy DB from old volume to current volume
print('=== Restore DB from backup ===')
cmd = """
docker run --rm \
  -v /vol1/docker/volumes/moduforge_data/_data:/src:ro \
  -v /vol1/docker/volumes/moduforge_moduforge_data/_data:/dst \
  alpine sh -c '
    echo "=== Source files ==="
    ls -la /src/
    
    echo ""
    echo "=== Copy DB ==="
    cp /src/moduforge.db /dst/moduforge.db
    
    echo ""
    echo "=== Destination ==="
    ls -la /dst/
    
    echo ""
    echo "=== Quick verify ==="
    apk add --no-cache sqlite3 2>/dev/null
    sqlite3 /dst/moduforge.db "SELECT COUNT(*) FROM users;" 2>&1 || echo "DB check failed"
    sqlite3 /dst/moduforge.db "SELECT COUNT(*) FROM projects;" 2>&1 || echo "DB check failed"
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=60)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print(err)

# Start container
print('\n=== Start ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())

# Wait
import time
print('\n=== Wait ===')
for i in range(15):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'  {i*2}s: healthy!')
        break
    print(f'  {i*2}s: {health[:50]}')
else:
    stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 5 2>&1')
    print(stdout.read().decode(errors='replace'))

# Test
print('\n=== Test clear failed ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "Projects:"
curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys, json
for p in json.load(sys.stdin): print(f'  {p[\"name\"]}')
"
echo ""
echo "Clear failed:"
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
