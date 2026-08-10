#!/usr/bin/env python3
import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace').strip()

print("=== Check if there are any specific API calls that might be hanging ===")
# Check if there are any specific API calls that might be hanging
result = run("docker exec moduforge curl -s -m 5 http://localhost:8080/api/v1/projects -H 'Authorization: Bearer test' 2>/dev/null")
print(f"Projects API: {result[:100]}...")

print("\n=== Check for any specific error messages ===")
# Check for any specific error messages in the logs
logs = run("docker logs --tail 100 moduforge 2>&1")
error_messages = []
for line in logs.split('\n'):
    if 'error' in line.lower() or 'fail' in line.lower() or 'timeout' in line.lower():
        error_messages.append(line)

if error_messages:
    print("Recent error messages:")
    for msg in error_messages[-10:]:
        print(msg)
else:
    print("No error messages found")

print("\n=== Check for any specific WebSocket issues ===")
# Check for any specific WebSocket issues
ws_issues = []
for line in logs.split('\n'):
    if 'ws' in line.lower() or 'websocket' in line.lower():
        ws_issues.append(line)

if ws_issues:
    print("WebSocket activity:")
    for issue in ws_issues[-10:]:
        print(issue)
else:
    print("No WebSocket issues found")

print("\n=== Check for any specific loading indicators ===")
# Check if there are any specific loading indicators in the frontend
result = run("docker exec moduforge grep -r 'loading\\|spinner\\|processing' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No loading indicators found")

ssh.close()
