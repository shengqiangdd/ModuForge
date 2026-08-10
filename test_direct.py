#!/usr/bin/env python3
"""Test database access directly."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test 1: Direct file access ===")
    cmd = """docker run --rm -v moduforge_data:/data alpine sh -c "ls -la /data && file /data/moduforge.db" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    print("\n=== Test 2: Check file content ===")
    cmd = """docker run --rm -v moduforge_data:/data alpine sh -c "head -c 100 /data/moduforge.db | xxd | head -5" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    print("\n=== Test 3: Check if file is SQLite ===")
    cmd = """docker run --rm -v moduforge_data:/data alpine sh -c "head -c 16 /data/moduforge.db" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read()
    print(f"Header bytes: {output}")
    if output.startswith(b'SQLite format 3'):
        print("  -> Valid SQLite file")
    else:
        print(f"  -> Not SQLite, first bytes: {output[:20]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
