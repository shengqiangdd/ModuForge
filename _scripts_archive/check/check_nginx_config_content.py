#!/usr/bin/env python3
"""Check nginx config content and fix"""
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
    print(f"Content length: {len(out)}")
    print(f"First 200 chars: {out[:200]}")
    
    # Check if file exists
    print("\n=== File exists? ===")
    out, _ = run("ls -la /usr/trim/nginx/conf/conf.d/moduforge.conf")
    print(out)
    
    # Check file permissions
    print("\n=== File permissions ===")
    out, _ = run("stat /usr/trim/nginx/conf/conf.d/moduforge.conf")
    print(out)
    
    # Check nginx.conf includes
    print("\n=== nginx.conf includes ===")
    out, _ = run("sed -n '125,135p' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check if there are any other conf files in conf.d
    print("\n=== All conf files ===")
    out, _ = run("ls -la /usr/trim/nginx/conf/conf.d/")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
