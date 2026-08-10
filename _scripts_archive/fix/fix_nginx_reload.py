#!/usr/bin/env python3
"""Fix nginx reload and verify"""
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
    
    # Check nginx PID
    print("=== Check nginx PID ===")
    out, _ = run("cat /run/nginx.pid 2>/dev/null || echo 'no pid'")
    print(f"PID: {out}")
    
    # Find nginx master process
    print("\n=== Find nginx master ===")
    out, _ = run("ps aux | grep 'nginx: master' | grep -v grep")
    print(out)
    
    # Check if moduforge.conf is included
    print("\n=== Check nginx.conf includes ===")
    out, _ = run("grep -n 'include' /usr/trim/nginx/conf/nginx.conf")
    print(out)
    
    # Check if our config is loaded
    print("\n=== Check if moduforge.conf loaded ===")
    out, _ = run("cat /usr/trim/nginx/conf/conf.d/moduforge.conf | head -5")
    print(out)
    
    # Try to reload nginx properly
    print("\n=== Reload nginx ===")
    out, err = run("sudo killall -HUP nginx")
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
    
    client.close()

if __name__ == "__main__":
    main()
