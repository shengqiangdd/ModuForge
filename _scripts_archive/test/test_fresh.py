#!/usr/bin/env python3
"""Fresh test: trigger agent and check NEW logs only"""
import paramiko, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Clear old logs
ssh.exec_command("docker truncate /tmp/moduforge-logs.txt 2>/dev/null; docker logs moduforge 2>&1 | tail -1 > /dev/null")
print("Connected")

# Login
stdin, stdout, stderr = ssh.exec_command(
    """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"csq","password":"csq0216"}'""",
    timeout=10
)
resp = json.loads(stdout.read().decode().strip())
token = resp.get("token", "")
print(f"Login: {'OK' if token else 'FAILED'}")

if not token:
    import sys; sys.exit(1)

# Record current log position
stdin, stdout, stderr = ssh.exec_command("docker logs moduforge 2>&1 | wc -l", timeout=10)
log_lines_before = int(stdout.read().decode().strip())
print(f"Log lines before test: {log_lines_before}")

# Trigger agent
print("\n--- Triggering agent call ---")
cmd = f"""curl -s -X POST http://localhost:8087/api/v1/agent/run \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {token}" \
  -d '{{"task":"say hi","provider_id":"rhythm","model":"dsv4f"}}' \
  --max-time 20 2>&1"""

stdin, stdout, stderr = ssh.exec_command(cmd, timeout=25)
resp = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"Agent response ({len(resp)} bytes): {resp[:500]}")

# Wait a bit
time.sleep(2)

# Get logs AFTER the test
stdin, stdout, stderr = ssh.exec_command(f"docker logs moduforge 2>&1 | tail -20", timeout=10)
new_logs = stdout.read().decode("utf-8", errors="ignore").strip()
print(f"\n--- Latest container logs ---")
print(new_logs)

ssh.close()
