#!/usr/bin/env python3
"""Fix container volume mount and re-test Agent"""
import sys, io, json, time
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=60):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

# Step 1: Stop current container
print("=== Step 1: Stop container ===")
out, _ = run("docker stop moduforge")
print(f"Stop: {out.strip()}")

# Step 2: Remove container (keep the image)
print("\n=== Step 2: Remove container ===")
out, _ = run("docker rm moduforge")
print(f"Remove: {out.strip()}")

# Step 3: Recreate with volume mounts
print("\n=== Step 3: Recreate with volume mounts ===")
cmd = '''docker run -d \
  --name moduforge \
  -p 127.0.0.1:8087:8080 \
  -v /vol1/docker/volumes/moduforge_moduforge_data/_data:/data \
  -v /vol1/docker/volumes/moduforge_moduforge_uploads/_data:/app/uploads \
  --restart unless-stopped \
  moduforge:latest'''
out, _ = run(cmd)
print(f"Run: {out.strip()}")

# Step 4: Wait and verify
print("\n=== Step 4: Wait for container to start ===")
time.sleep(5)
out, _ = run("docker ps --filter name=moduforge --format '{{.Status}}'")
print(f"Status: {out.strip()}")

# Step 5: Verify data is accessible
print("\n=== Step 5: Verify data ===")
out, _ = run("docker exec moduforge ls /data/projects/")
print(f"Projects: {out.strip()}")

out, _ = run("docker exec moduforge ls /data/projects/1785249992652501794-1864/src/rust/src/main.rs 2>/dev/null")
print(f"main.rs: {out.strip()}")

# Step 6: Test Agent API
print("\n=== Step 6: Test Agent ===")
out, _ = run('curl -s -X POST http://localhost:8087/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
data = json.loads(out) if out.startswith('{') else {}
token = data.get("token", "")
print(f"Token: {'OK' if token else 'FAILED'}")

if token:
    task = json.dumps({
        "task": "Read /app/data/projects/1785249992652501794-1864/src/rust/src/main.rs and tell me how many lines it has.",
        "provider_id": "opencode-zen",
        "model": "deepseek-v4-flash-free",
        "project_id": "1785249992652501794-1864"
    })
    cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
      -H "Authorization: Bearer {token}" \
      -H "Content-Type: application/json" \
      -d '{task}' 2>&1'''
    out, _ = run(cmd, timeout=120)
    
    events = []
    for line in out.split('\n'):
        line = line.strip()
        if line.startswith('data: '):
            try:
                data = json.loads(line[6:])
                events.append(data)
            except:
                pass
    
    print(f"\nEvents: {len(events)}")
    for e in events:
        stype = e.get('type', '')
        step = e.get('step', '')
        content = e.get('content', '')[:150]
        error = e.get('error', '')
        if error:
            print(f"  [ERROR] {error}")
        elif step == 'tool_start':
            print(f"  [TOOL] {e.get('tool', '?')}")
        elif step == 'tool_end':
            print(f"  [TOOL_END] len={len(e.get('result', ''))}")
        elif step == 'answer':
            print(f"  [ANSWER] {content}")
        elif step in ('think', 'task_plan', 'task_progress'):
            print(f"  [{step}] {content}")

client.close()
