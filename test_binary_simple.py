#!/usr/bin/env python3
"""Test if the server binary works at all."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Run server with --help or --version ===")
    cmd = """docker run --rm --entrypoint /server moduforge:latest --help 2>&1 | head -20"""
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode()[:500])
    
    print("\n=== Test: Check if server can start at all ===")
    cmd = """docker run --rm --entrypoint sh moduforge:latest -c "ls -la /server && /server --help" 2>&1 | head -30"""
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    test()
