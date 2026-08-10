#!/usr/bin/env python3
"""Find Go binary and check deployment options"""
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
    
    # Find Go binary
    print("=== Find Go binary ===")
    out, _ = run(f"docker exec {CONTAINER} find / -name 'moduforge*' -type f 2>/dev/null")
    print(out)
    
    # Check entrypoint
    print("\n=== Entrypoint ===")
    out, _ = run(f"docker exec {CONTAINER} cat /docker-entrypoint.sh")
    print(out)
    
    # Check running process
    print("\n=== Running process ===")
    out, _ = run(f"docker exec {CONTAINER} ps aux")
    print(out)
    
    # Check binary in common locations
    print("\n=== Check /usr/local/bin ===")
    out, _ = run(f"docker exec {CONTAINER} ls -la /usr/local/bin/ 2>/dev/null || echo 'not found'")
    print(out[:300])
    
    # Check if we can install Go
    print("\n=== Check package manager ===")
    out, _ = run(f"docker exec {CONTAINER} which apk && apk --version")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
