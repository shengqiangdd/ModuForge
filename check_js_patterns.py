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

print("=== Check for specific loading issues ===")
# Search for loading/processing patterns in the main JS file
result = run("docker exec moduforge cat /app/dist/assets/index-DLXJAqkQ.js 2>/dev/null")
if result:
    # Look for loading/processing patterns
    lines = result.split('\n')
    for i, line in enumerate(lines):
        if any(keyword in line.lower() for keyword in ['loading', 'spinner', 'processing', 'isLoading']):
            print(f"Line {i}: {line[:200]}")
else:
    print("No JS content found")

print("\n=== Check for specific error patterns ===")
# Look for error patterns
result = run("docker exec moduforge cat /app/dist/assets/index-DLXJAqkQ.js 2>/dev/null")
if result:
    lines = result.split('\n')
    for i, line in enumerate(lines):
        if any(keyword in line.lower() for keyword in ['error', 'catch', 'reject', 'timeout']):
            print(f"Line {i}: {line[:200]}")
else:
    print("No error patterns found")

print("\n=== Check for WebSocket patterns ===")
# Look for WebSocket patterns
result = run("docker exec moduforge cat /app/dist/assets/index-DLXJAqkQ.js 2>/dev/null")
if result:
    lines = result.split('\n')
    for i, line in enumerate(lines):
        if any(keyword in line.lower() for keyword in ['websocket', 'ws://', 'wss://']):
            print(f"Line {i}: {line[:200]}")
else:
    print("No WebSocket patterns found")

ssh.close()
