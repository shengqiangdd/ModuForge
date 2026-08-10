#!/usr/bin/env python3
"""Check Docker port mapping"""

import sys
import paramiko

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216')

# Check Docker port mapping
print("Checking Docker port mapping...")
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}"')
ip_address = stdout.read().decode().strip()
print(f"Container IP address: {ip_address}")

# Check port mapping
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format "{{json .NetworkSettings.Ports}}"')
ports = stdout.read().decode().strip()
print(f"Port mapping: {ports}")

# Check if the container is accessible from the host
print("\nTesting container accessibility from host...")
stdin, stdout, stderr = ssh.exec_command(f'curl -s http://{ip_address}:8080/health')
health = stdout.read().decode().strip()
print(f"Health from container IP: {health}")

# Check if the host can access the container
print("\nTesting host access to container...")
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
health = stdout.read().decode().strip()
print(f"Health from localhost:8086: {health}")

# Check Docker compose file
print("\nChecking Docker compose file...")
stdin, stdout, stderr = ssh.exec_command('cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml 2>/dev/null | head -50')
compose = stdout.read().decode()
print(compose)

ssh.close()