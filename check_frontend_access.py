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

print("=== Check if frontend is accessible ===")
# Test if the frontend is actually accessible
result = run("docker exec moduforge curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/")
print(f"Frontend HTTP status: {result}")

print("\n=== Check if there are any specific API calls ===")
# Check if there are any specific API calls that might be failing
result = run("docker exec moduforge curl -s http://localhost:8080/api/v1/notifications/unread-count -H 'Authorization: Bearer test' 2>/dev/null")
print(f"Notifications API: {result[:100]}...")

print("\n=== Check for any recent changes ===")
# Check if there are any recent changes to the frontend
result = run("docker exec moduforge ls -la /app/dist/assets/ | head -10")
print(result)

print("\n=== Check for any specific pages ===")
# Check if there are any specific pages that might be loading
result = run("docker exec moduforge find /app/dist -name '*.html' 2>/dev/null")
print(result if result else "No HTML files found")

print("\n=== Check for any JavaScript errors ===")
# This would require browser logs, but let's check if there are any obvious issues
result = run("docker exec moduforge cat /app/dist/assets/index-DLXJAqkQ.js 2>/dev/null | head -50")
print(result[:500] if result else "No JS content found")

ssh.close()
