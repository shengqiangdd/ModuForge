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

print("=== Check container dist directory ===")
print(run("docker exec moduforge ls -la /app/dist/"))

print("\n=== Check dist assets ===")
print(run("docker exec moduforge ls -la /app/dist/assets/ | head -20"))

print("\n=== Check index.html content ===")
print(run("docker exec moduforge cat /app/dist/index.html"))

print("\n=== Check if there are any JS errors in browser ===")
# This would require browser logs, but let's check server logs for errors
logs = run("docker logs --tail 100 moduforge 2>&1")
error_lines = [line for line in logs.split('\n') if 'error' in line.lower() or 'fail' in line.lower()]
if error_lines:
    print("Recent errors:")
    for line in error_lines[-10:]:
        print(line)
else:
    print("No errors found in logs")

print("\n=== Check WebSocket connection status ===")
ws_lines = [line for line in logs.split('\n') if 'ws' in line.lower() or 'websocket' in line.lower()]
if ws_lines:
    print("WebSocket activity:")
    for line in ws_lines[-10:]:
        print(line)

ssh.close()
