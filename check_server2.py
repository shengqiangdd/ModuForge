#!/usr/bin/env python3
"""Check server file structure"""
import sys, io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=15)

REPO = "/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge"

cmds = [
    f"ls -la {REPO}/",
    f"ls -la {REPO}/backend/ | head -20",
    f"cat {REPO}/docker-compose.yml 2>/dev/null | head -30",
    f"find {REPO} -name 'Dockerfile*' 2>/dev/null",
    # Check if there's a different compose file
    f"docker compose -f {REPO}/docker-compose.yml config --services 2>/dev/null",
]

for cmd in cmds:
    print(f"\n>>> {cmd}")
    _, stdout, _ = client.exec_command(cmd, timeout=10)
    out = stdout.read().decode('utf-8', errors='replace').strip()
    if out:
        for line in out.split('\n')[:30]:
            print(f"  {line}")

client.close()
