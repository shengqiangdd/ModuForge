"""Fix DB permissions from inside container"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop container first
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Check if we can chmod on host
print('\n=== Check host filesystem ===')
stdin, stdout, stderr = ssh.exec_command("""
# Check filesystem type
df -T /vol1/docker/volumes/moduforge_moduforge_data/_data/ 2>/dev/null
echo "---"
# Try to copy and replace
cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db /tmp/moduforge_fix.db
chmod 664 /tmp/moduforge_fix.db
chown 1000:1000 /tmp/moduforge_fix.db
ls -la /tmp/moduforge_fix.db
echo "---"
# Replace
mv /tmp/moduforge_fix.db /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db
ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db
""", timeout=15)
print(stdout.read().decode(errors='replace'))

# If chown still doesn't work, try running container as root temporarily
print('\n=== Start container and fix from inside ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose up -d 2>&1')
print(stdout.read().decode())

import time
time.sleep(5)

# Try to fix from inside container using root
stdin, stdout, stderr = ssh.exec_command("""
# Check container user
docker exec moduforge id
echo "---"
# Check file perms inside container
docker exec moduforge ls -la /data/moduforge.db
echo "---"
# Try to make it writable
docker exec moduforge chmod 666 /data/moduforge.db 2>&1 || echo "chmod failed"
docker exec moduforge ls -la /data/moduforge.db
echo "---"
# Check if the issue is the directory
docker exec moduforge ls -la /data/
""", timeout=15)
print(stdout.read().decode(errors='replace'))

# Check container logs
print('\n=== Container logs ===')
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 5 2>&1')
print(stdout.read().decode(errors='replace'))

ssh.close()
