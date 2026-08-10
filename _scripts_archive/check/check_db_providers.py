#!/usr/bin/env python3
"""Check ModuForge database for provider config"""
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

# Check the SQLite database for providers
print("=== Database providers ===")
out, _ = run('docker exec moduforge sqlite3 /app/data/moduforge.db "SELECT id, name, base_url, provider_id FROM providers WHERE enabled=1;"')
print(out)

print("\n=== Database agent_settings ===")
out, _ = run('docker exec moduforge sqlite3 /app/data/moduforge.db "SELECT key, value FROM agent_settings;"')
print(out)

print("\n=== Container env vars (filtered) ===")
out, _ = run('docker exec moduforge env | grep -i "api\\|key\\|provider\\|model\\|base_url" | head -20')
print(out)

# Check if the container can reach the LLM API
print("\n=== Test LLM connectivity from container ===")
out, _ = run('docker exec moduforge curl -s --connect-timeout 5 https://api.opencode.ai/ 2>&1 | head -5')
print(f"opencode.ai: {out[:200]}")

# Check what the resolveProvider does
print("\n=== Check agent runner source ===")
out, _ = run('docker exec moduforge cat /app/src/agent/runner.go 2>/dev/null | head -100')
print(out[:3000])

client.close()
