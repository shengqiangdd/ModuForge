"""Fix DB using temporary root container"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop container
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Use alpine container with root to fix permissions
print('\n=== Fix with alpine ===')
fix_cmd = """
docker run --rm \
  -v /vol1/docker/volumes/moduforge_moduforge_data/_data:/data \
  alpine sh -c '
    echo "Before:"
    ls -la /data/moduforge.db
    chown 1000:1000 /data/moduforge.db
    chmod 664 /data/moduforge.db
    chown 1000:1000 /data
    chmod 775 /data
    echo "After:"
    ls -la /data/moduforge.db
    ls -la /data/
  '
"""
stdin, stdout, stderr = ssh.exec_command(fix_cmd, timeout=30)
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print(err)

# Verify
print('\n=== Verify on host ===')
stdin, stdout, stderr = ssh.exec_command('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
print(stdout.read().decode())

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

# Test clear failed
print('\n=== Test clear failed ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" \
  -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
