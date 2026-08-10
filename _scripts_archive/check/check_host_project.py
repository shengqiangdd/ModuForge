#!/usr/bin/env python3
"""Check host project structure"""
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

# Check project structure on host
print("=== Project 1785249992652501794-1864 structure ===")
out, _ = run('find /vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864 -type f 2>/dev/null | head -50')
print(out)

print("\n=== main.rs exists? ===")
out, _ = run('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864/src/main.rs 2>/dev/null')
print(out)

# Check what the container SHOULD be mounting
# The old docker run command probably had -v mounts
print("\n=== Container image name ===")
out, _ = run('docker inspect moduforge --format="{{.Config.Image}}"')
print(f"Image: {out.strip()}")

# Check all containers to find the right one
print("\n=== All running containers ===")
out, _ = run('docker ps --format "table {{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Ports}}"')
print(out)

# Check if there's another moduforge container
print("\n=== All containers (including stopped) ===")
out, _ = run('docker ps -a --filter "name=modu" --format "table {{.ID}}\t{{.Names}}\t{{.Status}}\t{{.Image}}"')
print(out)

# The correct approach: mount the host volume into the container
# Let me check what the original docker run command was
print("\n=== Container config (look for volume mounts) ===")
out, _ = run('docker inspect moduforge --format="{{json .HostConfig}}" | python3 -m json.tool 2>/dev/null | head -50')
print(out[:2000])

# Check if there's a compose file
print("\n=== Find compose files ===")
out, _ = run('find /vol1/docker -name "docker-compose*" -o -name "compose.yaml" -o -name "compose.yml" 2>/dev/null | head -10')
print(out)

# Check the deploy script
print("\n=== deploy_unified.py ===")
out, _ = run('cat /vol1/docker/volumes/moduforge_moduforge_data/deploy_unified.py 2>/dev/null | head -80')
print(out)

client.close()
