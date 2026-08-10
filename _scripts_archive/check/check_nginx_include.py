#!/usr/bin/env python3
"""Check nginx include and config"""
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
    out, _ = run("cat /usr/trim/nginx/conf/moduforge/moduforge.conf")
    print(out[:500])
    
    # Check nginx.conf includes
    print("\n=== nginx.conf includes ===")
    out, _ = run("grep -n 'include' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check if 8086 is in the config
    print("\n=== Check for 8086 ===")
    out, _ = run("grep -n '8086' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check nginx error log for config issues
    print("\n=== Nginx error log ===")
    out, _ = run("tail -10 /usr/trim/nginx/logs/error.log")
    print(out)
    
    # Try to test config with verbose
    print("\n=== Test config verbose ===")
    out, err = run("sudo /usr/trim/nginx/sbin/nginx -T 2>&1 | grep -A5 'moduforge'")
    print(f"Output: {out}")
    print(f"Error: {err}")
    
    client.close()

if __name__ == "__main__":
    main()
