#!/usr/bin/env python3
"""
Fix deployment - remove Windows backslash paths and unnecessary files.
"""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def fix():
    print("=" * 60)
    print("Fixing ModuForge Deployment")
    print("=" * 60)
    
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop container
    print("\n1. Stopping container...")
    ssh.exec_command(f"docker stop {CONTAINER}")
    import time
    time.sleep(2)
    
    # Clean up problematic files inside container
    print("\n2. Cleaning up backslash paths...")
    commands = [
        # Remove files with backslash in name (Windows artifact)
        f"docker exec {CONTAINER} find /app/dist -name '*\\\\*' -delete",
        # Remove unnecessary files
        f"docker exec {CONTAINER} rm -f /app/dist/manifest.json",
        f"docker exec {CONTAINER} rm -f /app/dist/sw.js",
        f"docker exec {CONTAINER} rm -f /app/dist/icon-192.svg",
        f"docker exec {CONTAINER} rm -f /app/dist/icon-512.svg",
        f"docker exec {CONTAINER} rm -f /app/dist/fonts/MaterialSymbolsOutlined.ttf",
    ]
    
    for cmd in commands:
        print(f"  > {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        stdout.channel.recv_exit_status()
    
    # Start container
    print("\n3. Starting container...")
    ssh.exec_command(f"docker start {CONTAINER}")
    time.sleep(3)
    
    # Verify
    print("\n4. Verifying...")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} find /app/dist -type f | wc -l")
    count = stdout.read().decode().strip()
    print(f"  Files: {count}")
    
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} du -sh /app/dist")
    size = stdout.read().decode().strip()
    print(f"  Size: {size}")
    
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"  Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

if __name__ == "__main__":
    import sys
    success = fix()
    sys.exit(0 if success else 1)
