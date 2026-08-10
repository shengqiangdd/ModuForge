#!/usr/bin/env python3
"""Fix nginx PID and reload"""
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
    
    # Find nginx master PID
    print("=== Find nginx master PID ===")
    out, _ = run("ps aux | grep 'nginx: master' | grep -v grep | awk '{print $2}'")
    master_pid = out.strip()
    print(f"Master PID: {master_pid}")
    
    # Write PID file
    print("\n=== Write PID file ===")
    if master_pid:
        run(f"echo {master_pid} | sudo tee /run/nginx.pid")
        print(f"Written PID {master_pid} to /run/nginx.pid")
    
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
