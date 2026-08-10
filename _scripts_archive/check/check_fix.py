# -*- coding: utf-8 -*-
import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check state
print("Status:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("Image:", run('docker inspect moduforge --format "{{.Config.Image}}"'))
print("Entrypoint:", run('docker inspect moduforge --format "{{.Config.Entrypoint}}"'))
print("Mounts:", run('docker inspect moduforge --format "{{range .Mounts}}{{.Name}}:{{.Destination}} {{end}}"'))

# Remove and recreate properly
print("\n=== Recreating ===")
run("docker rm -f moduforge 2>/dev/null")

# Run with --entrypoint /bin/sh to bypass broken entrypoint, fix inside, then commit
# First, let's try running directly with the fixed binary approach
# Create a working container from the image
print("Create:", run(
    'docker create --name moduforge-work '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8086:8080 '
    'moduforge:latest'
))

# Start with /bin/sh to get a shell
print("Start sh:", run('docker start moduforge-work'))
time.sleep(2)

# Fix entrypoint permissions
print("Fix entrypoint:", run('docker exec moduforge-work chmod +x /docker-entrypoint.sh'))

# Copy our fixed binary
print("Copy binary:", run('docker cp /tmp/moduforge-server-new moduforge-work:/server'))
print("Chmod binary:", run('docker exec moduforge-work chmod +x /server'))

# Verify
print("Entrypoint exec:", run('docker exec moduforge-work ls -la /docker-entrypoint.sh'))
print("Server exec:", run('docker exec moduforge-work ls -la /server'))
print("Server strings:", run('docker exec moduforge-work strings /server | grep -c "WHERE name="'))

# Commit as new image
print("Commit:", run('docker commit moduforge-work moduforge:fixed'))
run("docker rm -f moduforge-work")

# Now create the final container from the fixed image
print("\n=== Starting final container ===")
run("docker rm -f moduforge 2>/dev/null")
print("Run:", run(
    'docker run -d --name moduforge --restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8086:8080 '
    'moduforge:fixed'
))

time.sleep(5)
print("Status:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("Logs:", run('docker logs moduforge --tail 15 2>&1'))
print("Health:", run('docker exec moduforge curl -s http://localhost:8080/health'))
print("Binary check:", run('docker exec moduforge strings /server | grep -c "WHERE name="'))

ssh.close()
