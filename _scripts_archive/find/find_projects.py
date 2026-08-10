#!/usr/bin/env python3
"""Find the actual project structure"""
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

# Find projects
print("=== /data/projects/ ===")
out, _ = run('docker exec moduforge ls -la /data/projects/')
print(out)

# List all project dirs
print("\n=== All project dirs ===")
out, _ = run('docker exec moduforge find /data/projects -maxdepth 2 -type d 2>/dev/null')
print(out[:2000])

# Check the actual project ID - maybe it's different
print("\n=== Check DB for project IDs ===")
out, _ = run('docker exec moduforge cat /data/moduforge.db 2>/dev/null | strings | grep -E "[0-9]{13}-[0-9]+" | head -10')
print(out)

# Try to access projects through API
out, _ = run('curl -s -X POST http://localhost:8087/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(out).get("token", "")

print("\n=== List projects via API ===")
out, _ = run(f'curl -s http://localhost:8087/api/v1/projects -H "Authorization: Bearer {token}"')
print(out[:2000])

# Check if the server sees the projects
print("\n=== Check server config ===")
out, _ = run('docker exec moduforge cat /data/.env')
print(out)

# Check storage directory
print("\n=== /data/storage/ ===")
out, _ = run('docker exec moduforge ls -la /data/storage/ 2>/dev/null')
print(out)

# Check /data/storage/projects
print("\n=== /data/storage/projects/ ===")
out, _ = run('docker exec moduforge ls -la /data/storage/projects/ 2>/dev/null')
print(out)

client.close()
