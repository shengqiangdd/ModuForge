#!/usr/bin/env python3
"""Run server binary directly, bypassing entrypoint."""
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def test():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    print("=== Test: Run /server directly ===")
    # Override entrypoint completely
    cmd = """docker run --rm --entrypoint /bin/sh -v moduforge_data:/data moduforge:latest -c "rm -f /data/moduforge.db* && ls -la /data && /server" """
    stdin, stdout, stderr = ssh.exec_command(cmd)
    output = stdout.read().decode()
    error = stderr.read().decode()
    print(f"Output: {output[:3000]}")
    if error:
        print(f"Error: {error[:1000]}")
    
    ssh.close()

if __name__ == '__main__':
    test()
