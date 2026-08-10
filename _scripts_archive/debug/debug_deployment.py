#!/usr/bin/env python3
"""Debug deployment issues"""
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
    
    # Check all containers
    print("=== All containers ===")
    out, _ = run("docker ps -a --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'")
    print(out)
    
    # Check port 5666
    print("\n=== Port 5666 ===")
    out, _ = run("curl -sI http://localhost:5666/health")
    print(out[:300])
    
    # Check nginx config
    print("\n=== Nginx config ===")
    out, _ = run("docker exec moduforge-nginx cat /etc/nginx/conf.d/default.conf")
    print(out)
    
    # Check nginx logs
    print("\n=== Nginx logs ===")
    out, _ = run("docker logs moduforge-nginx --tail 10 2>&1")
    print(out)
    
    # Try to start ModuForge
    print("\n=== Start ModuForge ===")
    run("docker rm -f moduforge 2>/dev/null")
    time.sleep(2)
    out, err = run("docker run -d --name moduforge -p 127.0.0.1:8086:8080 moduforge:latest")
    print(f"Started: {out[:12]}")
    if err:
        print(f"Error: {err}")
    
    time.sleep(10)
    
    # Check health
    print("\n=== ModuForge health ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
