#!/usr/bin/env python3
"""Test if the server binary exists and works."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Testing server binary ===")
    cmd = """docker run --rm moduforge:latest ls -la /server"""
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode())
    
    print("\n=== Testing server execution ===")
    cmd = """docker run --rm moduforge:latest /server --version 2>&1 || echo "Failed with code $?" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    test()
