#!/usr/bin/env python3
import paramiko
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect("192.168.2.9", username="admin", password="csq0216", timeout=10)

cmds = [
    # Check all running containers
    "docker ps --format '{{.Names}}\t{{.Image}}\t{{.Command}}'",
    # Check all processes in moduforge
    "docker exec moduforge ps aux",
    # Check if there's a supervisord or multiple processes
    "docker exec moduforge sh -c 'ls /etc/supervisor/ 2>/dev/null || ls /etc/services/ 2>/dev/null || echo none'",
    # Check what's listening on 8080
    "docker exec moduforge sh -c 'cat /proc/1/cmdline | tr \"\\0\" \" \" '",
    # Find the binary that's actually running
    "docker exec moduforge sh -c 'ls -la /proc/1/exe 2>/dev/null'",
    # Check if there's a symlink or wrapper
    "docker exec moduforge sh -c 'file /app/moduforge-server 2>/dev/null || echo no-file-cmd'",
    # Check inode of binary
    "docker exec moduforge sh -c 'ls -li /app/moduforge-server'",
    # Check inode of running binary
    "docker exec moduforge sh -c 'ls -li /proc/1/exe 2>/dev/null'",
]

for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode("utf-8", errors="ignore").strip()
    err = stderr.read().decode("utf-8", errors="ignore").strip()
    print(f"$ {cmd[:80]}")
    if out: print(f"  {out[:400]}")
    if err and err != out: print(f"  ERR: {err[:200]}")

ssh.close()
