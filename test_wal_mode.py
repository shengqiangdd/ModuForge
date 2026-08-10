#!/usr/bin/env python3
"""Test if WAL mode is the issue."""
import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Try with DELETE journal mode ===")
    # Stop existing container
    ssh.exec_command(f"docker stop {CONTAINER}")
    time.sleep(2)
    
    # Test with modified DB_PATH that disables WAL
    cmd = """docker run --rm --user root -v moduforge_data:/data moduforge:latest sh -c "rm -f /data/moduforge.db* && DB_PATH='/data/moduforge.db?_journal_mode=DELETE' /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:2000]}")
    if error:
        print(f"Error: {error[:500]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
