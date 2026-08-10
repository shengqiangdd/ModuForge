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

print("=== Check /app directory ===")
print(run("ls -la /app/ 2>/dev/null"))

print("\n=== Check /app/dist directory ===")
print(run("ls -la /app/dist/ 2>/dev/null || echo 'dist directory not found'"))

print("\n=== Check container file system ===")
print(run("docker exec moduforge ls -la /app/ 2>/dev/null"))

print("\n=== Check if dist exists in container ===")
print(run("docker exec moduforge ls -la /app/dist/ 2>/dev/null || echo 'dist not found in container'"))

print("\n=== Check entrypoint ===")
print(run("docker exec moduforge cat /docker-entrypoint.sh 2>/dev/null | head -30"))

print("\n=== Check server binary ===")
print(run("docker exec moduforge ls -la /server /app/server 2>/dev/null"))

ssh.close()
