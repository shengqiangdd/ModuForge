#!/usr/bin/env python3
"""Check container dist and find deployment path"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Check dist contents
    print("=== Container /app/dist ===")
    out, _ = run(f"docker exec {CONTAINER} ls -la /app/dist/")
    print(out)
    
    # Check binary
    print("\n=== Binary info ===")
    out, _ = run(f"docker exec {CONTAINER} file /app/dist/moduforge")
    print(out)
    
    # Check what's in /app
    print("\n=== /app tree ===")
    out, _ = run(f"docker exec {CONTAINER} find /app -type f | head -30")
    print(out)
    
    # Check if there's a dockerfile or build script
    print("\n=== Docker image history ===")
    out, _ = run(f"docker history moduforge:latest --no-trunc | head -20")
    print(out[:800])
    
    # Check if we can install Go in container
    print("\n=== Check Go in container ===")
    out, _ = run(f"docker exec {CONTAINER} which go")
    print(f"Go: {out.strip()}")
    
    # Check container OS
    print("\n=== Container OS ===")
    out, _ = run(f"docker exec {CONTAINER} cat /etc/os-release")
    print(out[:300])
    
    client.close()

if __name__ == "__main__":
    main()
