# -*- coding: utf-8 -*-
"""Rebuild with simplified Dockerfile."""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=600):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    return o.read().decode().strip() or e.read().decode().strip()

# Upload simplified Dockerfile
print("=== Upload Dockerfile ===")
sftp = ssh.open_sftp()
sftp.put(
    r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\Dockerfile.simple",
    "/tmp/moduforge-build/Dockerfile"
)
sftp.close()

# Ensure entrypoint is in place
run("cp /tmp/moduforge-build/backend/docker-entrypoint.sh /tmp/moduforge-build/docker-entrypoint.sh")
run("chmod +x /tmp/moduforge-build/docker-entrypoint.sh")

# Verify files
print("Files:", run("ls -la /tmp/moduforge-build/"))
print("Frontend dist:", run("ls /tmp/moduforge-build/frontend/dist/ | head -5"))

# Build
print("\n=== Build ===")
result = run(
    "cd /tmp/moduforge-build && docker build -t moduforge:latest . 2>&1",
    timeout=600
)
print("Build:", result[-500:] if len(result) > 500 else result)

# Check image
print("\nImage:", run("docker images moduforge:latest --format '{{.ID}} {{.Size}}'"))

# Deploy
print("\n=== Deploy ===")
run("docker rm -f moduforge 2>/dev/null")
result = run(
    'docker run -d --name moduforge '
    '--restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8087:8080 '
    'moduforge:latest'
)
print("Run:", result)

time.sleep(10)
print("Status:", run('docker inspect moduforge --format "{{.State.Status}}"'))
print("Logs:", run("docker logs moduforge --tail 15 2>&1"))
print("Health:", run("curl -s http://localhost:8086/health"))

# Verify binary
fix = run('docker exec moduforge strings /server | grep -c "WHERE name=" 2>/dev/null')
print("Binary fix count:", fix)

ssh.close()
print("\nDone")
