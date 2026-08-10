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

print("=== Check WebSocket implementation ===")
# Look for WebSocket implementation in the frontend
result = run("docker exec moduforge grep -r 'WebSocket\\|ws://' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No WebSocket implementation found")

print("\n=== Check for reconnection logic ===")
# Look for reconnection logic
result = run("docker exec moduforge grep -r 'reconnect\\|retry\\|backoff' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No reconnection logic found")

print("\n=== Check for loading state management ===")
# Look for loading state management
result = run("docker exec moduforge grep -r 'isLoading\\|setLoading\\|loading =' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No loading state management found")

print("\n=== Check for specific error handling ===")
# Look for specific error handling
result = run("docker exec moduforge grep -r 'onerror\\|onclose\\|onopen' /app/dist/assets/*.js 2>/dev/null | head -10")
print(result if result else "No error handling found")

ssh.close()
