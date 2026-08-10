# -*- coding: utf-8 -*-
import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    out = o.read().decode().strip()
    err = e.read().decode().strip()
    return out or err

# Step 1: Remove all broken containers
print("=== Cleanup ===")
run("docker rm -f moduforge moduforge-work 2>/dev/null")

# Step 2: Fix the original image by running a temp container with /bin/sh
# The key: use --entrypoint /bin/sh AND pass -c to run a command
print("\n=== Fixing image ===")

# Create a container that will fix the entrypoint
run_cmd = (
    'docker run --rm '
    '--entrypoint /bin/sh '
    '-v moduforge_moduforge_data:/data '
    'moduforge:latest '
    '-c "chmod +x /docker-entrypoint.sh && ls -la /docker-entrypoint.sh"'
)
print("Fix entrypoint:", run(run_cmd))

# Now copy our fixed binary into a running container
# First start a container with /bin/sh as entrypoint that stays running
print("\n=== Creating fixed container ===")
run(
    'docker run -d --name moduforge-fix '
    '--entrypoint /bin/sh '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8086:8080 '
    'moduforge:latest '
    '-c "while true; do sleep 3600; done"'
)
time.sleep(2)

# Fix entrypoint inside
print("Chmod entrypoint:", run('docker exec moduforge-fix chmod +x /docker-entrypoint.sh'))

# Copy our fixed binary
print("Copy binary:", run('docker cp /tmp/moduforge-server-new moduforge-fix:/server'))
print("Chmod binary:", run('docker exec moduforge-fix chmod +x /server'))

# Verify
print("Entry check:", run('docker exec moduforge-fix ls -la /docker-entrypoint.sh'))
print("Server check:", run('docker exec moduforge-fix ls -la /server'))
print("Binary fix:", run('docker exec moduforge-fix strings /server | grep -c "WHERE name="'))

# Commit the fixed image
print("\n=== Committing ===")
print("Commit:", run('docker commit moduforge-fix moduforge:fixed'))
run("docker rm -f moduforge-fix")

# Step 3: Start the real container from the fixed image
print("\n=== Starting final container ===")
print("Run:", run(
    'docker run -d --name moduforge --restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8086:8080 '
    'moduforge:fixed'
))

time.sleep(8)
print("Status:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("Logs:", run('docker logs moduforge --tail 15 2>&1'))
print("Health:", run('docker exec moduforge curl -s http://localhost:8080/health'))
print("Binary:", run('docker exec moduforge strings /server | grep -c "WHERE name="'))

ssh.close()
