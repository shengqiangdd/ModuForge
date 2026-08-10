#!/usr/bin/env python3
"""Read .env and DB from inside container"""
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

# Read .env
print("=== /data/.env ===")
out, _ = run('docker exec moduforge cat /data/.env')
print(out)

# Check if there's sqlite3 or python in the container
print("\n=== Available tools in container ===")
out, _ = run('docker exec moduforge ls /usr/bin/ /usr/local/bin/ 2>/dev/null')
print(out)

# The DB is SQLite with WAL. Let me try to extract data via strings
print("\n=== Provider data from DB (strings) ===")
out, _ = run('docker exec moduforge cat /data/moduforge.db 2>/dev/null | strings | grep -E "^[a-f0-9-]{36}$" | head -5')
print(f"UUIDs: {out}")

# Better approach: use the API to check providers
print("\n=== Check all API endpoints ===")
out, _ = run('curl -s http://localhost:8087/api/v1/health 2>&1')
print(f"Health (8087): {out[:200]}")

# Try to list providers through the API
out, _ = run('curl -s http://localhost:8087/api/v1/agent/providers 2>&1')
print(f"Agent providers: {out[:500]}")

out, _ = run('curl -s http://localhost:8087/api/v1/models 2>&1')
print(f"Models: {out[:500]}")

# Try with auth
out, _ = run('curl -s -X POST http://localhost:8087/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
data = json.loads(out) if out.startswith('{') else {}
token = data.get("token", "")
print(f"\nToken: {token[:50]}...")

if token:
    out, _ = run(f'curl -s http://localhost:8087/api/v1/settings/providers -H "Authorization: Bearer {token}"')
    print(f"\nSettings providers: {out[:2000]}")
    
    out, _ = run(f'curl -s http://localhost:8087/api/v1/settings/agent -H "Authorization: Bearer {token}"')
    print(f"\nSettings agent: {out[:2000]}")

    # List all routes by trying common paths
    for path in ["/api/v1/settings", "/api/v1/settings/llm", "/api/v1/settings/models", 
                 "/api/v1/llm/providers", "/api/v1/providers", "/api/v1/agent/config"]:
        out, _ = run(f'curl -s http://localhost:8087{path} -H "Authorization: Bearer {token}"')
        if "Not Found" not in out:
            print(f"\n{path}: {out[:500]}")

client.close()
