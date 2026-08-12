"""Switch to bind mount and fix DB"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Create a host directory for bind mount
print('\n=== Prepare bind mount directory ===')
cmds = [
    # Create directory
    'mkdir -p /vol1/docker/moduforge-data',
    # Copy DB from named volume
    'cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db /vol1/docker/moduforge-data/moduforge.db',
    # Copy .env
    'cp /vol1/docker/volumes/moduforge_moduforge_data/_data/.env /vol1/docker/moduforge-data/.env 2>/dev/null || true',
    # Fix permissions - make world writable for container
    'chmod 666 /vol1/docker/moduforge-data/moduforge.db',
    'chmod 777 /vol1/docker/moduforge-data',
    # Verify
    'ls -la /vol1/docker/moduforge-data/',
]
for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode(errors='replace')
    if out: print(out)

# Update docker-compose.yml to use bind mount
print('\n=== Update docker-compose.yml ===')
compose_fix = """
# Replace volume mounts with bind mounts
sed -i 's|moduforge_data:/data|/vol1/docker/moduforge-data:/data|' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml
# Remove the volumes section for moduforge_data
sed -i '/^volumes:/,/^  moduforge_uploads:/ { /^  moduforge_data:/d; }' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml
"""
stdin, stdout, stderr = ssh.exec_command(compose_fix, timeout=10)
print(stdout.read().decode(errors='replace'))

# Show updated compose
print('\n=== Updated docker-compose.yml ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml')
print(stdout.read().decode(errors='replace'))

# Start
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
print('\n=== Test ===')
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
