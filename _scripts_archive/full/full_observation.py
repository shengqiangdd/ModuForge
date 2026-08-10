#!/usr/bin/env python3
"""Full Agent behavior observation with 3 difficulty levels"""
import sys, io, json, time
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=120):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

out, _ = run('curl -s -X POST http://localhost:8087/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(out).get("token", "")
PID = "1785249992652501794-1864"

def send_task(task_desc, task_name):
    print(f"\n{'='*60}")
    print(f"TASK: {task_name}")
    print(f"{'='*60}")
    
    task = json.dumps({
        "task": task_desc,
        "provider_id": "opencode-zen",
        "model": "deepseek-v4-flash-free",
        "project_id": PID
    })
    cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
      -H "Authorization: Bearer {token}" \
      -H "Content-Type: application/json" \
      -d '{task}' 2>&1'''
    out, _ = run(cmd, timeout=180)
    
    tool_calls = []
    tool_results = {}
    errors = []
    answers = []
    reasoning_tokens = 0
    stream_tokens = 0
    
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
                errors.append(error or content)
            elif stype == 'reasoning':
                reasoning_tokens += 1
            elif stype == 'stream_delta':
                stream_tokens += 1
            elif step == 'tool_start':
                tool_name = data.get('tool', '?')
                tool_calls.append(tool_name)
                args = data.get('args', {})
                if tool_name == 'read_file':
                    path = args.get('file_path', args.get('path', '?'))
                    print(f"  📖 READ: {path}")
                elif tool_name == 'write_file':
                    path = args.get('file_path', args.get('path', '?'))
                    print(f"  ✏️  WRITE: {path}")
                elif tool_name == 'execute_shell_command':
                    cmd_text = args.get('command', '?')[:100]
                    print(f"  🔧 SHELL: {cmd_text}")
                else:
                    print(f"  🔧 {tool_name}: {str(args)[:100]}")
            elif step == 'tool_end':
                result = data.get('result', '')
                tool_results[tool_calls[-1] if tool_calls else 'unknown'] = len(result)
            elif step == 'answer':
                answers.append(content[:300])
                print(f"  💬 ANSWER: {content[:200]}")
        except:
            pass
    
    print(f"\n  📊 Stats: tools={tool_calls}, reasoning={reasoning_tokens}, output={stream_tokens}")
    if errors:
        print(f"  ❌ Errors: {errors}")
    return {
        "tools": tool_calls,
        "errors": errors,
        "answers": answers,
        "reasoning": reasoning_tokens,
        "output": stream_tokens
    }

# ============================================================
# TASK 1: Simple - write a single file
# ============================================================
r1 = send_task(
    f"Use write_file to create /app/data/projects/{PID}/test_simple.txt with the exact content: 'Hello from Agent - test successful'. Do NOT read any files first. Just call write_file directly.",
    "Simple: write a file"
)

time.sleep(2)
# Verify
out, _ = run(f'docker exec moduforge cat /data/projects/{PID}/test_simple.txt 2>&1')
print(f"\n  ✅ File content: {out.strip()}")
if "Hello from Agent" in out:
    print("  ✅ SUCCESS: File was actually written!")
else:
    print("  ❌ FAIL: File was NOT written!")

# ============================================================
# TASK 2: Medium - read then modify
# ============================================================
r2 = send_task(
    f"Read /app/data/projects/{PID}/module.prop to understand its structure. Then use write_file to add a new line 'version=2.0' at the end of the file.",
    "Medium: read then modify"
)

time.sleep(2)
out, _ = run(f'docker exec moduforge cat /data/projects/{PID}/module.prop 2>&1')
print(f"\n  📄 module.prop after task:\n{out}")
if "version=2.0" in out:
    print("  ✅ SUCCESS: File was modified!")
else:
    print("  ❌ FAIL: File was NOT modified!")

# ============================================================
# TASK 3: Hard - multi-file understanding
# ============================================================
r3 = send_task(
    f"Project at /app/data/projects/{PID}.\nStep 1: Read src/rust/src/main.rs to understand the structure.\nStep 2: Read src/rust/src/linucb.rs to understand the LinUCB implementation.\nStep 3: Based on your understanding, add a new public function 'get_stats() -> String' to linucb.rs that returns a summary string of the current LinUCB state (num_arms, dim, alpha).\nYou MUST use write_file to modify linucb.rs.",
    "Hard: multi-file understanding + code modification"
)

time.sleep(2)
out, _ = run(f'docker exec moduforge grep -n "get_stats" /data/projects/{PID}/src/rust/src/linucb.rs 2>&1')
print(f"\n  📄 grep get_stats in linucb.rs: {out.strip()}")
if "get_stats" in out:
    print("  ✅ SUCCESS: Function was added!")
else:
    print("  ❌ FAIL: Function was NOT added!")

# Summary
print(f"\n{'='*60}")
print("SUMMARY")
print(f"{'='*60}")
print(f"Task 1 (Simple):   tools={r1['tools']}, errors={r1['errors']}")
print(f"Task 2 (Medium):   tools={r2['tools']}, errors={r2['errors']}")
print(f"Task 3 (Hard):     tools={r3['tools']}, errors={r3['errors']}")

client.close()
