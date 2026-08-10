# -*- coding: utf-8 -*-
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check what's using port 8086
print("=== Port 8086 usage ===")
print(run("docker ps --format '{{.Names}}:{{.Ports}}' | grep 8086"))

# Check all containers
print("\n=== All containers ===")
print(run("docker ps -a --format '{{.Names}}\t{{.Status}}\t{{.Ports}}'"))

# Stop moduforge first, then check
run("docker rm -f moduforge 2>/dev/null")

# Check port again
print("\n=== Port after cleanup ===")
print(run("docker ps --format '{{.Names}}:{{.Ports}}' | grep 8086"))

# Check what's listening on 8086
print("\n=== Listening on 8086 ===")
print(run("ss -tlnp | grep 8086 2>/dev/null || netstat -tlnp | grep 8086 2>/dev/null"))

# If nginx is using it, check nginx config
print("\n=== Nginx check ===")
print(run("docker ps -a --format '{{.Names}}' | grep -i nginx"))
print(run("docker inspect nginx --format '{{range .NetworkSettings.Ports}}{{.}} {{end}}' 2>/dev/null"))

ssh.close()
