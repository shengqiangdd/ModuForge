#!/usr/bin/env python3
"""Test with bind mount instead of named volume."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Create a fresh directory and test ===")
    cmd = """mkdir -p /tmp/moduforge_test && docker run --rm --user root -v /tmp/moduforge_test:/data moduforge:latest sh -c "rm -f /data/moduforge.db* && ls -la /data && DB_PATH=/data/moduforge.db /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:3000]}")
    if error:
        print(f"Error: {error[:1000]}")
    
    print("\n=== Test: Check host directory ===")
    stdin, stdout, stderr = ssh.exec_command("ls -la /tmp/moduforge_test/")
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    test()
