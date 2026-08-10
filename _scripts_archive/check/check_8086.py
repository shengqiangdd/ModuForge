#!/usr/bin/env python3
"""Check if nginx is now listening on 8086"""
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
    
    # Check all listening ports
    print("=== All listening ports ===")
    out, _ = run("netstat -tlnp | grep -E '(80|8086|8087|5666)'")
    print(out)
    
    # Check if 8086 is listening
    print("\n=== Port 8086 ===")
    out, _ = run("netstat -tlnp | grep :8086")
    print(out)
    
    # Test
    print("\n=== Test health ===")
    out, _ = run("curl -s http://localhost:8086/health")
    print(out)
    
    # Test with headers
    print("\n=== Test with headers ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out)
    
    # Check nginx error log
    print("\n=== Nginx error log ===")
    out, _ = run("tail -5 /usr/trim/nginx/logs/error.log")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
