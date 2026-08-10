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

print("=== Container Status ===")
print(run("docker ps --filter name=moduforge --format '{{.Names}} {{.Status}}'"))

print("\n=== Recent Logs ===")
logs = run("docker logs --tail 50 moduforge 2>&1")
print(logs[-2000:] if len(logs) > 2000 else logs)

print("\n=== Health Check ===")
print(run("docker exec moduforge curl -s http://localhost:8080/health"))

print("\n=== Check for stuck processes ===")
print(run("docker exec moduforge ps aux | grep -E 'server|go' | head -5"))

ssh.close()
