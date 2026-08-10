#!/usr/bin/env python3
"""Verify the running binary matches our source"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

# Check if the binary in container matches what we uploaded
# by looking for the new strings
checks = [
    ('docker exec moduforge strings /app/moduforge-server | grep -c "not in DB"', "old string count"),
    ('docker exec moduforge strings /app/moduforge-server | grep -c "WHERE name="', "new string count"),
    ('docker exec moduforge strings /app/moduforge-server | grep "custom provider" | head -5', "custom provider strings"),
]

for cmd, desc in checks:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=15)
    out = stdout.read().decode("utf-8", errors="ignore").strip()
    err = stderr.read().decode("utf-8", errors="ignore").strip()
    print(f"{desc}: {out}")
    if err:
        print(f"  stderr: {err}")

# Also compare MD5
import hashlib, os
local_path = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"
with open(local_path, "rb") as f:
    local_md5 = hashlib.md5(f.read()).hexdigest()
print(f"\nLocal binary MD5: {local_md5}")

stdin, stdout, stderr = ssh.exec_command("md5sum /tmp/moduforge-server-new 2>/dev/null || docker exec moduforge md5sum /app/moduforge-server", timeout=10)
remote_md5 = stdout.read().decode().strip().split()[0] if stdout.read().decode().strip() else "unknown"
# Re-read since we consumed it
stdin, stdout, stderr = ssh.exec_command("md5sum /tmp/moduforge-server-new 2>/dev/null || docker exec moduforge md5sum /app/moduforge-server", timeout=10)
line = stdout.read().decode().strip()
print(f"Remote (tmp/container) MD5: {line}")

ssh.close()
