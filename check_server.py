#!/usr/bin/env python3
"""Check server paths"""
import sys, io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')
import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect("192.168.2.9", username="admin", password="csq0216", timeout=15)

# Find ModuForge repo
cmds = [
    "ls -la /vol1/ 2>/dev/null || echo 'no /vol1'",
    "find /home -name 'docker-compose.yml' -path '*ModuForge*' 2>/dev/null | head -5",
    "find /opt -name 'docker-compose.yml' -path '*ModuForge*' 2>/dev/null | head -5",
    "find /root -name 'docker-compose.yml' -path '*ModuForge*' 2>/dev/null | head -5",
    "docker inspect moduforge --format '{{json .Mounts}}' 2>/dev/null | python3 -m json.tool 2>/dev/null || docker inspect moduforge --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{println}}{{end}}' 2>/dev/null",
    "docker inspect moduforge --format '{{json .Config.Labels}}' 2>/dev/null | head -200",
]

for cmd in cmds:
    print(f"\n>>> {cmd}")
    _, stdout, _ = client.exec_command(cmd, timeout=10)
    out = stdout.read().decode('utf-8', errors='replace').strip()
    if out:
        for line in out.split('\n')[:20]:
            print(f"  {line}")

client.close()
