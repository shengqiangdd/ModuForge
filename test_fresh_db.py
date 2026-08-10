#!/usr/bin/env python3
"""Test with a fresh database."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test 1: Create fresh DB and test ===")
    cmd = """docker run --rm -v moduforge_data:/data moduforge:latest sh -c "rm -f /data/moduforge.db* && ls -la /data && /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:1000]}")
    print(f"Error: {error[:500]}")
    
    print("\n=== Test 2: Check if directory exists ===")
    cmd = """docker run --rm -v moduforge_data:/data alpine sh -c "ls -la /data && mkdir -p /data && touch /data/test.txt && ls -la /data" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    print("\n=== Test 3: Try with explicit path ===")
    cmd = """docker run --rm -v moduforge_data:/data --user 1000:1001 moduforge:latest sh -c "whoami && id && ls -la /data && DB_PATH=/data/test.db /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    print(f"Output: {output[:1000]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
