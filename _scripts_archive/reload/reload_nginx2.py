#!/usr/bin/env python3
"""Reload nginx using kill command"""
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
    
    if master_pid:
        # Send HUP signal to reload
        print(f"\n=== Send HUP to PID {master_pid} ===")
        out, err = run(f"sudo kill -HUP {master_pid}")
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
    else:
        print("No nginx master process found")
    
    # Check all listening ports
    print("\n=== All listening ports ===")
    out, _ = run("netstat -tlnp | grep -E '(80|8086|8087|5666)'")
    print(out)
    
    client.close()

if __name__ == "__main__":
    main()
