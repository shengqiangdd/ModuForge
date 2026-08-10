# -*- coding: utf-8 -*-
"""Extract entrypoint from original image, then rebuild."""
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=60):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    return o.read().decode().strip() or e.read().decode().strip()

# Step 1: Extract entrypoint from the original image (not our patched one)
print("=== Extract entrypoint ===")
# Use the original image tag (before our patches)
# First check what images we have
print("Images:", run("docker images moduforge --format '{{.Repository}}:{{.Tag}} {{.ID}}'"))

# Create temp container from original image to extract entrypoint
run("docker rm -f temp-extract 2>/dev/null")
run("docker create --name temp-extract moduforge:latest 2>/dev/null || docker create --name temp-extract moduforge:e0d19ba6afa8 2>/dev/null")
run("docker cp temp-extract:/docker-entrypoint.sh /tmp/docker-entrypoint-original.sh")
run("docker rm -f temp-extract")

# Show content
print("Entrypoint content:")
print(run("cat /tmp/docker-entrypoint-original.sh"))

# Copy to build context
run("cp /tmp/docker-entrypoint-original.sh /tmp/moduforge-build/backend/docker-entrypoint.sh")
run("chmod +x /tmp/moduforge-build/backend/docker-entrypoint.sh")

# Also check if frontend dist exists in build context
print("\nFrontend dist:", run("ls /tmp/moduforge-build/frontend/dist/ 2>/dev/null | head -5"))

# Step 2: Rebuild
print("\n=== Rebuild ===")
result = run(
    "cd /tmp/moduforge-build && docker build -t moduforge:latest -f backend/Dockerfile . 2>&1",
    timeout=600
)
print("Build:", result[-300:] if len(result) > 300 else result)

# Check image
print("\nImage:", run("docker images moduforge:latest --format '{{.ID}} {{.Size}}'"))

# Step 3: Deploy
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

ssh.close()
print("\nDone")
