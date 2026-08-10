#!/usr/bin/env python3
"""Test SQLite directly without the server binary."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Create SQLite DB directly ===")
    cmd = """docker run --rm --user root --entrypoint sh -v moduforge_data:/data moduforge:latest -c "cd /data && sqlite3 moduforge.db 'CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY);' && echo 'SQLite works!' && ls -la moduforge.db" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    print(stderr.read().decode()[:500])
    
    print("\n=== Test: Check if sqlite3 exists ===")
    cmd = """docker run --rm --user root --entrypoint sh moduforge:latest -c "which sqlite3 || apk add sqlite 2>/dev/null && sqlite3 --version" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    print(stdout.read().decode())
    
    ssh.close()

if __name__ == '__main__':
    test()
