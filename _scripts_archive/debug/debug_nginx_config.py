#!/usr/bin/env python3
"""Debug why nginx not listening on 8086"""
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
    print(out)
    
    # Check nginx.conf to see how includes work
    print("\n=== nginx.conf (around line 131) ===")
    out, _ = run("sed -n '125,140p' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check if there's a http block before conf.d
    print("\n=== Check http block ===")
    out, _ = run("grep -n 'http {' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check all listening ports
    print("\n=== All listening ports ===")
    out, _ = run("netstat -tlnp | grep nginx")
    print(out)
    
    # Check nginx error log for config issues
    print("\n=== Recent nginx errors ===")
    out, _ = run("tail -5 /usr/trim/nginx/logs/error.log")
    print(out)
    
    # Try to test config with verbose
    print("\n=== Test config verbose ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -T 2>&1 | grep -A5 'moduforge'")
    print(f"Output: {out}")
    print(f"Error: {err}")
    
    client.close()

if __name__ == "__main__":
    main()
