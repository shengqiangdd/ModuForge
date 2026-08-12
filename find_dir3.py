#!/usr/bin/env python3
"""Find ModuForge directory on server."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect(HOST, username=USER, password=PASSWORD, timeout=15)

# Check the compose working dir
stdin, stdout, stderr = client.exec_command("ls -la /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/")
print("=== Compose working dir ===")
print(stdout.read().decode())

# Check /root/moduforge
stdin, stdout, stderr = client.exec_command("ls -la /root/moduforge/")
print("=== /root/moduforge ===")
print(stdout.read().decode())

# Check if there's a git repo in either
stdin, stdout, stderr = client.exec_command("cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && git remote -v 2>/dev/null || echo 'no git'")
print("=== Git remote (compose dir) ===")
print(stdout.read().decode())

stdin, stdout, stderr = client.exec_command("cd /root/moduforge && git remote -v 2>/dev/null || echo 'no git'")
print("=== Git remote (/root/moduforge) ===")
print(stdout.read().decode())

client.close()
