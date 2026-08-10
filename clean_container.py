#!/usr/bin/env python3
"""
Clean container deployment - remove Windows artifacts properly.
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
    
    # Get list of files with backslash
    print("Finding backslash files...")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} ls -1 /app/dist/")
    files = stdout.read().decode().split('\n')
    print(f"  All files: {files}")
    
    # Delete problematic files using Python on the host
    print("Cleaning via host Python...")
    for f in files:
        if '\\' in f:
            # This file has backslash in name - need to remove it
            # Use docker exec with proper quoting
            cmd = f"docker exec {CONTAINER} rm -rf '/app/dist/{f}'"
            print(f"  Removing: {f}")
            stdin, stdout, stderr = ssh.exec_command(cmd)
            err = stderr.read().decode()
            if err:
                print(f"    Error: {err}")
    
    # Also remove unnecessary files
    print("Removing unnecessary files...")
    unnecessary = ['manifest.json', 'sw.js', 'icon-192.svg', 'icon-512.svg', 
                   'fonts/MaterialSymbolsOutlined.ttf']
    for f in unnecessary:
        cmd = f"docker exec {CONTAINER} rm -f /app/dist/{f}"
        ssh.exec_command(cmd)
        print(f"  Removed: {f}")
    
    # Start container
    print("Starting container...")
    ssh.exec_command(f"docker start {CONTAINER}")
    time.sleep(3)
    
    # Verify
    print("\nFinal state:")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} find /app/dist -type f | wc -l")
    print(f"  Files: {stdout.read().decode().strip()}")
    
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} du -sh /app/dist")
    print(f"  Size: {stdout.read().decode().strip()}")
    
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    print(f"  Health: {stdout.read().decode().strip()}")
    
    ssh.close()

if __name__ == '__main__':
    clean()
