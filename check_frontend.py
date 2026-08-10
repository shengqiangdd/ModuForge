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

print("=== Check frontend dist files ===")
print(run("ls -la /app/dist/ 2>/dev/null | head -20"))

print("\n=== Check if index.html exists ===")
print(run("head -20 /app/dist/index.html 2>/dev/null || echo 'index.html not found'"))

print("\n=== Check for loading/spinner related JS ===")
# Search for loading state in JS files
result = run("grep -r 'loading\\|spinner\\|processing' /app/dist/assets/*.js 2>/dev/null | head -5")
print(result if result else "No matches found")

print("\n=== Check recent API calls ===")
# Check if there are any hanging requests
print(run("docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H 'Authorization: Bearer test' 2>/dev/null | head -100"))

ssh.close()
