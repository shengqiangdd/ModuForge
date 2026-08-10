#!/usr/bin/env python3
"""Check .env and network from ModuForge"""
import sys, io, json
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=30):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

# Check .env file
print("=== .env file ===")
out, _ = run('cat /vol1/docker/volumes/moduforge_moduforge_data/.env')
print(out)

# Check docker-compose
print("\n=== docker-compose.yml ===")
out, _ = run('cat /vol1/docker/volumes/moduforge_moduforge_data/docker-compose.yml 2>/dev/null | head -80')
print(out)

# Check container's /app directory
print("\n=== Container /app ===")
out, _ = run('docker exec moduforge ls -la /app/ 2>/dev/null')
print(out)

# Check if data directory is mounted
print("\n=== Container mount points ===")
out, _ = run('docker inspect moduforge --format="{{range .Mounts}}{{.Source}} -> {{.Destination}}{{println}}{{end}}"')
print(out)

# Check the container's network
print("\n=== Container network test ===")
out, _ = run('docker exec moduforge wget -q -O /dev/null --timeout=5 https://api.deepseek.com/ 2>&1; echo "exit=$?"')
print(f"deepseek: {out}")

out, _ = run('docker exec moduforge wget -q -O /dev/null --timeout=5 https://api.opencode.ai/ 2>&1; echo "exit=$?"')
print(f"opencode: {out}")

# Check the DB location in container
print("\n=== DB in container ===")
out, _ = run('docker exec moduforge ls -la /app/data/ 2>/dev/null')
print(out)

out, _ = run('docker exec moduforge ls -la /app/data/*.db 2>/dev/null')
print(out)

# Try to read DB through the container
print("\n=== Read DB via cat + hexdump (last resort) ===")
out, _ = run('docker exec moduforge cat /app/data/moduforge.db 2>/dev/null | strings | grep -i "provider\\|opencode\\|rhythm\\|base_url" | head -30')
print(out)

client.close()
