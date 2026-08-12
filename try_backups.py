"""Try different backup files to find working one"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop container
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Try each backup file
backups = [
    '/vol1/docker/volumes/moduforge_data/_data/moduforge.db.bak',
    '/vol1/docker/volumes/moduforge_data/_data/moduforge.db.pre_recovery.bak',
    '/vol1/docker/volumes/moduforge_data/_data/moduforge_recovered.db',
    '/vol1/docker/volumes/moduforge_data/_data.bak.202608100015/moduforge.db',
]

for bak in backups:
    print(f'\n=== Try {bak.split("/")[-1]} ===')
    cmd = f"""
docker run --rm \
  -v {bak}:/src/db:ro \
  -v /vol1/docker/moduforge-data:/dst \
  alpine sh -c '
    cp /src/db /dst/moduforge.db
    apk add --no-cache sqlite3 2>/dev/null
    echo "Integrity:"
    sqlite3 /dst/moduforge.db "PRAGMA integrity_check;"
    echo "Users:"
    sqlite3 /dst/moduforge.db "SELECT COUNT(*) FROM users;"
    echo "Projects:"
    sqlite3 /dst/moduforge.db "SELECT COUNT(*) FROM projects;"
    echo "Conversations:"
    sqlite3 /dst/moduforge.db "SELECT COUNT(*) FROM ai_conversations;"
  '
"""
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=60)
    out = stdout.read().decode(errors='replace')
    print(out)
    
    if 'ok' in out and 'Users:' in out:
        print('>>> THIS ONE WORKS!')
        break

# Start and test
print('\n=== Start ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())

import time
print('\n=== Wait ===')
for i in range(10):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'  {i*2}s: healthy!')
        break

print('\n=== Test clear failed ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
