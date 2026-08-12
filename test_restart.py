"""Test: restart container and verify DB stays intact"""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Start container
print('=== Start ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())

# Wait for healthy
for i in range(10):
    time.sleep(2)
    stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
    health = stdout.read().decode().strip()
    if '"ok"' in health:
        print(f'Healthy at {i*2}s')
        break

# Test clear failed
print('\n=== Test clear failed ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

# Now restart container and check if DB survives
print('\n=== Restart container ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose restart 2>&1')
print(stdout.read().decode())

time.sleep(5)

# Check health
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health 2>&1')
print(f'Health after restart: {stdout.read().decode().strip()}')

# Check DB integrity after restart
print('\n=== DB after restart ===')
stdin, stdout, stderr = ssh.exec_command("""
python3 -c "
import sqlite3
conn = sqlite3.connect('/vol1/docker/moduforge-data/moduforge.db')
result = conn.execute('PRAGMA integrity_check').fetchone()
print(f'Integrity: {result[0]}')
for t in ['users', 'projects', 'ai_conversations']:
    cnt = conn.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
    print(f'  {t}: {cnt}')
conn.close()
"
""")
print(stdout.read().decode(errors='replace'))

# Test clear failed again
print('\n=== Test clear failed after restart ===')
stdin, stdout, stderr = ssh.exec_command("""
TOKEN=$(curl -s -X POST http://localhost:8086/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"csq","password":"csq0216"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
curl -s -X DELETE "http://localhost:8086/api/v1/projects/155f1629-6e33-4407-b348-f28698f6f5cd/builds/failed" -H "Authorization: Bearer $TOKEN"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
