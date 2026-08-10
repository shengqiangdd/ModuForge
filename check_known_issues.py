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

print("=== Check for known issues ===")
# Check if there are any known issues with the current deployment
logs = run("docker logs --tail 200 moduforge 2>&1")

# Look for specific patterns that might indicate issues
issues = []
for line in logs.split('\n'):
    if any(keyword in line.lower() for keyword in ['error', 'fail', 'timeout', 'reject', 'exception']):
        issues.append(line)

if issues:
    print("Found potential issues:")
    for issue in issues[-10:]:
        print(issue)
else:
    print("No obvious issues found in logs")

print("\n=== Check for specific API failures ===")
# Check if there are any specific API failures
result = run("docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}' 2>/dev/null")
print(f"Login test: {result[:100]}...")

print("\n=== Check for any stuck processes ===")
# Check if there are any stuck processes
result = run("docker exec moduforge ps aux | grep -E 'server|go' | head -5")
print(f"Processes: {result}")

print("\n=== Check memory usage ===")
# Check memory usage
result = run("docker stats moduforge --no-stream --format '{{.MemUsage}}' 2>/dev/null || echo 'Stats not available'")
print(f"Memory: {result}")

ssh.close()
