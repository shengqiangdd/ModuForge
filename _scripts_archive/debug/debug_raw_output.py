#!/usr/bin/env python3
"""Capture full raw SSE output for analysis"""
import sys, io, json, time
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=30):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

# Login
out, _ = run('curl -s -X POST http://localhost:8087/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(out).get("token", "")
PROJECT_ID = "1785249992652501794-1864"

# First check what's in the project
print("=== Project files ===")
out, _ = run(f'docker exec moduforge ls -la /data/projects/{PROJECT_ID}/src/')
print(out)

out, _ = run(f'docker exec moduforge ls -la /data/projects/{PROJECT_ID}/src/main.rs 2>/dev/null')
print(f"main.rs: {out}")

out, _ = run(f'docker exec moduforge ls -la /data/projects/{PROJECT_ID}/src/go/web/ 2>/dev/null')
print(f"web dir: {out}")

# Simple task - write to a file
print("\n=== TASK: Write test file ===")
task = json.dumps({
    "task": f"Use write_file to create the file /app/data/storage/projects/{PROJECT_ID}/test_output.txt with content 'Hello from Agent'. Do NOT read any files first. Just call write_file directly.",
    "provider_id": "opencode-zen",
    "model": "deepseek-v4-flash-free",
    "project_id": PROJECT_ID
})

cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task}' 2>&1'''

out, _ = run(cmd, timeout=120)

# Print ALL events
print("\n=== ALL SSE EVENTS ===")
event_count = 0
for line in out.split('\n'):
    line = line.strip()
    if not line.startswith('data: '):
        continue
    event_count += 1
    raw = line[6:]
    print(f"EVENT {event_count}: {raw[:300]}")

print(f"\nTotal events: {event_count}")

# Check if file was created
time.sleep(2)
out, _ = run(f'docker exec moduforge cat /data/projects/{PROJECT_ID}/test_output.txt 2>&1')
print(f"\ntest_output.txt content: {out}")

client.close()
