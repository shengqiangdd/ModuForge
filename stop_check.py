#!/usr/bin/env python3
import paramiko
import time
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace').strip()

print("=== Step 1: Stop container ===")
run("docker stop moduforge")
time.sleep(3)
print("Container stopped")

print("\n=== Step 2: Check stopped container ===")
print(run("docker ps -a --filter name=moduforge --format '{{.Names}} {{.Status}}'"))

print("\n=== Step 3: Try to run command in stopped container ===")
# Actually, we can't run exec on stopped container. Let's use a different approach.
# We'll modify the entrypoint to use /app/server instead of /server

print("Container is stopped. We need to modify the entrypoint or volume.")

ssh.close()
