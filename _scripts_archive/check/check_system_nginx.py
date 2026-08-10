#!/usr/bin/env python3
"""Check system nginx config"""
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
    
    # Check nginx config
    print("=== Main nginx.conf ===")
    out, _ = run("cat /usr/trim/nginx/conf/nginx.conf")
    print(out[:1000])
    
    # Check conf.d
    print("\n=== conf.d contents ===")
    out, _ = run("ls -la /usr/trim/nginx/conf/conf.d/ 2>/dev/null || echo 'no conf.d'")
    print(out)
    
    # Check default.conf
    print("\n=== default.conf ===")
    out, _ = run("cat /usr/trim/nginx/conf/conf.d/default.conf 2>/dev/null || echo 'no default.conf'")
    print(out[:1000])
    
    # Check what's proxied to 5666
    print("\n=== Check 5666 proxy ===")
    out, _ = run("grep -r '5666' /usr/trim/nginx/conf/ 2>/dev/null")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
