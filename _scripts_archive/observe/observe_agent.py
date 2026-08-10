#!/usr/bin/env python3
"""Send real coding tasks and observe Agent behavior for weakness analysis"""
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

# ============================================================
# TASK 1: Simple - read a file and add a comment
# ============================================================
print("=" * 60)
print("TASK 1: Simple edit - add a comment to main.rs")
print("=" * 60)

task1 = json.dumps({
    "task": f"Read the file /app/data/storage/projects/{PROJECT_ID}/src/main.rs, then use write_file to add a comment '// Agent test: hello from Agent' at the very first line of the file. You MUST use write_file to actually modify the file.",
    "provider_id": "opencode-zen",
    "model": "deepseek-v4-flash-free",
    "project_id": PROJECT_ID
})

cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task1}' 2>&1'''

out, _ = run(cmd, timeout=120)

# Parse events
steps = []
tool_calls = []
errors = []
for line in out.split('\n'):
    line = line.strip()
    if not line.startswith('data: '):
        continue
    try:
        data = json.loads(line[6:])
        step = data.get('step', '?')
        stype = data.get('type', '')
        content = data.get('content', '')
        error = data.get('error', '')
        
        if stype == 'error':
            errors.append(content or error)
            print(f"  [ERROR] {content or error}")
        elif step == 'tool_start':
            tool_name = data.get('tool', '?')
            tool_calls.append(tool_name)
            print(f"  [TOOL] {tool_name}")
        elif step == 'tool_end':
            result_len = len(data.get('result', ''))
            print(f"  [TOOL_END] result_len={result_len}")
        elif step == 'answer':
            print(f"  [ANSWER] {content[:200]}")
        elif step in ('think', 'task_plan', 'task_progress'):
            pass  # skip noise
    except:
        pass

print(f"\nSummary: tool_calls={tool_calls}, errors={errors}")

# Check if file was actually modified
time.sleep(2)
out, _ = run(f'docker exec moduforge head -3 /data/projects/{PROJECT_ID}/src/main.rs')
print(f"File first 3 lines after task:\n{out}")

# ============================================================
# TASK 2: Medium - read index.html and add a div
# ============================================================
print("\n" + "=" * 60)
print("TASK 2: Medium edit - add a div to index.html")
print("=" * 60)

task2 = json.dumps({
    "task": f"Read /app/data/storage/projects/{PROJECT_ID}/src/go/web/index.html. Then use write_file to add the following HTML right before the closing </body> tag:\n<div id='agent-test' style='background:green;color:white;padding:10px'>Agent Test Panel</div>\nYou MUST call write_file to modify the file.",
    "provider_id": "opencode-zen",
    "model": "deepseek-v4-flash-free",
    "project_id": PROJECT_ID
})

cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task2}' 2>&1'''

out, _ = run(cmd, timeout=120)

tool_calls = []
errors = []
for line in out.split('\n'):
    line = line.strip()
    if not line.startswith('data: '):
        continue
    try:
        data = json.loads(line[6:])
        step = data.get('step', '?')
        stype = data.get('type', '')
        content = data.get('content', '')
        error = data.get('error', '')
        
        if stype == 'error':
            errors.append(content or error)
            print(f"  [ERROR] {content or error}")
        elif step == 'tool_start':
            tool_name = data.get('tool', '?')
            tool_calls.append(tool_name)
            print(f"  [TOOL] {tool_name}")
        elif step == 'tool_end':
            result_len = len(data.get('result', ''))
            print(f"  [TOOL_END] result_len={result_len}")
        elif step == 'answer':
            print(f"  [ANSWER] {content[:200]}")
    except:
        pass

print(f"\nSummary: tool_calls={tool_calls}, errors={errors}")

time.sleep(2)
out, _ = run(f'docker exec moduforge grep -c "agent-test" /data/projects/{PROJECT_ID}/src/go/web/index.html')
print(f"agent-test div count in file: {out.strip()}")

# ============================================================
# TASK 3: Hard - understand codebase and make multi-file change
# ============================================================
print("\n" + "=" * 60)
print("TASK 3: Hard - understand structure, create new route")
print("=" * 60)

task3 = json.dumps({
    "task": f"Project at /app/data/storage/projects/{PROJECT_ID}.\nStep 1: Read src/main.rs to understand the structure.\nStep 2: Read src/api/routes.rs to understand the route patterns.\nStep 3: Add a new GET route /api/health-check that returns {{\"status\": \"ok\"}} by modifying routes.rs.\nYou MUST use write_file to modify routes.rs.",
    "provider_id": "opencode-zen",
    "model": "deepseek-v4-flash-free",
    "project_id": PROJECT_ID
})

cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task3}' 2>&1'''

out, _ = run(cmd, timeout=180)

tool_calls = []
errors = []
answers = []
for line in out.split('\n'):
    line = line.strip()
    if not line.startswith('data: '):
        continue
    try:
        data = json.loads(line[6:])
        step = data.get('step', '?')
        stype = data.get('type', '')
        content = data.get('content', '')
        error = data.get('error', '')
        
        if stype == 'error':
            errors.append(content or error)
            print(f"  [ERROR] {content or error}")
        elif step == 'tool_start':
            tool_name = data.get('tool', '?')
            tool_calls.append(tool_name)
            print(f"  [TOOL] {tool_name}")
        elif step == 'tool_end':
            result_len = len(data.get('result', ''))
            print(f"  [TOOL_END] result_len={result_len}")
        elif step == 'answer':
            answers.append(content[:300])
            print(f"  [ANSWER] {content[:300]}")
    except:
        pass

print(f"\nSummary: tool_calls={tool_calls}, errors={errors}, answers={len(answers)}")

client.close()

print("\n" + "=" * 60)
print("ANALYSIS COMPLETE")
print("=" * 60)
