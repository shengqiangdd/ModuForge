#!/usr/bin/env python3
"""Find ModuForge directory on server."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect(HOST, username=USER, password=PASSWORD, timeout=15)

# Find where the container volume is mounted
stdin, stdout, stderr = client.exec_command("docker inspect moduforge --format '{{json .Mounts}}' | python3 -m json.tool")
print("=== Container mounts ===")
print(stdout.read().decode())

# Find compose project
stdin, stdout, stderr = client.exec_command("docker inspect moduforge --format '{{index .Config.Labels \"com.docker.compose.project.working_dir\"}}'")
print("=== Compose working dir ===")
print(stdout.read().decode())

# List home directory
stdin, stdout, stderr = client.exec_command("ls -la ~/")
print("=== Home directory ===")
print(stdout.read().decode())

# Check for git repos
stdin, stdout, stderr = client.exec_command("find /home/admin -maxdepth 3 -name '.git' -type d 2>/dev/null")
print("=== Git repos ===")
print(stdout.read().decode())

client.close()
