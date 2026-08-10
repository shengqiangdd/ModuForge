#!/usr/bin/env python3
"""Debug raw API response"""
import sys, io, json
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=60):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

# Login
out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(out).get("token", "")
print(f"Token: {token[:80]}...")

# Check available routes
print("\n=== Check API routes ===")
out, _ = run('curl -s http://localhost:8086/api/v1/agent/tasks')
print(f"agent/tasks: {out[:500]}")

# Send task with verbose output
print("\n=== Send task (verbose) ===")
task_json = json.dumps({"task":"say hello"})
cmd = f'''curl -v -X POST http://localhost:8086/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task_json}' 2>&1'''
out, err = run(cmd, timeout=120)
print(f"FULL OUTPUT:\n{out[:3000]}")

# Try without SSE
print("\n=== Try /api/v1/agent/run-sync ===")
cmd2 = f'''curl -s -X POST http://localhost:8086/api/v1/agent/run-sync \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task_json}' 2>&1'''
out2, err2 = run(cmd2, timeout=120)
print(f"run-sync: {out2[:2000]}")

# List all API routes
print("\n=== List all routes ===")
out3, _ = run('curl -s http://localhost:8086/api/v1/ 2>&1')
print(f"Routes: {out3[:1000]}")

client.close()
