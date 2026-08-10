#!/usr/bin/env python3
"""Fix nginx config - check structure and fix"""
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
    
    # Check nginx.conf structure
    print("=== nginx.conf structure ===")
    out, _ = run("cat /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check where conf.d is included
    print("\n=== Find include conf.d ===")
    out, _ = run("grep -n 'include.*conf.d' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
