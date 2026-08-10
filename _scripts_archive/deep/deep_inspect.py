#!/usr/bin/env python3
"""Deep inspection of ModuForge container"""
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

# Find ALL files in the container
print("=== All files in /app ===")
out, _ = run('docker exec moduforge find /app -type f 2>/dev/null | head -50')
print(out)

print("\n=== Container volumes ===")
out, _ = run('docker inspect moduforge --format="{{json .Mounts}}"')
print(out)

# Check where the DB is
print("\n=== Find .db files ===")
out, _ = run('docker exec moduforge find / -name "*.db" -o -name "*.sqlite" 2>/dev/null | head -10')
print(out)

# Check /data directory
print("\n=== /data ===")
out, _ = run('docker exec moduforge ls -la /data/ 2>/dev/null')
print(out)

# Check /var/data
print("\n=== /var ===")
out, _ = run('docker exec moduforge ls -la /var/lib/ 2>/dev/null | head -10')
print(out)

# Check the dist directory
print("\n=== dist structure ===")
out, _ = run('docker exec moduforge find /app/dist -type f | head -30')
print(out)

# Check all env vars
print("\n=== ALL env vars ===")
out, _ = run('docker exec moduforge printenv 2>/dev/null')
print(out)

# Find config files
print("\n=== Config files ===")
out, _ = run('docker exec moduforge find / -name "*.yaml" -o -name "*.toml" -o -name "*.json" -o -name "*.conf" 2>/dev/null | grep -v proc | grep -v sys | head -20')
print(out)

# Check the DB from the dist directory
print("\n=== /app/dist contents ===")
out, _ = run('docker exec moduforge ls -la /app/dist/ 2>/dev/null')
print(out)

# The server binary
print("\n=== Server binary ===")
out, _ = run('docker exec moduforge file /server 2>/dev/null')
print(out)

# Check where the server reads config from
print("\n=== Check /etc/config ===")
out, _ = run('docker exec moduforge find /etc -name "*moduforge*" -o -name "*config*" 2>/dev/null | head -10')
print(out)

# Check /home
print("\n=== /home ===")
out, _ = run('docker exec moduforge ls -laR /home/ 2>/dev/null')
print(out)

# Check /tmp
print("\n=== /tmp ===")
out, _ = run('docker exec moduforge ls -la /tmp/ 2>/dev/null')
print(out)

client.close()
