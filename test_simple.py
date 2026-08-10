#!/usr/bin/env python3
"""Simple test - check if the issue is CGO/SQLite."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Check server binary ldd ===")
    cmd = """docker create --name temp_check moduforge:latest ls -la /server"""
    ssh.exec_command(cmd)
    stdin, stdout, stderr = ssh.exec_command("docker cp temp_check:/server /tmp/server_check")
    stdout.channel.recv_exit_status()
    ssh.exec_command("docker rm temp_check")
    
    # Check binary on host
    print("Checking binary on host...")
    stdin, stdout, stderr = ssh.exec_command("file /tmp/server_check && ldd /tmp/server_check 2>&1 || echo 'not on host'")
    print(stdout.read().decode())
    
    # Check inside container
    print("\n=== Check ldd inside container ===")
    cmd = """docker create --name temp_ldd moduforge:latest sh -c "ldd /server" """
    ssh.exec_command(cmd)
    stdin, stdout, stderr = ssh.exec_command("docker start temp_ldd && docker logs temp_ldd")
    print(stdout.read().decode())
    ssh.exec_command("docker rm temp_ldd")
    
    ssh.close()

if __name__ == '__main__':
    test()
