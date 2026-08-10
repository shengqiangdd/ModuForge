#!/usr/bin/env python3
"""Check host volume mapping for ModuForge"""
import sys, io, json
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=30)
def run(cmd, timeout=30):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
    return stdout.read().decode(), stderr.read().decode()

# Check docker volumes
print("=== Docker volumes for moduforge ===")
out, _ = run('docker volume ls | grep moduforge')
print(out)

print("\n=== Docker inspect Mounts ===")
out, _ = run('docker inspect moduforge --format="{{range .Mounts}}{{.Type}} {{.Source}} -> {{.Destination}}{{println}}{{end}}"')
print(f"Mounts: '{out.strip()}'")

print("\n=== Docker inspect Binds ===")
out, _ = run('docker inspect moduforge --format="{{json .HostConfig.Binds}}"')
print(f"Binds: {out}")

# Check the host path directly
print("\n=== Host: /vol1/docker/volumes/moduforge_moduforge_data/_data/ ===")
out, _ = run('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/ 2>/dev/null')
print(out)

print("\n=== Host: projects dir ===")
out, _ = run('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/projects/ 2>/dev/null')
print(out)

# Check if the project files exist on host
print("\n=== Host: project main.rs ===")
out, _ = run('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864/src/main.rs 2>/dev/null')
print(out)

# Check docker-compose.yml
print("\n=== docker-compose.yml ===")
out, _ = run('cat /vol1/docker/volumes/moduforge_moduforge_data/_data/docker-compose.yml 2>/dev/null || cat /vol1/docker/volumes/moduforge_moduforge_data/docker-compose.yml 2>/dev/null')
print(out[:2000])

# Check if there's a different compose file location
print("\n=== Find docker-compose files ===")
out, _ = run('find /vol1/docker -name "docker-compose*" 2>/dev/null | head -10')
print(out)

client.close()
