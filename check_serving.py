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

print("=== Test frontend access ===")
# Test if frontend is being served
result = run("docker exec moduforge curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/")
print(f"Frontend HTTP status: {result}")

print("\n=== Check index.html content ===")
print(run("docker exec moduforge head -30 /app/dist/index.html"))

print("\n=== Check server logs for static file serving ===")
logs = run("docker logs --tail 100 moduforge 2>&1")
# Look for static file related logs
for line in logs.split('\n'):
    if 'dist' in line.lower() or 'static' in line.lower() or 'frontend' in line.lower():
        print(line)

print("\n=== Check if DIST_DIR env var is set ===")
print(run("docker exec moduforge env | grep -i dist"))

print("\n=== Check server configuration ===")
# Check if the server is configured to serve from /app/dist
print(run("docker exec moduforge strings /server | grep -i 'dist\\|frontend' | head -10"))

ssh.close()
