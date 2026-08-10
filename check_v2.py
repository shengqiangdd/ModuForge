#!/usr/bin/env python3
import paramiko
import time
import sys

# Fix encoding
sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace').strip()

print("=== Check container status ===")
print(run("docker ps -a --filter name=moduforge --format '{{.Names}} {{.Status}}'"))

print("\n=== Check logs ===")
logs = run("docker logs --tail 20 moduforge 2>&1")
print(logs[:1000])

print("\n=== Check binary ===")
print(run("docker exec moduforge ls -la /server /app/server 2>&1"))

print("\n=== Check webroot in binary ===")
print(run("docker exec moduforge strings /server | grep webroot | head -3 2>&1"))

print("\n=== Check if isFrontendFile is compiled in ===")
print(run("docker exec moduforge strings /server | grep -i 'isFrontendFile\\|frontendPatterns' | head -5 2>&1"))

ssh.close()
