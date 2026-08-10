#!/usr/bin/env python3
"""Final status check"""
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
    
    # Check ModuForge health
    print("=== ModuForge Health ===")
    out, _ = run("curl -s http://localhost:8087/health")
    print(f"Direct (8087): {out}")
    
    out, _ = run("curl -s http://localhost:8086/health")
    print(f"Proxy (8086): {out}")
    
    # Check containers
    print("\n=== Containers ===")
    out, _ = run("docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}'")
    print(out)
    
    # Check nginx config
    print("\n=== Nginx Config ===")
    out, _ = run("cat /tmp/moduforge_nginx.conf | head -5")
    print(f"Config: {out}")
    
    # Check nginx logs
    print("\n=== Nginx Logs ===")
    out, _ = run("docker logs moduforge-nginx --tail 5 2>&1")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
