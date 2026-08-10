# -*- coding: utf-8 -*-
"""
Deploy fixed ModuForge: rebuild image with correct entrypoint + fixed binary.
"""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, timeout=120):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode().strip()
    err = e.read().decode().strip()
    return out or err

print("=== Step 1: Upload files ===")

# Create temp build directory
run("mkdir -p /tmp/moduforge-fix")

# Upload Dockerfile.fix
sftp = ssh.open_sftp()
sftp.put(
    r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\Dockerfile.fix",
    "/tmp/moduforge-fix/Dockerfile"
)
print("  Uploaded Dockerfile")

# Upload fixed binary
sftp.put(
    r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\moduforge-server-linux",
    "/tmp/moduforge-fix/server"
)
print("  Uploaded binary")
sftp.close()

print("\n=== Step 2: Build new image ===")
result = run(
    "cd /tmp/moduforge-fix && docker build -t moduforge:patched .",
    timeout=180
)
print("Build:", result)

print("\n=== Step 3: Stop old container ===")
run("docker stop %s 2>/dev/null" % CONTAINER)
run("docker rm %s 2>/dev/null" % CONTAINER)

print("\n=== Step 4: Start new container ===")
result = run(
    'docker run -d --name %s '
    '--restart unless-stopped '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8086:8080 '
    'moduforge:patched' % CONTAINER
)
print("Run:", result)

print("\n=== Step 5: Wait and verify ===")
time.sleep(8)

status = run('docker inspect %s --format "{{.State.Status}}"' % CONTAINER)
print("Status:", status)

logs = run("docker logs %s --tail 15 2>&1" % CONTAINER)
print("Logs:", logs)

health = run("docker exec %s curl -s http://localhost:8080/health" % CONTAINER)
print("Health:", health)

# Verify binary fix
fix_count = run('docker exec %s strings /server | grep -c "WHERE name="' % CONTAINER)
print("Binary 'WHERE name=' count:", fix_count)

# Test Agent API
print("\n=== Step 6: Test Agent ===")
# Login first
login_result = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login_result[:200] if login_result else "empty")

# Get token
import json
try:
    token_data = json.loads(login_result)
    token = token_data.get("token", "")
    if token:
        # Test custom providers API
        providers = run(
            'curl -s http://localhost:8086/api/v1/llm/custom-providers '
            '-H "Authorization: Bearer %s"' % token
        )
        print("Custom providers:", providers[:300] if providers else "empty")
except:
    print("Could not parse login response")

print("\n=== Done ===")
ssh.close()
