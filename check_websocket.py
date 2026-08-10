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

print("=== Check WebSocket/SSE related logs ===")
logs = run("docker logs --tail 200 moduforge 2>&1")
for line in logs.split('\n'):
    if any(keyword in line.lower() for keyword in ['websocket', 'sse', 'stream', 'event', 'connect']):
        print(line)

print("\n=== Check for hanging requests ===")
# Check if there are any long-running requests
print(run("docker exec moduforge ps aux | grep -E 'curl|wget' | head -5"))

print("\n=== Check server process ===")
print(run("docker exec moduforge ps aux | grep server"))

print("\n=== Check memory usage ===")
print(run("docker stats moduforge --no-stream --format '{{.MemUsage}}' 2>/dev/null || echo 'Stats not available'"))

print("\n=== Check if there are any stuck goroutines ===")
# This would require pprof, but let's check if there are any obvious issues
print(run("docker exec moduforge ls -la /proc/1/fd 2>/dev/null | wc -l"))

ssh.close()
