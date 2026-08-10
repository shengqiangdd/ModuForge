#!/usr/bin/env python3
"""
Clean container - use tar approach to properly sync files.
"""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def clean():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    # Stop container
    print("Stopping container...")
    ssh.exec_command(f"docker stop {CONTAINER}")
    time.sleep(2)
    
    # List all files
    print("Listing all files...")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} ls -la /app/dist/")
    output = stdout.read().decode()
    print(output)
    
    # Remove the entire dist directory and recreate from fresh
    print("\nRemoving old dist...")
    ssh.exec_command(f"docker exec {CONTAINER} rm -rf /app/dist")
    time.sleep(1)
    
    # Recreate with fresh copy from deploy script
    print("Re-deploying fresh copy...")
    ssh.exec_command(f"docker start {CONTAINER}")
    time.sleep(2)
    
    # Use the deploy script
    import subprocess
    subprocess.run(["python", "ModuForge/deploy_local.py"], check=True)
    
    # Verify
    print("\nVerifying...")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} find /app/dist -type f | sort")
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    clean()
