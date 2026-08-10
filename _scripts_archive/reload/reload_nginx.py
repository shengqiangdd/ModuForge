#!/usr/bin/env python3
"""Reload nginx properly"""
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
    
    # Check nginx master PID
    print("=== Check nginx master PID ===")
    out, _ = run("cat /run/nginx.pid")
    print(f"PID: {out}")
    
    # Reload nginx
    print("\n=== Reload nginx ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -s reload")
    print(f"Reload: {out}")
    if err:
        print(f"Error: {err}")
    
    time.sleep(3)
    
    # Check if 8086 is now listening
    print("\n=== Check port 8086 ===")
    out, _ = run("netstat -tlnp | grep :8086")
    print(out)
    
    # Test
    print("\n=== Test ===")
    out, _ = run("curl -sI http://localhost:8086/health")
    print(out[:300])
    
    # Check nginx error log
    print("\n=== Nginx error log ===")
    out, _ = run("tail -5 /usr/trim/nginx/logs/error.log")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
