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

print("=== Check frontend source code ===")
# Look for loading states in the frontend
result = run("grep -r 'loading\\|spinner\\|processing\\|isLoading' /app/dist/assets/*.js 2>/dev/null | head -20")
print(result if result else "No loading state found in dist")

print("\n=== Check for specific pages ===")
# Check if there's a dashboard or main page that might be loading
print(run("ls -la /app/dist/assets/ | head -20"))

print("\n=== Check index.html for clues ===")
print(run("cat /app/dist/index.html"))

print("\n=== Check if there are any error pages ===")
print(run("find /app/dist -name '*.html' 2>/dev/null"))

ssh.close()
