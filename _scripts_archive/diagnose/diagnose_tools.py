#!/usr/bin/env python3
"""Diagnose why Agent doesn't execute tools"""
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

# Check agent settings
print("=== Agent Settings ===")
out, _ = run(f'curl -s http://localhost:8087/api/v1/settings/agent -H "Authorization: Bearer {token}"')
print(out)

# Check agent skills/tools config
print("\n=== Agent Skills ===")
out, _ = run(f'curl -s http://localhost:8087/api/v1/settings/skills -H "Authorization: Bearer {token}"')
print(out[:2000])

# Check if tools are enabled
print("\n=== Check tool_list or tool_config ===")
for path in ["/api/v1/agent/tools", "/api/v1/tools", "/api/v1/settings/tools"]:
    out, _ = run(f'curl -s http://localhost:8087{path} -H "Authorization: Bearer {token}"')
    if "Not Found" not in out:
        print(f"{path}: {out[:1000]}")

# Check the system prompt - maybe tools aren't injected
print("\n=== Check agent system prompt ===")
for path in ["/api/v1/agent/prompt", "/api/v1/settings/prompt", "/api/v1/settings/system-prompt"]:
    out, _ = run(f'curl -s http://localhost:8087{path} -H "Authorization: Bearer {token}"')
    if "Not Found" not in out:
        print(f"{path}: {out[:2000]}")

# Try with a different model - maybe deepseek-v4-flash-free can't use tools
print("\n=== Try with mimo-v2.5-free ===")
task = json.dumps({
    "task": f"Call write_file to create /app/data/projects/{PID}/test_mimo.txt with content 'mimo test'. Do NOT read first.",
    "provider_id": "opencode-zen",
    "model": "mimo-v2.5-free",
    "project_id": PID
})
cmd = f'''curl -s -N -X POST http://localhost:8087/api/v1/agent/run \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{task}' 2>&1'''
out, _ = run(cmd, timeout=120)

tool_calls = []
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
        if error:
            print(f"  [ERROR] {error}")
        elif step == 'tool_start':
            tool_calls.append(data.get('tool', '?'))
            print(f"  [TOOL] {data.get('tool', '?')}: {str(data.get('args', {}))[:150]}")
        elif step == 'answer':
            print(f"  [ANSWER] {content[:200]}")
    except:
        pass

print(f"  Tool calls: {tool_calls}")

# Check if the container has the tools directory
print("\n=== Check tools in container ===")
out, _ = run('docker exec moduforge ls /app/tools/ 2>/dev/null || echo "no /app/tools"')
print(out)

# Check the dist for agent-related files
print("\n=== Agent-related files in dist ===")
out, _ = run('docker exec moduforge find /app/dist -name "*agent*" -o -name "*tool*" -o -name "*skill*" 2>/dev/null')
print(out)

# Check if there's a tools config in the DB
print("\n=== DB strings for tool/agent ===")
out, _ = run('docker exec moduforge cat /data/moduforge.db 2>/dev/null | strings | grep -iE "tool|skill|agent_prompt|system_prompt" | head -20')
print(out)

client.close()
