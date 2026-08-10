#!/usr/bin/env python3
"""Check current status"""
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
    
    # Check containers
    print("=== Running containers ===")
    out, _ = run("docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'")
    print(out)
    
    # Check ModuForge health
    print("\n=== ModuForge health ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(out)
    
    # Check nginx
    print("\n=== Nginx status ===")
    out, _ = run("curl -sI http://localhost:80/health 2>/dev/null || echo 'nginx not running'")
    print(out[:300])
    
    client.close()

if __name__ == "__main__":
    main()
