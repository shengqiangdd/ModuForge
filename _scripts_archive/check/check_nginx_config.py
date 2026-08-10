#!/usr/bin/env python3
"""Check nginx config and reload"""
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
    
    # Check moduforge.conf content
    print("=== moduforge.conf content ===")
    out, _ = run("cat /usr/trim/nginx/conf/conf.d/moduforge.conf")
    print(out[:500])
    
    # Check if nginx is using the config
    print("\n=== Check nginx config includes ===")
    out, _ = run("grep -n 'include' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check if 8086 is in nginx config
    print("\n=== Check for 8086 in config ===")
    out, _ = run("grep -n '8086' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check nginx error log for config issues
    print("\n=== Nginx error log ===")
    out, _ = run("tail -10 /usr/trim/nginx/logs/error.log")
    print(out)
    
    # Try to reload nginx with verbose
    print("\n=== Reload nginx verbose ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -s reload 2>&1")
    print(f"Output: {out}")
    print(f"Error: {err}")
    
    time.sleep(3)
    
    # Check ports again
    print("\n=== Check ports ===")
    out, _ = run("netstat -tlnp | grep -E '(80|8086|8087)'")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
