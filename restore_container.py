#!/usr/bin/env python3
"""
Restore container properly via SSH.
"""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def restore():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Check current state
    print("=== Current state ===")
    stdin, stdout, stderr = ssh.exec_command("docker ps -a | grep moduforge")
    print(stdout.read().decode())
    
    # Use docker compose on the server
    print("\n=== Using docker compose ===")
    commands = [
        "cd /opt/moduforge 2>/dev/null || cd ~/moduforge 2>/dev/null || echo 'no project dir'",
        "docker compose up -d --force-recreate 2>&1 || docker-compose up -d --force-recreate 2>&1",
    ]
    
    for cmd in commands:
        print(f"  > {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        exit_status = stdout.channel.recv_exit_status()
        output = stdout.read().decode()
        error = stderr.read().decode()
        if output:
            print(f"    {output}")
        if error:
            print(f"    stderr: {error}")
    
    # Wait for container
    time.sleep(5)
    
    # Check status
    print("\n=== Container status ===")
    stdin, stdout, stderr = ssh.exec_command("docker ps | grep moduforge")
    print(stdout.read().decode())
    
    # Check health
    print("=== Health check ===")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    restore()
