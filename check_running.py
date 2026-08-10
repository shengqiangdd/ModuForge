#!/usr/bin/env python3
import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode().strip()

print("=== Check running binary ===")
out = run("docker exec moduforge ls -la /server /app/server")
print(out)

print("\n=== Check if webroot logic is in running binary ===")
out = run("docker exec moduforge strings /server | grep -c webroot")
print(f"webroot count in /server: {out}")

out = run("docker exec moduforge strings /app/server | grep -c webroot")
print(f"webroot count in /app/server: {out}")

print("\n=== Check isFrontendFile function ===")
out = run("docker exec moduforge strings /server | grep -i 'isFrontendFile\\|frontend'")
print(f"Frontend strings in /server: {out[:500]}")

print("\n=== Check which binary is running ===")
out = run("docker exec moduforge ps aux | grep server")
print(f"Running process: {out}")

print("\n=== Check entrypoint ===")
out = run("docker exec moduforge cat /docker-entrypoint.sh")
print(f"Entrypoint: {out}")

ssh.close()
