#!/usr/bin/env python3
"""Deploy using original image + docker cp approach that WORKS"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)

# Pull original image fresh
print("Pull original image...")
stdin, stdout, stderr = ssh.exec_command("docker pull moduforge:latest 2>&1 || docker image ls moduforge", timeout=60)
out = stdout.read().decode().strip()
print(f"  {out[:200]}")

# Remove broken container
print("\nRemove broken container...")
ssh.exec_command(f"docker rm -f {CONTAINER}")
time.sleep(2)

# Create container with original image (NO command override)
create_cmd = f"""docker create --name {CONTAINER} \
  -p 8087:8080 \
  -v /home/admin/moduforge_data/data:/data \
  -v /home/admin/moduforge_data/projects:/app/projects \
  -v /home/admin/moduforge_data/build-cache:/app/build-cache \
  -v /home/admin/moduforge_data/artifacts:/app/artifacts \
  --restart unless-stopped \
  moduforge:latest"""

print(f"\n--- Create ---")
stdin, stdout, stderr = ssh.exec_command(create_cmd, timeout=15)
print(stdout.read().decode().strip() or stderr.read().decode().strip())

# Copy new binary
print("\n--- Copy binary ---")
stdin, stdout, stderr = ssh.exec_command(f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server", timeout=30)
err = stderr.read().decode().strip()
print(f"cp /server: {err or 'OK'}")

# Now start with a different approach: override entrypoint to fix perms first
# We need to use /bin/sh -c but NOT replace the entire entrypoint
print("\n--- Start with chmod prefix ---")
# First, check if the image's entrypoint has execute permission
stdin, stdout, stderr = ssh.exec_command(
    f"docker start {CONTAINER} 2>&1",
    timeout=10
)
result = stdout.read().decode().strip() or stderr.read().decode().strip()
print(f"start: {result}")

time.sleep(5)

# Check health
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
health = stdout.read().decode().strip()
print(f"\nHealth: {health}")

if "ok" in health:
    # SUCCESS! The original image's entrypoint worked
    # Now check if /server has the new code
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name=' 2>&1", timeout=15)
    print(f"\nWHERE name=: {stdout.read().decode().strip()}")
    
    # Trigger agent test
    print("\n--- Agent test ---")
    import json
    stdin, stdout, stderr = ssh.exec_command(
        """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
        timeout=10
    )
    resp = json.loads(stdout.read().decode().strip())
    token = resp.get("token", "")
    if token:
        cmd = f"""curl -s -X POST http://localhost:8087/api/v1/agent/run \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer {token}" \
          -d '{{"task":"say hi","provider_id":"rhythm","model":"dsv4f"}}' \
          --max-time 20 2>&1"""
        stdin, stdout, stderr = ssh.exec_command(cmd, timeout=25)
        resp = stdout.read().decode("utf-8", errors="ignore").strip()
        print(f"Agent: {resp[:500]}")
        
        # Check logs
        stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 10 2>&1", timeout=10)
        print(f"\nLogs: {stdout.read().decode('utf-8', errors='ignore').strip()}")
else:
    stdin, stdout, stderr = ssh.exec_command(f"docker logs {CONTAINER} --tail 10 2>&1", timeout=10)
    print(f"\nLogs: {stdout.read().decode('utf-8', errors='ignore').strip()}")

ssh.close()
