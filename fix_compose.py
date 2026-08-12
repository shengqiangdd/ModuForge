"""Write correct docker-compose.yml and restart"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Write correct docker-compose.yml
compose = """services:
  moduforge:
    build:
      context: .
      dockerfile: backend/Dockerfile
    image: moduforge:latest
    container_name: moduforge
    restart: unless-stopped
    ports:
      - "8086:8080"
    environment:
      - PORT=:8080
      - DATABASE_PATH=/data/moduforge.db
      - BUILD_DIR=/data/builds
      - MODULES_DIR=/data/modules
      - PROJECTS_DIR=/data/projects
      - GIN_MODE=release
      - JWT_SECRET=${JWT_SECRET:-}
      - MODUFORGE_DEV=0
      - TZ=Asia/Shanghai
    volumes:
      - /vol1/docker/moduforge-data:/data
      - moduforge_uploads:/app/uploads
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  moduforge_uploads:
    driver: local
"""

sftp = ssh.open_sftp()
with sftp.open('/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml', 'w') as f:
    f.write(compose)
sftp.close()
print('docker-compose.yml written')

# Start
print('\n=== Start ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())
err = stderr.read().decode(errors='replace')
if err: print(err)

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
    stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 10 2>&1')
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
