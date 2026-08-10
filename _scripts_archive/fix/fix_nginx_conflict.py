#!/usr/bin/env python3
"""Fix nginx port conflict"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

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
    
    # Check what's using port 80
    print("=== Port 80 usage ===")
    out, _ = run("netstat -tlnp | grep :80")
    print(out)
    
    # Check all nginx containers
    print("\n=== All nginx containers ===")
    out, _ = run("docker ps -a | grep nginx")
    print(out)
    
    # Check port 5666
    print("\n=== Port 5666 ===")
    out, _ = run("netstat -tlnp | grep :5666")
    print(out)
    
    # Stop our nginx
    print("\n=== Stop our nginx ===")
    run("docker stop moduforge-nginx")
    run("docker rm moduforge-nginx")
    
    # Check if there's a system nginx
    print("\n=== System nginx ===")
    out, _ = run("systemctl status nginx 2>/dev/null || echo 'no systemctl'")
    print(out[:300])
    
    # Check port 8086
    print("\n=== Port 8086 ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
