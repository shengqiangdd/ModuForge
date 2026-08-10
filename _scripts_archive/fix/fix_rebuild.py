# -*- coding: utf-8 -*-
"""Fix and rebuild."""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    return o.read().decode().strip() or e.read().decode().strip()

# Check what's in the build context
print("=== Build context ===")
print(run("ls -la /tmp/moduforge-build/backend/"))
print(run("cat /tmp/moduforge-build/backend/Dockerfile | head -55"))

# Check if entrypoint is in the right place
print("\nEntrypoint exists:", run("test -f /tmp/moduforge-build/backend/docker-entrypoint.sh && echo YES || echo NO"))
print("Entrypoint content:", run("cat /tmp/moduforge-build/backend/docker-entrypoint.sh"))

# The issue: Dockerfile COPY is relative to build context root
# Dockerfile is at backend/Dockerfile, so COPY looks in backend/
# But the tar extracted to /tmp/moduforge-build/backend/docker-entrypoint.sh
# So the COPY should work... unless the Docker build context is wrong

# Let me check the docker build command - it uses -f backend/Dockerfile
# which means the build context is /tmp/moduforge-build (the parent)
# So COPY docker-entrypoint.sh looks for /tmp/moduforge-build/docker-entrypoint.sh
# NOT /tmp/moduforge-build/backend/docker-entrypoint.sh!

# Fix: copy entrypoint to build context root
print("\n=== Fix ===")
run("cp /tmp/moduforge-build/backend/docker-entrypoint.sh /tmp/moduforge-build/docker-entrypoint.sh")

# Rebuild
print("\n=== Rebuild ===")
result = run(
    "cd /tmp/moduforge-build && docker build -t moduforge:latest -f backend/Dockerfile . 2>&1",
    timeout=600
)
print("Build:", result[-500:] if len(result) > 500 else result)

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

# Verify binary fix
fix = run('docker exec moduforge strings /server | grep -c "WHERE name=" 2>/dev/null')
print("Binary fix count:", fix)

ssh.close()
print("\nDone")
