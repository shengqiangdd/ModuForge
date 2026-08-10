#!/usr/bin/env python3
"""Test if container can access /data properly."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop moduforge first
    print("Stopping moduforge...")
    ssh.exec_command("docker stop moduforge")
    
    # Test volume mount with correct image
    print("\n=== Testing volume mount ===")
    cmd = """docker run --rm -v moduforge_data:/data moduforge:latest ls -la /data"""
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    # Test with user 1000
    print("\n=== Testing as user 1000 ===")
    cmd = """docker run --rm -v moduforge_data:/data --user 1000:1001 moduforge:latest ls -la /data"""
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    # Test database access
    print("\n=== Testing database access ===")
    cmd = """docker run --rm -v moduforge_data:/data --user 1000:1001 moduforge:latest ls -la /data/moduforge.db"""
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    test()
