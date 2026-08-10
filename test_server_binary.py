#!/usr/bin/env python3
"""
Test server binary directly - bypass entrypoint.
"""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test server binary directly ===")
    cmd = """docker run --rm -v moduforge_data:/data moduforge:latest sh -c "ls -la /data/ && DB_PATH=/data/moduforge.db /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:1500]}")
    if error:
        print(f"Error: {error[:500]}")
    
    print("\n=== Check server binary ===")
    cmd = """docker run --rm moduforge:latest sh -c "ls -la /server && file /server && ldd /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    test()
