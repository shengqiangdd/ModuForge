#!/usr/bin/env python3
"""Debug nginx start issue"""
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
    
    # Check if nginx is running
    print("=== Check nginx process ===")
    out, _ = run("ps aux | grep nginx | grep -v grep")
    print(out)
    
    # Check nginx error log
    print("\n=== Nginx error log ===")
    out, _ = run("tail -20 /usr/trim/nginx/logs/error.log")
    print(out)
    
    # Check nginx.conf syntax
    print("\n=== Test nginx config ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -t 2>&1")
    print(f"Output: {out}")
    print(f"Error: {err}")
    
    # Check if port 8086 is in use
    print("\n=== Check port 8086 ===")
    out, _ = run("netstat -tlnp | grep :8086")
    print(out)
    
    # Check all listening ports
    print("\n=== All listening ports ===")
    out, _ = run("netstat -tlnp | head -20")
    print(out)
    
    # Try to start nginx in foreground to see errors
    print("\n=== Try starting nginx ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -g 'daemon off;' &", timeout=5)
    print(f"Output: {out}")
    print(f"Error: {err}")
    
    client.close()

if __name__ == "__main__":
    main()
