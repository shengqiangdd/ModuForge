#!/usr/bin/env python3
"""Test with strace to see what's happening."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Check if strace is available ===")
    cmd = """docker run --rm --user root moduforge:latest sh -c "which strace || apk add strace 2>/dev/null" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode()[:500])
    
    print("\n=== Test: Run with strace ===")
    cmd = """docker run --rm --user root -v moduforge_data:/data moduforge:latest sh -c "rm -f /data/moduforge.db* && strace -e trace=open,openat,access,stat /server 2>&1 | head -100" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    print(f"Output: {output[:3000]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
