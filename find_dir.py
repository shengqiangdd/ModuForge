#!/usr/bin/env python3
"""Find ModuForge directory on server."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect(HOST, username=USER, password=PASSWORD, timeout=15)
print(f"Connected to {USER}@{HOST}")

# Find docker containers
stdin, stdout, stderr = client.exec_command("docker ps -a --format '{{.Names}} {{.Image}} {{.Status}}'")
print("\n=== Docker containers ===")
print(stdout.read().decode())

# Find moduforge directories
stdin, stdout, stderr = client.exec_command("find /home /opt /root /srv -name 'docker-compose.yml' -o -name 'compose.yml' 2>/dev/null | head -20")
print("\n=== Compose files ===")
print(stdout.read().decode())

# Check docker compose in common locations
for d in ["/home/admin", "/home/admin/moduforge", "/root", "/opt"]:
    stdin, stdout, stderr = client.exec_command(f"ls -la {d}/docker-compose.yml 2>/dev/null || echo 'not found'")
    out = stdout.read().decode().strip()
    if "not found" not in out:
        print(f"\nFound: {d}/docker-compose.yml")
        print(out)

client.close()
