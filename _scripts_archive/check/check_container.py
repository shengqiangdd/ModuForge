#!/usr/bin/env python3
"""Check ModuForge - use sqlite3 from host, check container tools"""
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

# Check container is running and what's inside
print("=== Container status ===")
out, _ = run('docker inspect moduforge --format="{{.State.Status}}"')
print(f"Status: {out.strip()}")

print("\n=== Container OS ===")
out, _ = run('docker exec moduforge cat /etc/os-release 2>/dev/null | head -5')
print(out)

print("\n=== Container ls /app/src/ ===")
out, _ = run('docker exec moduforge ls /app/src/ 2>/dev/null')
print(out)

print("\n=== Check if sqlite3 exists in container ===")
out, _ = run('docker exec moduforge which sqlite3 2>/dev/null; docker exec moduforge find / -name "sqlite3" 2>/dev/null | head -5')
print(out)

# Use host sqlite3 to query
print("\n=== Host sqlite3 query ===")
out, _ = run('sqlite3 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db ".tables" 2>&1')
print(f"Tables: {out}")

if "no such file" not in out.lower():
    out, _ = run('sqlite3 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db "SELECT id, name, provider_id, substr(base_url,1,50) FROM providers;" 2>&1')
    print(f"Providers:\n{out}")

    out, _ = run('sqlite3 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db "SELECT key, substr(value,1,100) FROM agent_settings;" 2>&1')
    print(f"Agent settings:\n{out}")

# Check the container's network and API key
print("\n=== Container process list ===")
out, _ = run('docker exec moduforge ps aux 2>/dev/null')
print(out[:1000])

# Check if there's a .env in the container
print("\n=== Container env ===")
out, _ = run('docker exec moduforge printenv 2>/dev/null | grep -iE "key|api|model|provider|base" | head -20')
print(out)

# Try to test API from container
print("\n=== Test API from container (using built-in Go tool) ===")
out, _ = run('docker exec moduforge wget -q -O- --timeout=5 http://localhost:8087/api/v1/health 2>&1 | head -5')
print(f"Health from inside container: {out}")

client.close()
