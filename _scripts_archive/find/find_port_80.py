#!/usr/bin/env python3
"""Find what's using port 80"""
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
    
    def run(cmd, timeout=15):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Find process on port 80
    print("=== Process on port 80 ===")
    out, _ = run("lsof -i :80 2>/dev/null || ss -tlnp | grep :80")
    print(out)
    
    # Find process on port 5666
    print("\n=== Process on port 5666 ===")
    out, _ = run("lsof -i :5666 2>/dev/null || ss -tlnp | grep :5666")
    print(out)
    
    # Check all listening ports
    print("\n=== All listening ports ===")
    out, _ = run("netstat -tlnp 2>/dev/null | head -30")
    print(out)
    
    # Check if it's in a container
    print("\n=== Check containers with port 80 ===")
    out, _ = run("docker ps --format '{{.Names}}: {{.Ports}}' | grep -E '(:80|:5666)'")
    print(out)
    
    # Check nginx config in moduforge container
    print("\n=== Check moduforge container ports ===")
    out, _ = run("docker inspect moduforge --format '{{json .NetworkSettings.Ports}}'")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
