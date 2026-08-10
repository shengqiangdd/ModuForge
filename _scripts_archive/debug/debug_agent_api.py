#!/usr/bin/env python3
"""Debug the ModuForge Agent API - test sending a simple task"""
import sys, io, json, time, re
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
PROJECT_ID = "1785249992652501794-1864"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()

    # 1. Login
    print("=== 1. Login ===")
    out, err = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
    print(f"Login response: {out[:200]}")
    data = json.loads(out)
    token = data.get("token", "")
    print(f"Token OK: {len(token) > 10}")

    # 2. Check if the API endpoint exists first
    print("\n=== 2. Check API endpoints ===")
    out, err = run('curl -s http://localhost:8086/api/v1/agent/status')
    print(f"Agent status: {out[:300]}")

    # 3. Send a VERY simple task first
    print("\n=== 3. Send simple test task ===")
    # Use a temp file to avoid shell escaping issues
    task_json = json.dumps({
        "task": f"Read the file /app/data/storage/projects/{PROJECT_ID}/src/go/web/index.html and tell me how many lines it has.",
        "provider_id": "opencode-go",
        "model": "mimo-v2.5"
    })
    
    # Write task JSON to a temp file on the server
    run(f"echo '{task_json}' > /tmp/agent_task.json")
    
    # Send using the file
    out, err = run('curl -s -X POST http://localhost:8086/api/v1/agent/run -H "Authorization: Bearer $(cat /tmp/agent_token.txt)" -H "Content-Type: application/json" -d @/tmp/agent_task.json 2>&1', timeout=120)
    
    if not out:
        print(f"No response! stderr: {err[:500]}")
        # Try without the token file approach
        print("\nRetrying with inline token...")
        cmd = f'curl -s -X POST http://localhost:8086/api/v1/agent/run -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d @/tmp/agent_task.json 2>&1'
        out, err = run(cmd, timeout=120)
        print(f"Response length: {len(out)}")
        print(f"First 2000 chars:\n{out[:2000]}")
        if err:
            print(f"STDERR: {err[:500]}")

    # Parse SSE stream
    if out:
        print(f"\n=== SSE Events ===")
        event_count = 0
        for line in out.split('\n'):
            line = line.strip()
            if line.startswith('data: '):
                event_count += 1
                try:
                    data = json.loads(line[6:])
                    step = data.get('step', '?')
                    content = data.get('content', '')
                    error = data.get('error', '')
                    if content:
                        print(f"[{step}] {content[:150]}")
                    if error:
                        print(f"[ERROR] {error}")
                except json.JSONDecodeError:
                    print(f"[RAW] {line[:200]}")
        print(f"\nTotal events: {event_count}")

    client.close()

if __name__ == "__main__":
    main()
