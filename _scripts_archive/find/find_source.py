#!/usr/bin/env python3
"""Find source code and Dockerfile on server"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Search for Dockerfile
    print("=== Search for Dockerfile ===")
    out, _ = run("find /home/admin -name 'Dockerfile*' -o -name 'docker-compose*' 2>/dev/null | head -20")
    print(out)
    
    # Search for Go source
    print("\n=== Search for Go source ===")
    out, _ = run("find /home/admin -name '*.go' -type f 2>/dev/null | head -20")
    print(out)
    
    # Search for go.mod
    print("\n=== Search for go.mod ===")
    out, _ = run("find /home/admin -name 'go.mod' -type f 2>/dev/null | head -20")
    print(out)
    
    # Check if there's a backup of the binary
    print("\n=== Check for binary backup ===")
    out, _ = run("find /home/admin -name 'moduforge' -type f 2>/dev/null | head -10")
    print(out)
    
    # Check if we can extract binary from container
    print("\n=== Copy binary from container ===")
    out, _ = run("docker cp moduforge:/server /home/admin/server_backup")
    print(f"Copy: {out}")
    
    out, _ = run("ls -la /home/admin/server_backup 2>/dev/null || echo 'failed'")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
