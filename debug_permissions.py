#!/usr/bin/env python3
"""Debug database permissions."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def debug():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Volume mountpoint ===")
    stdin, stdout, stderr = ssh.exec_command("docker volume inspect moduforge_data --format='{{.Mountpoint}}'")
    mountpoint = stdout.read().decode().strip()
    print(f"Mountpoint: {mountpoint}")
    
    print("\n=== Directory permissions ===")
    stdin, stdout, stderr = ssh.exec_command(f"ls -la {mountpoint}")
    print(stdout.read().decode())
    
    print("\n=== Fix permissions ===")
    commands = [
        f"chmod -R 777 {mountpoint}",
        f"chown -R 1000:1001 {mountpoint}",
    ]
    for cmd in commands:
        print(f"  {cmd}")
        ssh.exec_command(cmd)
    
    print("\n=== Verify permissions ===")
    stdin, stdout, stderr = ssh.exec_command(f"ls -la {mountpoint}")
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    debug()
