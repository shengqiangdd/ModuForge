#!/usr/bin/env python3
import paramiko
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

cmds = [
    'docker exec moduforge ls -la /data/',
    'docker exec moduforge file /data/moduforge.db',
]
for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    print(f"$ {cmd}")
    print(f"  {stdout.read().decode().strip()}")

# Check via the settings API that the web UI uses
# First check what routes exist for settings/providers
stdin, stdout, stderr = ssh.exec_command(
    """curl -s http://localhost:8087/api/v1/providers/custom 2>&1""",
    timeout=10
)
print(f"\nGET /api/v1/providers/custom: {stdout.read().decode().strip()[:300]}")

ssh.close()
