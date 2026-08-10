#!/usr/bin/env python3
"""Find what's using port 80 and 5666"""
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
    print("=== Find process on port 80 ===")
    out, _ = run("sudo lsof -i :80 -P 2>/dev/null || sudo ss -tlnp | grep :80")
    print(out)
    
    # Find process on port 5666
    print("\n=== Find process on port 5666 ===")
    out, _ = run("sudo lsof -i :5666 -P 2>/dev/null || sudo ss -tlnp | grep :5666")
    print(out)
    
    # Check if it's nginx
    print("\n=== Check nginx processes ===")
    out, _ = run("ps aux | grep nginx")
    print(out[:500])
    
    # Check if it's in a container
    print("\n=== Check containers ===")
    out, _ = run("docker ps --format '{{.Names}}: {{.Ports}}'")
    print(out)
    
    # Try to find the nginx config
    print("\n=== Find nginx config ===")
    out, _ = run("find / -name 'nginx.conf' -o -name 'default.conf' 2>/dev/null | head -10")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
