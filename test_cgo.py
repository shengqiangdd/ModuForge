#!/usr/bin/env python3
"""Test CGO and SQLite support."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Check binary dependencies ===")
    cmd = """docker run --rm moduforge:latest sh -c "ls -la /server && file /server && ldd /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    print("\n=== Test: Check if SQLite works ===")
    cmd = """docker run --rm moduforge:latest sh -c "echo 'CREATE TABLE test (id INT);' | sqlite3 /tmp/test.db && echo 'SQLite works!' || echo 'SQLite failed'" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode())
    
    print("\n=== Test: Try creating DB manually ===")
    cmd = """docker run --rm --entrypoint sh -v moduforge_data:/data moduforge:latest -c "mkdir -p /data && touch /data/moduforge.db && ls -la /data/moduforge.db && DB_PATH=/data/moduforge.db /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:2000]}")
    if error:
        print(f"Error: {error[:500]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
