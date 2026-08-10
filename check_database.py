#!/usr/bin/env python3
"""Check if database file is valid."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def check():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Check database file header ===")
    stdin, stdout, stderr = ssh.exec_command("xxd /vol1/docker/volumes/moduforge_data/_data/moduforge.db | head -5")
    print(stdout.read().decode())
    
    print("\n=== Check database integrity ===")
    stdin, stdout, stderr = ssh.exec_command("sqlite3 /vol1/docker/volumes/moduforge_data/_data/moduforge.db '.tables' 2>&1 || echo 'sqlite3 not found'")
    print(stdout.read().decode())
    
    print("\n=== Test with new database ===")
    # Try creating a fresh database
    cmd = """docker run --rm -v moduforge_data:/data moduforge:latest sh -c "rm -f /data/moduforge.db && touch /data/moduforge.db && ls -la /data/moduforge.db" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    print("\n=== Test server with fresh DB ===")
    cmd = """docker run --rm -v moduforge_data:/data -e DB_PATH=/data/moduforge.db moduforge:latest sh -c "ls -la /data && /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode()[:1000])
    
    ssh.close()

if __name__ == '__main__':
    check()
