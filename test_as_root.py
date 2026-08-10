#!/usr/bin/env python3
"""Test as root to bypass permission issues."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Run as root ===")
    cmd = """docker run --rm --user root --entrypoint sh -v moduforge_data:/data moduforge:latest -c "rm -f /data/moduforge.db* && ls -la /data && DB_PATH=/data/moduforge.db /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:3000]}")
    if error:
        print(f"Error: {error[:500]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
