#!/usr/bin/env python3
"""Debug entrypoint and database path."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def debug():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Check volume mount ===")
    stdin, stdout, stderr = ssh.exec_command("docker volume inspect moduforge_data --format='{{.Mountpoint}}'")
    mountpoint = stdout.read().decode().strip()
    print(f"Mountpoint: {mountpoint}")
    
    print("\n=== Files in mountpoint ===")
    stdin, stdout, stderr = ssh.exec_command(f"ls -la {mountpoint}")
    print(stdout.read().decode())
    
    print("\n=== Check if DB exists ===")
    stdin, stdout, stderr = ssh.exec_command(f"ls -la {mountpoint}/moduforge.db")
    print(stdout.read().decode())
    
    # Try a simple container to test volume mount
    print("\n=== Testing volume mount with simple container ===")
    test_cmd = f"""docker run --rm -v moduforge_data:/data alpine ls -la /data"""
    stdin, stdout, stderr = ssh.exec_command(test_cmd)
    print(stdout.read().decode())
    
    print("\n=== Check entrypoint script ===")
    # Get image layers to find entrypoint
    stdin, stdout, stderr = ssh.exec_command("docker inspect moduforge:latest --format='{{json .Config.Entrypoint}}'")
    print(f"Entrypoint: {stdout.read().decode()}")
    
    stdin, stdout, stderr = ssh.exec_command("docker inspect moduforge:latest --format='{{json .Config.Cmd}}'")
    print(f"Cmd: {stdout.read().decode()}")
    
    ssh.close()

if __name__ == '__main__':
    debug()
