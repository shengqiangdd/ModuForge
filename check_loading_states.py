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

print("=== Check for specific loading states ===")
# Search for loading/processing states in the JS files
result = run("docker exec moduforge grep -r 'isLoading\\|loading\\|spinner\\|processing' /app/dist/assets/*.js 2>/dev/null | head -20")
print(result if result else "No loading states found")

print("\n=== Check for specific features ===")
# Check if there are any specific features that might be loading
result = run("docker exec moduforge grep -r 'dashboard\\|projects\\|builds' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No specific features found")

print("\n=== Check for error handling ===")
# Check if there are any error handling issues
result = run("docker exec moduforge grep -r 'error\\|catch\\|reject' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No error handling found")

print("\n=== Check for WebSocket handling ===")
# Check WebSocket connection handling
result = run("docker exec moduforge grep -r 'websocket\\|WebSocket\\|ws://' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No WebSocket handling found")

print("\n=== Check server logs for specific errors ===")
logs = run("docker logs --tail 200 moduforge 2>&1")
# Look for specific error patterns
for line in logs.split('\n'):
    if any(keyword in line.lower() for keyword in ['error', 'fail', 'timeout', 'reject']):
        print(line)

ssh.close()
