#!/usr/bin/env python3
import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode().strip()

print("=== Check container status ===")
print(run("docker ps -a --filter name=moduforge --format '{{.Names}} {{.Status}}'"))

print("\n=== Check logs ===")
print(run("docker logs --tail 20 moduforge 2>&1"))

print("\n=== Check if /server binary is OK ===")
# Start container and immediately check
run("docker start moduforge")
time.sleep(3)
print(run("docker exec moduforge ls -la /server /app/server 2>&1"))
print(run("docker exec moduforge file /server 2>&1"))

# Check the webroot count again
print(run("docker exec moduforge strings /server | grep webroot | head -5 2>&1"))

ssh.close()
